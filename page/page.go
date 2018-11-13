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
		<meta charset=utf-8>
		<title>Enigma</title>
	</head>
	<body>
		<h1>Enigma</h1>
		<form method="POST">
			<textarea name="content" cols="80" rows="8" placeholder="Your secret text" required></textarea><br>
			TTL: <select name="ttl" required>
				<option value='600'>10 minutes</option>
				<option value='3600'>a hour</option>
				<option value='86400' selected>a day</option>
				<option value='604800'>a week</option>
			</select>
			times: <input type="number" name="times" min="1" max="1000" value="1" required>
			password: <input type="password" name="password" placeholder="optional">
			<input type="submit" value="Send">
		</form>
	</body>
</html>
`
	PageError = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset=utf-8>
		<title>Enigma - {{ .Title }}</title>
	</head>
<body>
	<h1>{{ .Msg }}</h1>
</body>
</html>
`
	PageResul = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset=utf-8>
		<title>Enigma</title>
	</head>
	<body>
		<h1>Enigma</h1>
		<strong><a href="{{ .URL }}">{{ .URL }}</a></strong>
		<hr>
		<a href="/" title="Create new">create new message</a>
	</body>
</html>
`
)
