#!/bin/bash

BUILD = "go build -v"
STATIC = '-ldflags -extldflags="-static"'

rm -rfv build/
export CGO_ENABLED=1

echo "---"
export GOOS=linux
export GOARCH=386
go env
go build -v -ldflags -extldflags="-static" -o build/linux-386/pcm || exit 1

echo "---"
export GOOS=linux
export GOARCH=amd64
go env
go build -v -ldflags -extldflags="-static" -o build/linux-amd64/pcm || exit 1


find build -type f -exec file {} \;
