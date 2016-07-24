// +build windows

package util

import (
	"bytes"
	"os"
	"syscall"
	"unsafe"
)

func TransformInput(s []byte) []byte {
	return bytes.Replace(s, []byte{'\r', '\n'}, []byte{'\r'}, -1)
}

const (
	enableLineInput       = 2
	enableEchoInput       = 4
	enableProcessedInput  = 1
	enableWindowInput     = 8
	enableMouseInput      = 16
	enableInsertMode      = 32
	enableQuickEditMode   = 64
	enableExtendedFlags   = 128
	enableAutoPosition    = 256
	enableProcessedOutput = 1
	enableWrapAtEolOutput = 2
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type State struct {
	mode uint32
}

func SetupTerminal() (*State, error) {
	fd := os.Stdout.Fd()
	var st uint32
	_, _, e := syscall.Syscall(procGetConsoleMode.Addr(), 2, fd, uintptr(unsafe.Pointer(&st)), 0)
	if e != 0 {
		return nil, error(e)
	}
	//raw := st &^ (enableEchoInput | enableProcessedInput | enableLineInput | enableProcessedOutput)
	raw := st &^ (enableEchoInput | enableProcessedInput | enableLineInput)
	raw = raw | enableProcessedOutput
	_, _, e = syscall.Syscall(procSetConsoleMode.Addr(), 2, fd, uintptr(raw), 0)
	if e != 0 {
		return nil, error(e)
	}
	return &State{st}, nil
}
func RestoreTerminal(state *State) error {
	fd := os.Stdout.Fd()
	_, _, err := syscall.Syscall(procSetConsoleMode.Addr(), 2, uintptr(fd), uintptr(state.mode), 0)
	return err
}
