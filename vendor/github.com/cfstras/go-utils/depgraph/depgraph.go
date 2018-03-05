// This is a simple command line tool to create a dependency graph from a path.
//
// Patches welcome.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var stdLib []string = getStdLib()

type goList struct {
	Imports []string
}

var done = make(map[string]bool)

func getDeps(p string) []string {
	if containsString(ignored, p) {
		return []string{}
	}
	o, err := exec.Command("go", "list", "-json", p).Output()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s for %s", err, p)
	}

	list := goList{}
	json.Unmarshal(o, &list)

	return list.Imports
}

func printRecursive(p string) {
	done[p] = true

	for _, d := range getDeps(p) {
		spl := strings.Split(d, "/")
		if containsString(stdLib, spl[0]) {
			continue
		}
		fmt.Printf("\t\"%s\" -> \"%s\";\n", p, d)

		if !done[d] {
			printRecursive(d)
		}
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, `Usage:
	depgraph <root package> | dot -Tsvg > graph.svg`)
		return
	}

	fmt.Println("digraph G {")
	printRecursive(os.Args[1])
	fmt.Println("}")
}

var ignored = []string{"C"}

// this is an ugly hack, I know.
// TODO make this prettier
func getStdLib() []string {
	return []string{
		"bufio",
		"bytes",
		"crypto",
		"database",
		"encoding",
		"errors",
		"flag",
		"fmt",
		"hash",
		"html",
		"html",
		"io",
		"log",
		"math",
		"math/big",
		"net",
		"net/http",
		"os",
		"path",
		"reflect",
		"regexp",
		"runtime",
		"sort",
		"strconv",
		"strings",
		"sync",
		"syscall",
		"testing",
		"time",
		"unicode",
		"unsafe"}
}

func containsString(haystack []string, needle string) bool {
	for _, hay := range haystack {
		if hay == needle {
			return true
		}
	}
	return false
}
