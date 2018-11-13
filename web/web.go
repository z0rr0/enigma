// Copyright 2018 Alexander Zaytsev <thebestzorro@yandex.ru>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package web contains HTTP handlers methods.
// There are 2 URLs:
// 1. "/" - GET and POST
// 2. "/<hash>" - GET and POST
package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/z0rr0/enigma/conf"
)

// Item is data for new saving.
type Item struct {
	Content  string
	TTL      int
	Times    int
	Password string
}

// ErrorData is a struct for error handling.
type ErrorData struct {
	Title string
	Msg   string
}

// Error sets error page.
func Error(w http.ResponseWriter, cfg conf.Cfg, code int) {
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

// validateRange converts value to integer and check that it is in a range [1; max],
func validateRange(value, name string, max int) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if (n < 1) || (n > max) {
		return 0, fmt.Errorf("field %v=%v but available range [%v - %v]", name, n, 1, max)
	}
	return n, nil
}

// validate checks POST form data.
func validate(r *http.Request, cfg *conf.Cfg) (*Item, error) {
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

func create(w http.ResponseWriter, r *http.Request, cfg *conf.Cfg) error {
	item, err := validate(r, cfg)
	if err != nil {
		return err
	}
	fmt.Println(item)
	return nil
}

// Index is a base HTTP handler. POST request creates new item.
// Return value is HTTP status code.
func Index(w http.ResponseWriter, r *http.Request, cfg *conf.Cfg) (int, error) {
	if r.Method == "POST" {
		tpl := cfg.Templates["index"]
		tpl.Execute(w, nil)
		return http.StatusOK, nil
	}
	tpl := cfg.Templates["index"]
	tpl.Execute(w, nil)
	return http.StatusOK, nil
}
