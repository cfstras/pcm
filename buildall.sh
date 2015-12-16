#!/bin/bash

set -o pipefail
set -e

rm -rfv build/
export CGO_ENABLED=1

if [[ "$TRAVIS_OS_NAME" == "osx" ]]; then
    export GOARCH=amd64
    go env
    go build -v -o build/osx-x64/pcm

    export GOARCH=386
    go env
    go build -v -o build/osx-x86/pcm

    find build -type f -exec file {} \;
    exit
fi

echo "---"
export GOOS=linux
export GOARCH=386
go env
go build -v -ldflags -extldflags="-static" -o build/linux-x86/static/pcm
go build -v -o build/linux-x86/dynamic/pcm

echo "---"
export GOOS=linux
export GOARCH=amd64
go env
go build -v -ldflags -extldflags="-static" -o build/linux-x64/static/pcm
go build -v -o build/linux-x64/dynamic/pcm

find build -type f -exec file {} \;
