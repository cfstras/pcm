package xterm

import (
	"io"
	_log "log"
	"math"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/cfstras/pcm/util"

	"github.com/cfstras/go-utils/color"
	"github.com/cfstras/pcm/types"
	termbox "github.com/nsf/termbox-go"
)

var (
	log     *_log.Logger
	logfile *os.File
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
		run:     true,

		renderEverything: make(chan bool, 1),
		renderCell:       make(chan Pos, 10),
		space:            ' ',
	}
	return t
}
func (t *terminal) Start() error {
	logfile, err := os.OpenFile("pcm-xterm.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	log = _log.New(logfile,
		"", _log.Ltime|_log.LUTC|_log.Lshortfile)

	osSignals := make(chan os.Signal)
	go func() {
		for _ = range osSignals {
			t.exit <- true
		}
	}()
	signal.Notify(osSignals, syscall.SIGTERM)

	err = termbox.Init()
	if err != nil {
		return err
	}
	go t.inputHandler()
	go t.outputHandler()
	go t.renderer()
	return nil
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
	if logfile != nil {
		logfile.Close()
		logfile = nil
	}
	return nil
}

// Handles input events from the console window and sends them to the stdin pipe
func (t *terminal) inputHandler() {
	defer panicHandler()
	//TODO
	for b := range t.stdin {
		for {
			event := termbox.PollEvent()
			switch event.Type {
			case termbox.EventError:
				t.stdinStatus <- readWriteMsg{event.Err, 0}
				break
			case termbox.EventKey:
				var key byte = byte(event.Key)
				b[0] = key
				t.stdinStatus <- readWriteMsg{nil, 1}
			case termbox.EventResize:
				t.signals <- util.GetSigwinch()
				// no input, go on
			case termbox.EventMouse:
				// ignore mouse
				log.Print("Mouse: ", event.MouseX, event.MouseY)
			default:
				log.Println("unknown event", event)
			}
		}
	}
}

// Handles output from the SSH process and writes them into the buffer
func (t *terminal) outputHandler() {
	defer panicHandler()
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
	log.Println("output:", string(buf))
	for _, b := range buf {
		movement, draw := t.displayMapping(b)
		for len(t.buffer) < t.cursor.y+1 {
			t.buffer = append(t.buffer, []rune{})
		}
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
			//for len(t.buffer) <= t.cursor.y {
			//	t.buffer = append(t.buffer, []rune{t.space})
			//}
		}
	}
	t.renderEverything <- true
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
	log.Println("Mapping", string([]rune{rune(c)}), "to", act.movement,
		string([]rune{act.draw}))
	return act.movement, act.draw
	//TODO
}

func (t *terminal) renderer() {
	defer panicHandler()
	for {
		select {
		case <-t.renderEverything:
			log.Println("rendering everything")
			for y, line := range t.buffer {
				log.Println("rendering", y, string(line))
				for x, char := range line {
					termbox.SetCell(x, y, char, termbox.ColorDefault, termbox.ColorDefault)
				}
			}
			err := termbox.Flush()
			if err != nil {
				log.Println("sync:", err)
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

func panicHandler() {
	if err := recover(); err != nil {
		if termbox.IsInit {
			termbox.Close()
		}
		color.Redln(err)
		debug.PrintStack()
		os.Exit(1)
	}
}
