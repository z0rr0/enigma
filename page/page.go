// Copyright 2018 Alexander Zaytsev <thebestzorro@yandex.ru>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package html contains HTML text templates.
package page

const (
	PageIndex = `
<!DOCTYPE html>
<html>
	<head>
		<title>Enigma</title>
	</head>
<body>
	<h1>Enigma</h1>
</body>
</html>`
	PageError = `
<!DOCTYPE html>
<html>
	<head>
		<title>Enigma - {{ .Title }}</title>
	</head>
<body>
	<h1>{{ .Msg }}</h1>
</body>
</html>`
)
