# pcm [![Build Status](https://travis-ci.org/cfstras/pcm.svg?branch=master)](https://travis-ci.org/cfstras/pcm)
a Linux SSH client wrapper reading PuTTYConnectionManager configurations, with fuzzy searching.

Supports auto-login with stored passwords and running predefined commands as specified in PuTTYCM 

[![Screenshot](http://i.imgur.com/UrSlBvTl.png)](http://i.imgur.com/UrSlBvT.png)

Search example:  
[![fuzzy search example](http://i.imgur.com/qu8iJbMl.png)](http://i.imgur.com/qu8iJbM.png)

# Downloads

If you don't have Golang installed or don't want to build yourself (try it, it's not complex!), you can download the latest pre-built release here (I recommend putting it into `~/bin` or `/usr/local/bin`):

- Linux: [64 bit][linux-x64] or [32 bit][linux-x64]
- Mac OSX: [64 bit][osx-x64] or [32 bit][osx-x86]

[linux-x64]: https://github.com/cfstras/pcm/releases/download/1.1/pcm-linux-x64
[linux-x86]: https://github.com/cfstras/pcm/releases/download/1.1/pcm-linux-x86
[osx-x64]: https://github.com/cfstras/pcm/releases/download/1.1/pcm-osx-x64
[osx-x86]: https://github.com/cfstras/pcm/releases/download/1.1/pcm-osx-x86

# Requirements

- *nix (Successfully tested on OSX and Linux)
- Golang 1.5+ (Available in most distros, for OSX: Homebrew!)

# Installing

Once you have Golang, (go to [golang.org/dl](https://golang.org/dl/) or install with homebrew: `brew install go`)

- Set a `GOPATH` and include `$GOPATH/bin` in your `$PATH` (put these instructions in your `.bashrc`):

    export GOPATH=$HOME/go
    export PATH=$PATH:$GOPATH/bin

- Install the software:

    go get github.com/cfstras/pcm

- The binary will be at `$GOPATH/bin/pcm`, and will search for a connections.xml to be in $HOME/Downloads/.

- To invoke:

    pcm                          # open the UI
    pcm my-node                  # Open the UI, prefill the search box with "my-node"

Once you have the UI, use arrow keys to navigate, type to search, and press enter to connect.

### Arguments

    -connectionsPath path/to/xml # to override the search path to connections.xml
    -verbose/-v                  # display full info (with password) and hostname before connecting
    -simple                      # disable UI


Hint: If you don't want to put your connections.xml into Downloads, put this alias in your `~/.bashrc`:

    alias pcm="$GOPATH/bin/pcm -connectionsPath $HOME/secret/connections.xml"

# License
MIT. See [LICENSE.md](https://github.com/cfstras/pcm/LICENSE.md)
