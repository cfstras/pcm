// +build !windows

package util

import (
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

func TransformInput(s []byte) []byte {
	return s
}

type State terminal.State

func SetupTerminal() (*State, error) {
	s, err := terminal.MakeRaw(int(os.Stdout.Fd()))
	return (*State)(s), err
}
func RestoreTerminal(state *State) error {
	return terminal.Restore(int(os.Stdout.Fd()), (*terminal.State)(state))
}
