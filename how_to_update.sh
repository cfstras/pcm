cat Godeps/Godeps.json | grep ImportPath | cut -d'"' -f4 | grep golang.org | xargs godep update
