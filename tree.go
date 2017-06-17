package main

import (
	"strings"

	"github.com/cfstras/pcm/types"
)

func treePrint(target *[]string, index map[int]types.Node, pathToIndexMap map[string]int,
	node *types.Container, width int) {
	if node == nil {
		return
	}
	treeDescend(target, index, pathToIndexMap, "", "/", node, width)
	return
}

func treeDescend(target *[]string, index map[int]types.Node, pathToIndexMap map[string]int,
	prefix string, pathPrefix string, node *types.Container, width int) {

	if !node.Expanded {
		return
	}
	for i := range node.Containers {
		nextCont := &node.Containers[i]
		nextPathPrefix := pathPrefix + nextCont.Name + "/"

		var nodeSym string
		var newPrefix string
		var expand string
		if i == len(node.Containers)-1 {
			if len(node.Connections) > 0 {
				nodeSym = "┡"
				newPrefix = "│ "
			} else {
				nodeSym = "┗"
				newPrefix = "  "
			}
		} else {
			if len(node.Containers) > 0 {
				nodeSym = "┣"
				newPrefix = "┃ "
			} else if len(node.Containers) == 0 {
				nodeSym = "┗"
				newPrefix = "  "
			} else {
				nodeSym = "┣"
				newPrefix = "┃ "
			}
		}
		if nextCont.Expanded {
			if len(nextCont.Containers) > 0 {
				expand = "━┓ ▼ "
			} else {
				expand = "━┑ ▼ "
			}
		} else {
			expand = "━┅ ▶ "
		}
		index[len(*target)] = nextCont
		if pathToIndexMap != nil {
			pathToIndexMap[nextCont.Path()] = len(*target)
		}
		str := prefix + nodeSym + expand + nextCont.Name
		nextCont.TreeView = str

		spaces := width - len([]rune(str)) - len([]rune(nextCont.StatusInfo))
		if spaces > 0 {
			str += strings.Repeat(" ", spaces)
		}
		str += nextCont.StatusInfo

		*target = append(*target, str)
		treeDescend(target, index, pathToIndexMap, prefix+newPrefix,
			nextPathPrefix, nextCont, width)
	}
	for i := range node.Connections {
		conn := &node.Connections[i]
		var nodeSym string
		if i == len(node.Connections)-1 {
			nodeSym = "└"
		} else if len(node.Connections) != 0 {
			nodeSym = "├"
		} else {
			nodeSym = "┌"
		}
		index[len(*target)] = conn
		if pathToIndexMap != nil {
			pathToIndexMap[conn.Path()] = len(*target)
		}
		str := prefix + nodeSym + "─ " + conn.Name
		conn.TreeView = str

		spaces := width - len([]rune(str)) - len([]rune(conn.StatusInfo))
		if spaces > 0 {
			str += strings.Repeat(" ", spaces)
		}
		str += conn.StatusInfo

		*target = append(*target, str)
	}
}

// get all keys of the map as a slice
func listWords(conns map[string]*types.Connection) []string {
	words := make([]string, 0, len(conns))
	for k := range conns {
		words = append(words, k)
	}
	return words
}
