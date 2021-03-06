// Copyright 2018 Alexander Zaytsev <thebestzorro@yandex.ru>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package web contains HTTP handlers methods.
// There are 2 URLs:
// 1. "/" - GET and POST
// 2. "/<hash>" - GET and POST
package web

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gomodule/redigo/redis"
	"github.com/z0rr0/enigma/conf"
	"github.com/z0rr0/enigma/db"
)

var (
	// internal logger
	logger = log.New(os.Stderr, fmt.Sprintln("critical error"),
		log.Ldate|log.Ltime|log.Lshortfile)
)

// ErrorData is a struct for error handling.
type ErrorData struct {
	Title string
	Msg   string
}

// CheckPassword is data for failed password check.
type CheckPassword struct {
	Err bool
	Msg string
}

// Error sets error page. It returns code value.
func Error(w io.Writer, cfg *conf.Cfg, code int) int {
	var title, msg string
	httpWriter, ok := w.(http.ResponseWriter)
	if ok {
		httpWriter.WriteHeader(code)
	}
	tpl := cfg.Templates["error"]
	switch code {
	case http.StatusNotFound:
		title, msg = "Not found", "Page not found"
	case http.StatusBadRequest:
		title, msg = "Error", "Bad createData"
	default:
		title, msg = "Error", "Sorry, it is an error"
	}
	data := &ErrorData{title, msg}
	err := tpl.Execute(w, data)
	if err != nil {
		logger.Println("error-template execute failed")
		return http.StatusInternalServerError
	}
	return code
}

// create handles new item creation.
func create(w io.Writer, r *http.Request, cfg *conf.Cfg) (int, error) {
	item, err := db.New(r, cfg.Settings.TTL, cfg.Settings.Times)
	if err != nil {
		return Error(w, cfg, http.StatusBadRequest), err
	}
	conn := cfg.Connection()
	defer func() {
		err := conn.Close()
		if err != nil {
			logger.Println("failed connection close after creation")
		}
	}()
	err = item.Save(conn, cfg.CipherKey)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError), err
	}
	tpl := cfg.Templates["result"]
	err = tpl.Execute(w, map[string]string{"URL": item.GetURL(r, cfg.Secure).String()})
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError), err
	}
	return http.StatusOK, nil
}

// get user's data.
func get(w io.Writer, r *http.Request, item *db.Item, c redis.Conn, cfg *conf.Cfg) (int, error) {
	item.Password = r.PostFormValue("password")
	ok, err := item.CheckPassword(c)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError), err
	}
	if !ok {
		tpl := cfg.Templates["read"]

		httpWriter, ok := w.(http.ResponseWriter)
		if !ok {
			return Error(w, cfg, http.StatusInternalServerError), err
		}
		code := http.StatusBadRequest
		httpWriter.WriteHeader(code)

		err = tpl.Execute(w, CheckPassword{true, "Failed password"})
		if err != nil {
			return Error(w, cfg, http.StatusInternalServerError), err
		}
		return code, nil
	}
	exists, err := item.Read(c, cfg.CipherKey)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError), err
	}
	if !exists {
		return Error(w, cfg, http.StatusNotFound), nil
	}
	tpl := cfg.Templates["content"]
	err = tpl.Execute(w, map[string]string{"Content": item.Content})
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError), err
	}
	return http.StatusOK, nil
}

// Index is a base HTTP handler. POST createData creates new item.
// Return value is HTTP status code.
func Index(w io.Writer, r *http.Request, cfg *conf.Cfg) (int, error) {
	if r.Method == "POST" {
		return create(w, r, cfg)
	}
	tpl := cfg.Templates["index"]
	err := tpl.Execute(w, nil)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

// Read returns a page with decrypted user's data.
func Read(w io.Writer, r *http.Request, cfg *conf.Cfg) (int, error) {
	key := strings.Trim(r.RequestURI, "/ ")
	if len(key) != db.KeyLen*2 {
		return Error(w, cfg, http.StatusNotFound), nil
	}

	item := &db.Item{Key: key}
	conn := cfg.Connection()
	defer func() {
		err := conn.Close()
		if err != nil {
			logger.Println("failed connection close after reading")
		}
	}()
	// check items exists
	exists, err := item.Exists(conn)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError), err
	}
	if !exists {
		return Error(w, cfg, http.StatusNotFound), nil
	}

	if r.Method == "POST" {
		return get(w, r, item, conn, cfg)
	}
	tpl := cfg.Templates["read"]
	err = tpl.Execute(w, nil)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
