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
	"net/http"
)

const (
	indexPage = `
<!DOCTYPE html>
<html>
	<head>
		<title>Enigma</title>
	</head>
<body>
	<h1>Enigma</h1>
</body>
</html>	`
	resultPage = `
<!DOCTYPE html>
<html>
	<head>
		<title>Enigma</title>
	</head>
<body>
	<h1>Enigma</h1>
</body>
</html>`
)

type HTTPError struct {
	Code int
	Msg  string
}

func (e *HTTPError) Error() string {
	return e.Msg
}

// Index is a base HTTP handler. POST request creates new item.
func Index(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "POST" {
		return &HTTPError{200, "OK"}
	}
	fmt.Fprint(w, indexPage)
	return nil
}
