package xterm

import (
	"io"
	"math"
	"os"

	"github.com/cfstras/pcm/types"
	termbox "github.com/nsf/termbox-go"
)

func Terminal() types.Terminal {
	t := &terminal{
		stdin:        make(chan []byte),
		stdout:       make(chan []byte),
		stderr:       make(chan []byte),
		stdinStatus:  make(chan readWriteMsg),
		stdoutStatus: make(chan readWriteMsg),
		stderrStatus: make(chan readWriteMsg),

		signals: make(chan os.Signal),
		exit:    make(chan bool),

		renderEverything: make(chan bool, 1),
	}
	return t
}
func (t *terminal) Start() error {
	err := termbox.Init()

	go t.inputHandler()
	go t.outputHandler()
	go t.renderer()
	return err
}

func (t *terminal) GetSize() (width, height int, err error) {
	width, height = termbox.Size()
	err = nil
	return
}

func (c *channelReadWriter) Read(b []byte) (n int, err error) {
	c.pipe <- b
	status := <-c.status
	return status.n, status.err
}

func (c *channelReadWriter) Write(b []byte) (n int, err error) {
	return c.Read(b) // exactly the same method -- just passes the message
}

func (t *terminal) Stdin() io.Reader {
	return &channelReadWriter{t.stdin, t.stdinStatus}
}
func (t *terminal) Stdout() io.Writer {
	return &channelReadWriter{t.stdout, t.stdoutStatus}
}
func (t *terminal) Stderr() io.Writer {
	return &channelReadWriter{t.stderr, t.stderrStatus}
}
func (t *terminal) ExitRequests() <-chan bool {
	return t.exit
}
func (t *terminal) Signals() <-chan os.Signal {
	return t.signals
}
func (t *terminal) Close() error {
	if termbox.IsInit {
		termbox.Close()
	}
	return nil
}

// Handles input events from the console window and sends them to the stdin pipe
func (t *terminal) inputHandler() {
	//TODO
}

// Handles output from the SSH process and writes them into the buffer
func (t *terminal) outputHandler() {
	for t.run {
		select {
		case b := <-t.stdout:
			n, err := t.handleOutput(b)
			t.stdoutStatus <- readWriteMsg{err, n}
		case b := <-t.stderr:
			n, err := t.handleOutput(b)
			t.stderrStatus <- readWriteMsg{err, n}
		}
	}
}
func (t *terminal) handleOutput(buf []byte) (int, error) {
	for _, b := range buf {
		movement, draw := t.displayMapping(b)
		line := t.buffer[t.cursor.x]
		if len(line) <= t.cursor.y {
			line := append(line, draw)
			t.buffer[t.cursor.x] = line
		} else {
			line[t.cursor.y] = draw
		}
		if movement.x > 0 {
			for i := t.movement.x; i > 0; i-- {
				//TODO
			}
		}
	}
	return len(b), nil
}

var mappingTable [256]byteAct

func init() {
	mappingTable[0xd] = byteAct{Pos{-math.MaxInt32, 1}, 0}

	// map normal characters
	for i := ' '; i < '~'; i++ {
		mappingTable[i] = byteAct{Pos{1, 0}, i}
	}
}

func (t *terminal) displayMapping(c byte) (Pos, rune) {
	act := mappingTable[c]
	return act.movement, act.draw
	//TODO
}
