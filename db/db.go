// Copyright 2018 Alexander Zaytsev <thebestzorro@yandex.ru>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package db contains database usage methods.
package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	// KeyLen is a number of bytes for random db key.
	KeyLen = 64

	// maxCollisions a number of allowed attempts to generate a key without collisions
	maxCollisions = 16

	fieldContent  = "content"
	fieldPassword = "password"
	fieldTimes    = "times"
)

// Item is data for new saving.
type Item struct {
	Content   string
	TTL       int
	Times     int
	Password  string
	Key       string
	eContent  string
	hPassword string
}

// Cfg is configuration redis settings.
type Cfg struct {
	Host     string `json:"host"`
	Port     uint   `json:"port"`
	Network  string `json:"network"`
	Db       int    `json:"db"`
	Timeout  int64  `json:"timeout"`
	Password string `json:"password"`
	IndleCon int    `json:"indlecon"`
	MaxCon   int    `json:"maxcon"`
	timeout  time.Duration
}

// GetDbPool creates new Redis db connections pool.
func GetDbPool(c *Cfg) (*redis.Pool, error) {
	if c.Timeout < 1 {
		return nil, errors.New("invalid redis timeout value")
	}
	c.timeout = time.Duration(c.Timeout) * time.Second
	if (c.IndleCon < 1) || (c.MaxCon < 1) {
		return nil, errors.New("invalid redis connections settings")
	}
	if c.Db < 0 {
		return nil, errors.New("invalid db number")
	}
	pool := &redis.Pool{
		MaxIdle:     c.IndleCon,
		MaxActive:   c.MaxCon,
		IdleTimeout: c.timeout,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			return redis.Dial(
				c.Network,
				c.RedisAddr(),
				redis.DialConnectTimeout(c.timeout),
				redis.DialDatabase(c.Db),
				redis.DialPassword(c.Password),
			)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
	conn := pool.Get()
	_, err := conn.Do("PING")
	if err != nil {
		return nil, err
	}
	err = conn.Close()
	if err != nil {
		return nil, err
	}
	return pool, nil
}

// RedisAddr returns redis service's net address.
func (c *Cfg) RedisAddr() string {
	return net.JoinHostPort(c.Host, fmt.Sprint(c.Port))
}

// IsOk checks db is available using redis PING command.
func IsOk(conn redis.Conn) bool {
	resp, err := redis.String(conn.Do("PING"))
	if err != nil {
		return false
	}
	return resp == "PONG"
}

// Save saves the item to database.
func (item *Item) Save(c redis.Conn, skey []byte) error {
	key, err := generateKey(c)
	if err != nil {
		return err
	}
	item.Key = key

	err = item.hashPassword()
	if err != nil {
		return err
	}
	err = item.encrypt(skey)
	if err != nil {
		return err
	}
	err = c.Send("MULTI")
	if err != nil {
		return err
	}
	err = c.Send("HSET", item.Key, fieldContent, item.eContent)
	if err != nil {
		return err
	}
	err = c.Send("HSET", item.Key, fieldPassword, item.hPassword)
	if err != nil {
		return err
	}
	err = c.Send("HSET", item.Key, fieldTimes, item.Times)
	if err != nil {
		return err
	}
	err = c.Send("EXPIRE", item.Key, item.TTL)
	if err != nil {
		return err
	}
	r, err := c.Do("EXEC")
	if err != nil {
		return err
	}
	result, ok := r.([]interface{})
	if !ok {
		return errors.New("failed multi read result convertion")
	}
	if len(result) != 4 { // 4 operations: 3 hset + expire
		return errors.New("unexpected multi item result")
	}
	for i, v := range result {
		ok, err = redis.Bool(v, nil)
		if err != nil {
			return fmt.Errorf("failed operation=%v bool convertion", i)
		}
		if !ok {
			return fmt.Errorf("failed operation=%v result", i)
		}
	}
	return nil
}

// GetURL returns item's URL.
func (item *Item) GetURL(r *http.Request, secure bool) *url.URL {
	// r.URL.Scheme is blank, so use hint from settings
	scheme := "http"
	if secure {
		scheme = "https"
	}
	return &url.URL{
		Scheme: scheme,
		Host:   r.Host,
		Path:   item.Key,
	}
}

// cipherKey returns a key for user's data encryption/decryption.
func (item *Item) cipherKey(skey []byte) []byte {
	if item.Password == "" {
		return skey
	}
	n := len(skey)
	k := make([]byte, n)
	p := []byte(item.Password)
	// key = (byte of password) + (bytes of default key)
	for i := range k {
		if i < len(p) {
			k[i] = p[i]
		} else {
			k[i] = skey[i]
		}
	}
	return k
}

// encrypt encrypts user's data and sets it to the item.
func (item *Item) encrypt(skey []byte) error {
	if len(skey) == 0 {
		return errors.New("empty item key for encyption")
	}
	if item.Content == "" {
		return errors.New("empty plainText")
	}
	key := item.cipherKey(skey)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	plainText := []byte(item.Content)
	cipherText := make([]byte, aes.BlockSize+len(plainText))
	iv := cipherText[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return errors.New("iv random generation error")
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], plainText)
	item.eContent = hex.EncodeToString(cipherText)
	return nil
}

