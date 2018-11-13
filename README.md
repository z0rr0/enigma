[![GoDoc](https://godoc.org/github.com/z0rr0/enigma/pwgen?status.svg)](https://godoc.org/github.com/z0rr0/enigma/pwgen)  [![version](https://img.shields.io/github/tag/z0rr0/enigma.svg)](https://github.com/z0rr0/enigma/releases/latest) [![license](https://img.shields.io/github/license/z0rr0/enigma.svg)](https://github.com/z0rr0/enigma/blob/master/LICENSE)

# Enigma
A service to share private info using web.

1. Send data + settings (optional password + TTL + number of sharing)
1. Get a link
1. Share a link

## Build


Dependencies:

```
got get github.com/gomodule/redigo/redis
```

Check and build

```bash
make install
```

For docker container

```bash
make docker 
```

## Development

### Run

```bash
make start

make restart

make stop
```

### Docker usage for debugging

Redis server (for example custom persistent storage is "/tmp/redis")

```bash
docker run --name redis -v /tmp/redis:/data -v $PWD/docker:/usr/local/etc/redis -p 6379:6379 -d redis:alpine redis-server /usr/local/etc/redis/redis.conf
```

Redis client

```bash
docker run -it --link redis:redis --rm redis:alpine redis-cli -h redis -p 6379
```

## License

This source code is governed by a MIT license that can be found in the [LICENSE](https://github.com/z0rr0/enigma/blob/master/LICENSE) file.
