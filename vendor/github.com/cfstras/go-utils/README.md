# cfstras' go utilities

## depgraph
A script I put together from various others.

Generates a dependency graph for graphviz from a gopath.
Ignores most core packages.

### Usage

```bash
# install graphviz (debian/ubuntu example)
sudo apt-get install graphviz

# install depgraph
go get github.com/cfstras/go-utils/depgraph

# start it.
# <root package> should be a package you made, for example main.
$GOPATH/bin/depgraph <root package> | dot -Tsvg > graph.svg
```

# Help & Contributing
Feel free to contact me if you need help.

Patches and issue reports are always welcome.

# License
GPLv3.
