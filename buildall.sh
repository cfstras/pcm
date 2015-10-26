#!/bin/bash

rm -rfv build/
export CGO_ENABLED=1

echo "---"
export GOOS=linux
export GOARCH=386
go env
go build -v -ldflags -extldflags="-static" -o build/linux-386/pcm.static-linux-386 || exit 1
go build -v -o build/linux-386/pcm.linux-386

echo "---"
export GOOS=linux
export GOARCH=amd64
go env
go build -v -ldflags -extldflags="-static" -o build/linux-amd64/pcm.static-linux-amd64 || exit 1
go build -v -o build/linux-amd64/pcm.linux-amd64

find build -type f -exec file {} \;
