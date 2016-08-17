#!/bin/bash

set -o pipefail
set -e

rm -rfv build/
export CGO_ENABLED=1

if [[ "$TRAVIS_OS_NAME" == "osx" ]]; then
    export GOOS=darwin
    export GOARCH=amd64
    go env
    go build -v -o build/pcm-osx-x64

    export GOARCH=386
    go env
    go build -v -o build/pcm-osx-x86
    exit
fi

echo "---"
export GOOS=linux
export GOARCH=386
go env
go build -v -ldflags -extldflags="-static" -o build/pcm-linux-x86
#go build -v -o build/linux-x86/dynamic/pcm

echo "---"
export GOARCH=amd64
go env
go build -v -ldflags -extldflags="-static" -o build/pcm-linux-x64
#go build -v -o build/linux-x64/dynamic/pcm

find build -type f -exec file {} \;
