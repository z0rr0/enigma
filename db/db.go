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
	"net/http"
	"net/url"
	"strconv"

	"github.com/gomodule/redigo/redis"
	"github.com/z0rr0/enigma/conf"
)

const (
	// KeyLen is number of bytes for random db key.
	KeyLen = 64

	fieldContent  = "content"
	fieldPassword = "password"
	fielTimes     = "times"
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
	c.Send("MULTI")
	c.Send("HSET", item.Key, fieldContent, item.eContent)
	c.Send("HSET", item.Key, fieldPassword, item.hPassword)
	c.Send("HSET", item.Key, fielTimes, item.Times)
	c.Send("EXPIRE", item.Key, item.TTL)
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

// GetURL return item's URL.
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
		return errors.New("empty plaintext")
	}
	key := item.cipherKey(skey)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	plaintext := []byte(item.Content)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return errors.New("iv random generation error")
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)
	item.eContent = hex.EncodeToString(ciphertext)
	return nil
}

func (item *Item) decrypt(skey []byte) error {
	if item.eContent == "" {
		return errors.New("empty ciphertext")
	}
	ciphertext, err := hex.DecodeString(item.eContent)
	if err != nil {
		return err
	}
	if len(ciphertext) < aes.BlockSize {
		return errors.New("invalid cipher block length")
	}
	key := item.cipherKey(skey)
	block, err := aes.NewCipher(key)
	if err != nil {
		return errors.New("new cipher creation")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	item.Content = string(ciphertext)
	return nil
}

// Read gets data from database. Expected it is called after Exists and CheckPassword.
func (item *Item) Read(c redis.Conn, skey []byte) error {
	c.Send("MULTI")
	c.Send("HGET", item.Key, fieldContent)
	c.Send("HINCRBY", item.Key, fielTimes, -1)
	r, err := c.Do("EXEC")
	if err != nil {
		return err
	}
	result, ok := r.([]interface{})
	if !ok {
		return errors.New("failed multi read result")
	}
	if len(result) != 2 {
		return errors.New("unexpected multi item result")
	}

	content, err := redis.String(result[0], nil)
	if err != nil {
		return fmt.Errorf("failed multi read 'content': %v", err)
	}
	item.eContent = content

	times, err := redis.Int(result[1], nil)
	if err != nil {
		return fmt.Errorf("failed multi read 'times': %v", err)
	}
	item.Times = times

	err = item.decrypt(skey)
	if err != nil {
		return err
	}
	// delete item if no times for new requests
	if item.Times < 1 {
		_, err = redis.Bool(c.Do("DEL", item.Key))
		if err != nil {
			return err
		}
	}
	return nil
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

// generateKey generates a random key
func generateKey(c redis.Conn) (string, error) {
	var (
		key string
		err error
	)
	for {
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
			break
		}
	}
	return key, nil
}

// getKey return a radnom key.
func getKey() (string, error) {
	var b [KeyLen]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// validateRange converts value to integer and check that it is in a range [1; max].
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

// New checks POST form data anb return new item for saving.
func New(r *http.Request, cfg *conf.Cfg) (*Item, error) {
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
	ttl, err := validateRange(value, "ttl", cfg.Settings.TTL)
	if err != nil {
		return nil, err
	}
	// times
	value = r.PostFormValue("times")
	if value == "" {
		return nil, errors.New("required field times")
	}
	times, err := validateRange(value, "times", cfg.Settings.Times)
	if err != nil {
		return nil, err
	}
	// password
	password := r.PostFormValue("password")
	item := &Item{
		Content:  content,
		TTL:      ttl,
		Times:    times,
		Password: password,
	}
	return item, nil
}
