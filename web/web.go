// Copyright 2018 Alexander Zaytsev <thebestzorro@yandex.ru>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package web contains HTTP handlers methods.
// There are 2 URLs:
// 1. "/" - GET and POST
// 2. "/<hash>" - GET and POST
package web

import (
	"net/http"

	"github.com/z0rr0/enigma/conf"
	"github.com/z0rr0/enigma/db"
)

// ErrorData is a struct for error handling.
type ErrorData struct {
	Title string
	Msg   string
}

// Error sets error page.
func Error(w http.ResponseWriter, cfg *conf.Cfg, code int) {
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
}

func create(w http.ResponseWriter, r *http.Request, cfg *conf.Cfg) (int, error) {
	var code int
	item, err := db.New(r, cfg)
	if err != nil {
		code = http.StatusBadRequest
		Error(w, cfg, code)
		return code, err
	}
	err = item.Save(cfg.Connection(), cfg.CipherKey)
	if err != nil {
		code = http.StatusInternalServerError
		Error(w, cfg, code)
		return code, err
	}
	tpl := cfg.Templates["result"]
	tpl.Execute(w, map[string]string{"URL": item.Key})
	return http.StatusOK, err
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
