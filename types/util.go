package types

import (
	"io"
	"os"
)

type Terminal interface {
	GetSize() (width, height int, err error)
	Stdin() io.Reader
	Stdout() io.Writer
	Stderr() io.Writer
	ExitRequests() <-chan bool
	Signals() <-chan os.Signal
}
