# pcm
a Linux client for PuTTYConnectionManager configurations, with fuzzy searching.

# Requirements

- Linux
- Golang 1.5+

# Installing

- Ensure you have a `$GOPATH` set and `$GOPATH/bin` is included in your `PATH`
- Install:
```bash
$ go get github.com/cfstras/pcm
```
- Move your PuTTY `connections.xml` to `~/Downloads/`
- Run:
```bash
$ pcm [search-string]
```
