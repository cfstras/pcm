package util

import (
	"strings"
	"sync"

	"github.com/cfstras/pcm/types"
)

type CommandFunc func() *string

// Returns a function that will return any commands, or
// nil when no other commands are defined. It will also signal the passed
// Condition when if are no commands left and the Condition is not nil.
func GetCommandFunc(c *types.Connection, startWait *sync.Cond, moreCommands CommandFunc) CommandFunc {
	commands := make(chan string, 5) // command1-5
	//if c.Options.PostCommands { // official PCM ignores this flag... yay.
	for _, v := range []string{
		c.Commands.Command1, c.Commands.Command2, c.Commands.Command3,
		c.Commands.Command4, c.Commands.Command5} {
		if strings.TrimSpace(v) != "" {
			commands <- v
		}
	}
	//}
	return func() *string {
		select {
		case v := <-commands:
			return &v
		default:
			if c := moreCommands(); c != nil {
				return c
			} else if startWait != nil {
				startWait.Broadcast()
			}
			return nil
		}
	}
}
