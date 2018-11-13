// Copyright 2018 Alexander Zaytsev <thebestzorro@yandex.ru>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package web contains HTTP handlers methods.
// There are 2 URLs:
// 1. "/" - GET and POST
// 2. "/<hash>" - GET and POST
package web

import (
	"github.com/gomodule/redigo/redis"
	"net/http"
	"strings"

	"github.com/z0rr0/enigma/conf"
	"github.com/z0rr0/enigma/db"
)

// ErrorData is a struct for error handling.
type ErrorData struct {
	Title string
	Msg   string
}

// Error sets error page. It returns code value.
func Error(w http.ResponseWriter, cfg *conf.Cfg, code int) int {
	var title, msg string
	w.WriteHeader(code)

	tpl := cfg.Templates["error"]
	switch code {
	case http.StatusNotFound:
		title, msg = "Not found", "Page not found"
	case http.StatusBadRequest:
		title, msg = "Error", "Bad request"
	default:
		title, msg = "Error", "Sorry, it is an error"
	}
	data := &ErrorData{title, msg}
	tpl.Execute(w, data)
	return code
}

// create handles new item creation.
func create(w http.ResponseWriter, r *http.Request, cfg *conf.Cfg) (int, error) {
	item, err := db.New(r, cfg)
	if err != nil {
		return Error(w, cfg, http.StatusBadRequest), err
	}
	conn := cfg.Connection()
	defer conn.Close()
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
func get(w http.ResponseWriter, r *http.Request, item *db.Item, c redis.Conn, cfg *conf.Cfg) (int, error) {
	item.Password = r.PostFormValue("password")
	ok, err := item.CheckPassword(c)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError), err
	}
	if !ok {
		// TODO: retry
		return Error(w, cfg, http.StatusNotFound), nil
	}
	err = item.Read(c, cfg.CipherKey)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError), err
	}
	tpl := cfg.Templates["content"]
	err = tpl.Execute(w, map[string]string{"Content": item.Content})
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError), err
	}
	return http.StatusOK, nil
}

// Index is a base HTTP handler. POST request creates new item.
// Return value is HTTP status code.
func Index(w http.ResponseWriter, r *http.Request, cfg *conf.Cfg) (int, error) {
	if r.Method == "POST" {
		return create(w, r, cfg)
	}
	tpl := cfg.Templates["index"]
	tpl.Execute(w, nil)
	return http.StatusOK, nil
}

// Read returns a page with descrypted user's data.
func Read(w http.ResponseWriter, r *http.Request, cfg *conf.Cfg) (int, error) {
	key := strings.Trim(r.RequestURI, "/ ")
	if len(key) != db.KeyLen*2 {
		return Error(w, cfg, http.StatusNotFound), nil
	}

	item := &db.Item{Key: key}
	conn := cfg.Connection()
	defer conn.Close()

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
	tpl.Execute(w, nil)
	return http.StatusOK, nil
}
