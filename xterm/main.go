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
		renderCell:       make(chan Pos, 10),
		space:            ' ',
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

func (c *ioForwarder) Read(b []byte) (n int, err error) {
	c.pipe <- b
	status := <-c.status
	return status.n, status.err
}

func (c *ioForwarder) Write(b []byte) (n int, err error) {
	return c.Read(b) // exactly the same method -- just passes the message
}

func (t *terminal) Stdin() io.Reader {
	return &ioForwarder{t.stdin, t.stdinStatus}
}
func (t *terminal) Stdout() io.Writer {
	return &ioForwarder{t.stdout, t.stdoutStatus}
}
func (t *terminal) Stderr() io.Writer {
	return &ioForwarder{t.stderr, t.stderrStatus}
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
		line := t.buffer[t.cursor.y]
		if len(line) <= t.cursor.x {
			line := append(line, draw)
			t.buffer[t.cursor.y] = line
		} else {
			line[t.cursor.x] = draw
		}
		if movement.x != 0 {
			if movement.x == math.MaxInt32 {
				t.cursor.x = len(line)
			} else if movement.x == -math.MaxInt32 {
				t.cursor.x = 0
			} else {
				t.cursor.x += movement.x
			}
			for len(line) <= t.cursor.x {
				line = append(line, t.space)
				t.buffer[t.cursor.y] = line
			}
		}
		if movement.y != 0 {
			if movement.y == math.MaxInt32 {
				t.cursor.y = len(line)
			} else if movement.y == -math.MaxInt32 {
				t.cursor.y = 0
			} else {
				t.cursor.y += movement.y
			}
			for len(t.buffer) <= t.cursor.y {
				t.buffer = append(t.buffer, []rune{t.space})
			}
		}
	}
	return len(buf), nil
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

func (t *terminal) renderer() {
	for {
		select {
		case <-t.renderEverything:
			for x, line := range t.buffer {
				for y, char := range line {
					termbox.SetCell(x, y, char, termbox.ColorDefault, termbox.ColorDefault)
				}
			}
		case pos := <-t.renderCell:
			t.renderCellNow(pos)
		}
	}
}
func (t *terminal) renderCellNow(pos Pos) {
	//TODO
	panic("not implemented")
}
