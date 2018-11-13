// Copyright 2018 Alexander Zaytsev <thebestzorro@yandex.ru>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package conf implements methods setup configuration settings.
package conf

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/z0rr0/enigma/page"
)

// rediscfg is configuration redis settings.
type rediscfg struct {
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

// settings is app settings.
type settings struct {
	TTL   int `json:"ttl"`
	Times int `json:"times"`
}

// Cfg is rates' configuration settings.
type Cfg struct {
	Host      string   `json:"host"`
	Port      uint     `json:"port"`
	Timeout   int64    `json:"timeout"`
	Redis     rediscfg `json:"redis"`
	Key       string   `json:"key"`
	Settings  settings `json:"settings"`
	CipherKey []byte
	Templates map[string]*template.Template
	timeout   time.Duration
	pool      *redis.Pool
}

// isValid checks the settings are valid.
func (c *Cfg) isValid() error {
	if c.Timeout < 1 {
		return errors.New("invalid timeout value")
	}
	if c.Port < 1 {
		return errors.New("port should be positive")
	}
	if c.Settings.TTL < 1 {
		return errors.New("ttl setting should be positive")
	}
	if c.Settings.Times < 1 {
		return errors.New("times setting should be positive")
	}
	c.timeout = time.Duration(c.Timeout) * time.Second
	if c.Redis.Timeout < 1 {
		return errors.New("invalid redis timeout value")
	}
	err := c.loadTemplates()
	if err != nil {
		return err
	}
	c.Redis.timeout = time.Duration(c.Redis.Timeout) * time.Second
	if (c.Redis.IndleCon < 1) || (c.Redis.MaxCon < 1) {
		return errors.New("invalid redis connections settings")
	}
	if c.Redis.Db < 0 {
		return errors.New("invalid db number")
	}
	b, err := hex.DecodeString(c.Key)
	if err != nil {
		return errors.New("can not decode secret key")
	}
	c.CipherKey = b
	return c.setRedisPool()
}

// New returns new configuration.
func New(filename string) (*Cfg, error) {
	fullPath, err := filepath.Abs(strings.Trim(filename, " "))
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	jsonData, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	c := &Cfg{}
	err = json.Unmarshal(jsonData, c)
	if err != nil {
		return nil, err
	}
	err = c.isValid()
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Close frees resources.
func (c *Cfg) Close() error {
	return c.closeRedisPool()
}

// Addr returns service's net address.
func (c *Cfg) Addr() string {
	return net.JoinHostPort(c.Host, fmt.Sprint(c.Port))
}

// RedisAddr returns redis service's net address.
func (c *Cfg) RedisAddr() string {
	return net.JoinHostPort(c.Redis.Host, fmt.Sprint(c.Redis.Port))
}

// setRedisPool sets redis connections pool and checks it.
func (c *Cfg) setRedisPool() error {
	pool := &redis.Pool{
		MaxIdle:     c.Redis.IndleCon,
		MaxActive:   c.Redis.MaxCon,
		IdleTimeout: c.Redis.timeout,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			return redis.Dial(
				c.Redis.Network,
				c.RedisAddr(),
				redis.DialConnectTimeout(c.Redis.timeout),
				redis.DialDatabase(c.Redis.Db),
				redis.DialPassword(c.Redis.Password),
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
		return err
	}
	c.pool = pool
	return conn.Close()
}

// closeRedisPool releases redis pool.
func (c *Cfg) closeRedisPool() error {
	if c.pool == nil {
		return nil
	}
	return c.pool.Close()
}

// HandleTimeout is service timeout.
func (c *Cfg) HandleTimeout() time.Duration {
	return c.timeout
}

// loadTemplates loads HTML templates to memory.
func (c *Cfg) loadTemplates() error {
	if len(c.Templates) > 0 {
		return errors.New("tempalate is already loaded")
	}
	pages := map[string]string{
		"index":  page.PageIndex,
		"error":  page.PageError,
		"result": page.PageResul,
	}
	c.Templates = make(map[string]*template.Template, len(pages))

	for name, content := range pages {
		tpl, err := template.New(name).Parse(content)
		if err != nil {
			return err
		}
		c.Templates[name] = tpl
	}
	return nil
}

// Connection return new database connection.
func (c *Cfg) Connection() redis.Conn {
	return c.pool.Get()
}
