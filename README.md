# pcm
a Linux SSH client wrapper reading PuTTYConnectionManager configurations, with fuzzy searching.

Supports auto-login with stored passwords and running predefined commands as specified in PuTTYCM 

[![Screenshot](http://i.imgur.com/UrSlBvTl.png)](http://i.imgur.com/UrSlBvT.png)

Search example:  
[![fuzzy search example](http://i.imgur.com/qu8iJbMl.png)](http://i.imgur.com/qu8iJbM.png)

# Requirements

- *nix (Successfully tested on OSX and Linux)
- Golang 1.5+ (Available in most distros, for OSX: Homebrew!)

# Installing

- Ensure you have a `$GOPATH` set and `$GOPATH/bin` is included in your `$PATH`
- Install:
```bash
$ go get github.com/cfstras/pcm
```
- Move your PuTTY `connections.xml` to `~/Downloads/` (or supply `-connectionsPath path/to/your/.xml` to pcm)
- Run:
```bash
$ pcm [search-string]  # search string is optional.
```
