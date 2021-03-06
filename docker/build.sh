#!/usr/bin/env bash

NAME="enigma"
CONTAINER="golang:alpine"
SOURCES="${GOPATH}/src"
TARGET="${GOPATH}/bin/alpine"
ATTRS="`bash version.sh`"

rm -rf ${TARGET}
mkdir -p ${TARGET}/bin ${TARGET}/pkg

/usr/bin/docker run --rm --user `id -u ${USER}`:`id -g ${USER}` \
    --volume ${SOURCES}:/usr/p/src:ro \
    --volume ${TARGET}/pkg:/usr/p/pkg \
    --volume ${TARGET}/bin:/usr/p/bin \
    --workdir /usr/p/src/github.com/z0rr0/${NAME} \
    --env GOPATH=/usr/p \
    --env GOCACHE=/tmp/.cache \
    ${CONTAINER} go install -v -ldflags "${ATTRS}" github.com/z0rr0/${NAME}

if [[ $? -gt 0 ]]; then
	echo "ERROR: build container"
	exit 1
fi
cp -v ${TARGET}/bin/${NAME}  ./