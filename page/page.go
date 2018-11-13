// Copyright 2018 Alexander Zaytsev <thebestzorro@yandex.ru>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package page contains HTML text templates.
package page

const (
	// PageIndex is index page HTML template.
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
	// PageError is error page HTML template.
	PageError = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset=utf-8>
		<title>Enigma - {{ .Title }}</title>
	</head>
	<body>
		<h1><a href="/" title="Enigma">Enigma</a></h1>
		<h4>{{ .Msg }}</h4>
	</body>
</html>
`
	// PageResul is HTML template for link sharing.
	PageResul = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset=utf-8>
		<title>Enigma</title>
	</head>
	<body>
		<h1><a href="/" title="Enigma">Enigma</a></h1>
		<strong><a href="{{ .URL }}">{{ .URL }}</a></strong>
	</body>
</html>
`
	// PageRead is HTML template for data decyption.
	PageRead = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset=utf-8>
		<title>Enigma</title>
	</head>
	<body>
		<h1><a href="/" title="Enigma">Enigma</a></h1>
		<form method="POST">
			Password: <input type="password" name="password" placeholder="optional">
			<input type="submit" value="Get">
		</form>
	</body>
</html>
`
	// PageContent is HTML template with decrypted user's data.
	PageContent = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset=utf-8>
		<title>Enigma</title>
	</head>
	<body>
		<h1><a href="/" title="Enigma">Enigma</a></h1>
		<p>{{.Content}}</p>
	</body>
</html>
`
)
