// +build windows

package main

import (
	"github.com/cfstras/go-utils/color"
	"github.com/cfstras/pcm/types"
)

func connect(c *types.Connection, terminal types.Terminal, moreCommands func() *string) bool {
	color.Redln("Not implemented on windows")
	return false
}