// decrypt decrypts user's data and send it to the item.
func (item *Item) decrypt(skey []byte) error {
	if item.eContent == "" {
		return errors.New("empty cipherText")
	}
	cipherText, err := hex.DecodeString(item.eContent)
	if err != nil {
		return err
	}
	if len(cipherText) < aes.BlockSize {
		return errors.New("invalid cipher block length")
	}
	key := item.cipherKey(skey)
	block, err := aes.NewCipher(key)
	if err != nil {
		return errors.New("new cipher creation")
	}
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(cipherText, cipherText)

	item.Content = string(cipherText)
	return nil
}

// Read gets data from database. Expected it is called after Exists and CheckPassword.
func (item *Item) Read(c redis.Conn, skey []byte) (bool, error) {
	if item.Key == "" {
		return false, nil
	}
	// redis increment is an atomic operation
	times, err := redis.Int(c.Do("HINCRBY", item.Key, fieldTimes, -1))
	if err != nil {
		return false, err
	}
	if times < 0 {
		// probably concurrent request is reading the item at the same time
		ok, err := item.Exists(c)
		if err != nil {
			return false, err
		}
		if !ok {
			// one doesn't exist but HINCRBY called after deleting creates a new record
			_, err = item.delete(c)
			return false, err
		}
		// item will be deleted by the first concurrent reading
		return false, nil
	}
	content, err := redis.String(c.Do("HGET", item.Key, fieldContent))
	if err != nil {
		return false, err
	}
	if times == 0 {
		// no new attempts for read
		ok, err := item.delete(c)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, fmt.Errorf("item=%v was not deleted", item.Key)
		}
	}
	item.Times = times
	item.eContent = content

	err = item.decrypt(skey)
	if err != nil {
		return false, err
	}
	return true, nil
}

// delete removes the item from db.
func (item *Item) delete(c redis.Conn) (bool, error) {
	if item.Key == "" {
		return false, errors.New("empty key for delete")
	}
	return Delete(item.Key, c)
}

// Exists returns true if item exists in database.
func (item *Item) Exists(c redis.Conn) (bool, error) {
	exists, err := redis.Bool(c.Do("HEXISTS", item.Key, fieldContent))
	if err != nil {
		return false, err
	}
	return exists, nil
}

// CheckPassword checks that password is correct.
func (item *Item) CheckPassword(c redis.Conn) (bool, error) {
	var err error
	if item.hPassword == "" {
		err = item.hashPassword()
		if err != nil {
			return false, err
		}
	}
	h, err := redis.String(c.Do("HGET", item.Key, fieldPassword))
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(item.hPassword), []byte(h)), nil
}

// hashPassword calculates and sets password hash for item.
func (item *Item) hashPassword() error {
	if item.Key == "" {
		return errors.New("empty item key")
	}
	h := sha512.New()
	_, err := h.Write([]byte(item.Password + item.Key))
	if err != nil {
		return err
	}
	item.hPassword = hex.EncodeToString(h.Sum(nil))
	return nil
}

// generateKey generates unique random key for an item.
func generateKey(c redis.Conn) (string, error) {
	var (
		key string
		err error
	)
	// loop to exclude collisions
	for i := 0; i < maxCollisions; i++ {
		key, err = getKey()
		if err != nil {
			return "", err
		}
		// check that key doesn't exist before
		exists, err := redis.Bool(c.Do("HEXISTS", key, fieldContent))
		if err != nil {
			return "", err
		}
		if !exists {
			return key, nil
		}
	}
	return "", fmt.Errorf("can not get an unique key [%v] after %v attemps", KeyLen, maxCollisions)
}

// getKey returns string random key.
func getKey() (string, error) {
	var b [KeyLen]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// validateRange converts value to integer and checks that it is in a range [1; max].
func validateRange(value, field string, max int) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if (n < 1) || (n > max) {
		return 0, fmt.Errorf("field %v=%v but available range [%v - %v]", field, n, 1, max)
	}
	return n, nil
}

// New checks POST form data anb returns new item for saving.
func New(r *http.Request, ttl, times int) (*Item, error) {
	// text content
	content := r.PostFormValue("content")
	if content == "" {
		return nil, errors.New("required field content")
	}
	// TTL
	value := r.PostFormValue("ttl")
	if value == "" {
		return nil, errors.New("required field ttl")
	}
	ttl, err := validateRange(value, "ttl", ttl)
	if err != nil {
		return nil, err
	}
	// times
	value = r.PostFormValue("times")
	if value == "" {
		return nil, errors.New("required field times")
	}
	attempts, err := validateRange(value, "times", times)
	if err != nil {
		return nil, err
	}
	// password
	password := r.PostFormValue("password")
	item := &Item{
		Content:  content,
		TTL:      ttl,
		Times:    attempts,
		Password: password,
	}
	return item, nil
}

// Delete removes data struct by the key.
func Delete(key string, c redis.Conn) (bool, error) {
	return redis.Bool(c.Do("DEL", key))
}
