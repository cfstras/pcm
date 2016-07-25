// +build !windows

package util

import (
	"os"
	"syscall"
)

func GetSigwinch() os.Signal {
	return syscall.SIGWINCH
}
