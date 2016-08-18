package xterm

import "os"

type Pos struct {
	x, y int
}

type terminal struct {
	stdin, stdout, stderr                   chan []byte
	stdinStatus, stdoutStatus, stderrStatus chan readWriteMsg

	signals chan os.Signal
	exit    chan bool
	run     bool

	buffer           [][]rune
	cursor           Pos
	bottomLineOffset int

	renderEverything chan bool
	renderCell       chan Pos
	space            rune
}

type readWriteMsg struct {
	err error
	n   int
}
type ioForwarder struct {
	pipe   chan []byte
	status chan readWriteMsg
}

type byteAct struct {
	movement Pos
	draw     rune
}
