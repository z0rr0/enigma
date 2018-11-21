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
	"github.com/z0rr0/enigma/db"
	"github.com/z0rr0/enigma/page"
)

// settings is app settings.
type settings struct {
	TTL   int `json:"ttl"`
	Times int `json:"times"`
}

// Cfg is configuration settings.
type Cfg struct {
	Host      string   `json:"host"`
	Port      uint     `json:"port"`
	Timeout   int64    `json:"timeout"`
	Secure    bool     `json:"secure"`
	Redis     *db.Cfg  `json:"redis"`
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

	err := c.loadTemplates()
	if err != nil {
		return err
	}
	b, err := hex.DecodeString(c.Key)
	if err != nil {
		return errors.New("can not decode secret key")
	}
	c.CipherKey = b
	pool, err := db.GetDbPool(c.Redis)
	if err != nil {
		return err
	}
	c.pool = pool
	return nil
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
		return errors.New("templates are already loaded")
	}
	pages := map[string]string{
		"index":   page.Index,
		"error":   page.Error,
		"result":  page.Result,
		"read":    page.Read,
		"content": page.Content,
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
