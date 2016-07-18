package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"golang.org/x/text/encoding/unicode"

	"github.com/cfstras/go-utils/color"
	"github.com/cfstras/pcm/ssh"
	"github.com/cfstras/pcm/types"
	"github.com/kr/pty"
	"github.com/renstrom/fuzzysearch/fuzzy"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/html/charset"
)

/*
#include <sys/ioctl.h>
#include <unistd.h>

void getsize(int* rows, int* cols) {
	struct winsize w;
	ioctl(STDOUT_FILENO, TIOCGWINSZ, &w);
	*rows = w.ws_row;
	*cols = w.ws_col;
	//printf ("lines %d\n", w.ws_row);
	//printf ("columns %d\n", w.ws_col);
	//printf ("pixels x %d\n", w.ws_xpixel);
	//printf ("pixels y %d\n", w.ws_ypixel);
}
void setsize(int fd, int rows, int cols) {
	struct winsize w;
	ioctl(fd, TIOCGWINSZ, &w);
	w.ws_row = rows;
	w.ws_col = cols;
	ioctl(fd, TIOCSWINSZ, &w);
	//printf ("lines %d\n", w.ws_row);
	//printf ("columns %d\n", w.ws_col);
}

*/
import "C"

var (
	connectionsPath string = "~/Downloads/connections.xml"
)

type StringList []string

func (l StringList) Len() int           { return len(l) }
func (l StringList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l StringList) Less(i, j int) bool { return strings.Compare(l[i], l[j]) < 0 }

const DEBUG = false

func main() {
	defer func() {
		if err := recover(); err != nil {
			color.Redln(err)
			if DEBUG {
				debug.PrintStack()
			}
			os.Exit(1)
		}
	}()
	if DEBUG {
		go http.ListenAndServe(":3000", nil)
	}

	pathP := flag.String("connectionsPath", connectionsPath, "Path to PuTTY connections.xml")
	verbose := false
	useFuzzySimple := false
	useOwnSSH := false
	flag.BoolVar(&verbose, "verbose", false, "Display more info, such as hostnames and passwords")
	flag.BoolVar(&verbose, "v", false, "Display more info, such as hostnames and passwords")
	flag.BoolVar(&useFuzzySimple, "simple", false, "Use simple interface")
	flag.BoolVar(&useOwnSSH, "ssh", true, "Use golang ssh client instead of os-client")

	flag.Parse()
	if pathP != nil {
		connectionsPath = *pathP
	}

	if flag.NArg() > 1 {
		flag.Usage()
		color.Yellowln("Usage: pcm [search term]")
		panic("Only one arg allowed.")
	}
	var searchFor string
	if flag.NArg() == 1 {
		searchFor = flag.Arg(0)
	}

	conf := loadConns()

	var conn *types.Connection
	if useFuzzySimple {
		conn = fuzzySimple(&conf, searchFor)
	} else {
		conn = selectConnection(&conf, searchFor)
	}

	if conn == nil {
		return
	}

	color.Yellowln("Using", conn.Info.Name)
	color.Redln(conn.Info.Host, conn.Info.Port)
	if verbose {
		color.Yellow("User: ")
		fmt.Println(conn.Login.User)
		color.Yellow("Password: ")
		fmt.Println(conn.Login.Password)
		color.Yellowln("Commands:")
		for _, v := range conn.Commands.Commands {
			fmt.Println(v)
		}
	}
	//fmt.Println(conn.Login)
	//fmt.Println(conn.Command)

	console := &consoleTerminal{
		exit: make(chan bool),
	}
	oldState, err := terminal.MakeRaw(0)
	p(err, "making terminal raw")
	defer terminal.Restore(0, oldState)
	var changed bool
	if useOwnSSH {
		changed = ssh.Connect(conn, console, func() *string { return nil })
	} else {
		changed = connect(conn, console, func() *string { return nil })
	}
	if changed {
		saveConns(&conf)
	}
}

type consoleTerminal struct {
	exit    chan bool
	signals chan os.Signal
}

func (c *consoleTerminal) GetSize() (width, height int, err error) {
	return terminal.GetSize(int(os.Stdout.Fd()))
}
func (c *consoleTerminal) Stdin() io.Reader {
	return os.Stdin
}
func (c *consoleTerminal) Stdout() io.Writer {
	return os.Stdout
}
func (c *consoleTerminal) Stderr() io.Writer {
	return os.Stderr
}
func (c *consoleTerminal) ExitRequests() <-chan bool {
	return c.exit
}
func (c *consoleTerminal) Signals() <-chan os.Signal {
	if c.signals == nil {
		c.signals = make(chan os.Signal, 1)
		signal.Notify(c.signals, os.Interrupt, syscall.SIGWINCH)
	}
	return c.signals
}

func fuzzySimple(conf *types.Configuration, searchFor string) *types.Connection {
	words := listWords(conf.AllConnections)

	reader := bufio.NewReader(os.Stdin)
	var found string
	for {
		color.Yellow("Search for: ")
		var input string
		var err error
		if searchFor != "" {
			input = searchFor
			searchFor = ""
		} else {
			input, err = reader.ReadString('\n')
			p(err, "reading stdin")
			input = strings.Trim(input, "\r\n ")
		}
		if input == "" {
			for _, v := range words {
				sort.Stable(StringList(words))
				fmt.Println(v)
			}
			continue
		}
		suggs := fuzzy.RankFindFold(input, words)
		if len(suggs) > 1 {
			sort.Stable(suggs)
			color.Yellowln("Suggestions:")
			for _, v := range suggs {
				fmt.Println(v.Target)
			}
			color.Redln("Please try again.")
		} else if len(suggs) == 0 {
			color.Redln("Nothing found for", input+". Please try again.")
		} else {
			found = suggs[0].Target
			break
		}
	}
	conn := conf.AllConnections[found]
	return conn
}

func connect(c *types.Connection, terminal types.Terminal, moreCommands func() *string) bool {

	cmd := &exec.Cmd{}
	cmd.Path = "/usr/bin/ssh"
	cmd.Args = []string{"-v", "-p", fmt.Sprint(c.Info.Port), "-l", c.Login.User, c.Info.Host}
	var procExit int32 = 0

	outFunc := func(pipe *os.File, name string, nextCommand func() *string,
		startWait *sync.Cond) {
		buf := make([]byte, 1024)
		for {
			if atomic.LoadInt32(&procExit) != 0 {
				return
			}
			n, err := pipe.Read(buf)
			str := string(buf[:n])
			fmt.Fprint(terminal.Stdout(), str)
			if strings.HasSuffix(str, "assword: ") || strings.HasSuffix(str, "$ ") ||
				strings.HasSuffix(str, "# ") {
				if answer := nextCommand(); answer != nil {
					pipe.Write([]byte(*answer))
					pipe.Write([]byte("\n"))
				}
			}
			if strings.HasSuffix(str, "Are you sure you want to continue connecting (yes/no)? ") {
				pipe.Write([]byte("yes\n"))
			}
			if err != nil {
				if err != io.EOF {
					return
				}
				fmt.Fprintln(terminal.Stderr(), "pipe", name, "error", err)
				return
			}
		}
	}

	inFunc := func(pipe io.WriteCloser, inputConsole io.Reader, startWait *sync.Cond) {
		input := make(chan []byte, 32)
		buffers := &sync.Pool{
			New: func() interface{} { return make([]byte, 1024) },
		}

		go func(pipe io.WriteCloser, input chan []byte, startWait *sync.Cond,
			buffers *sync.Pool) {
			for {
				buf := buffers.Get().([]byte)
				n, err := inputConsole.Read(buf)

				if err != nil && err != io.EOF {
					fmt.Fprintln(terminal.Stderr(), "my stdin got error", err)
					input <- nil
					startWait.Broadcast()
					return
				}
				var write []byte
				if err == io.EOF {
					write = []byte{0x04}
					input <- nil
					startWait.Broadcast()
					return
				}
				for _, c := range []byte{0x04, 0x03, 0x1a} {
					if bytes.Contains(buf[:n], []byte{c}) {
						write = append(write, c)
					}
				}
				if write != nil {
					_, err = pipe.Write(write)
					if err != nil {
						fmt.Fprintln(terminal.Stderr(), "stdin got error", err)
						input <- nil
						startWait.Broadcast()
						return
					}
				} else {
					input <- buf[:n]
				}
			}
		}(pipe, input, startWait, buffers)

		startWait.L.Lock()
		startWait.Wait()
		startWait.L.Unlock()
		for buf := range input {
			if buf == nil {
				fmt.Fprintln(terminal.Stderr(), "closing stdIn:", pipe.Close())
				return
			}
			_, err := pipe.Write(buf)

			if err != nil {
				fmt.Fprintln(terminal.Stderr(), "stdin got error", err)
				fmt.Fprintln(terminal.Stderr(), "closing stdIn:", pipe.Close())
				return
			}
			if cap(buf) > 256 {
				buffers.Put(buf)
			}
		}
	}

	startWait := sync.NewCond(&sync.Mutex{})

	commands := make(chan string, len(c.Commands.Commands)+2)
	commands <- c.Login.Password
	for _, v := range c.Commands.Commands {
		if v != "" {
			commands <- v
		}
	}
	nextCommand := func() *string {
		if len(commands) > 1 {
			v := <-commands
			return &v
		} else if c := moreCommands(); c != nil {
			return c
		} else {
			startWait.Broadcast()
			return nil
		}
	}

	sendSize := func(out *os.File, cmd *exec.Cmd) {
		var row, col C.int
		C.getsize(&row, &col)
		C.setsize(C.int(out.Fd()), row, col)
		cmd.Process.Signal(syscall.SIGWINCH)
	}

	signalWatcher := func(out *os.File, cmd *exec.Cmd) {
		for s := range terminal.Signals() {
			if s == syscall.SIGWINCH {
				sendSize(out, cmd)
			} else if s == os.Interrupt {
				cmd.Process.Signal(syscall.SIGINT)
			} else if s == syscall.SIGSTOP {
				cmd.Process.Signal(syscall.SIGSTOP)
			}
		}
	}

	pty, err := pty.Start(cmd)
	p(err, "starting ssh")

	go outFunc(pty, "pty", nextCommand, startWait)
	go inFunc(pty, terminal.Stdin(), startWait)
	go signalWatcher(pty, cmd)
	sendSize(pty, cmd)

	go func(exit <-chan bool) {
		for _ = range exit {
			if atomic.LoadInt32(&procExit) != 0 {
				return
			}
			err := cmd.Process.Kill()
			p(err, "Killing ssh process")
			return
		}
	}(terminal.ExitRequests())

	err = cmd.Wait()
	atomic.StoreInt32(&procExit, 1)
	if err != nil {
		fmt.Fprintln(terminal.Stderr(), "SSH Process ended:", err)
	}
	return false
}

func DummyReader(label string, input io.Reader) (io.Reader, error) {
	return input, nil
}

func replaceHome(in string) string {
	if strings.Contains(in, "~") {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			panic("Error: $HOME not set")
		}
		in = strings.Replace(in, "~", homeDir+"/", -1)
	}
	return in
}

func loadConns() (result types.Configuration) {
	filename := replaceHome(connectionsPath)
	rd, err := os.Open(filename)
	p(err, "opening "+filename)
	defer rd.Close()
	rd2, err := charset.NewReaderLabel("utf-16", rd)
	p(err, "loading charset")

	decoder := xml.NewDecoder(rd2)
	decoder.CharsetReader = DummyReader
	p(decoder.Decode(&result), "decoding xml")

	result.AllConnections = listConnections(&result, false)
	deleteEmptyCommands(&result)

	result.Root.Expanded = true
	return
}

func saveConns(conf *types.Configuration) {
	filename := replaceHome(connectionsPath)
	tmp := filename + ".tmp"
	wr, err := os.Create(tmp)
	p(err, "opening "+filename)
	defer wr.Close()
	defer p(os.Rename(tmp, filename), "overwriting connections.xml")

	encoding := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM)
	textEncoder := encoding.NewEncoder()

	encoder := xml.NewEncoder(textEncoder.Writer(wr))
	encoder.Indent("", "  ")
	p(encoder.Encode(&conf), "encoding xml")
}

func deleteEmptyCommands(conf *types.Configuration) {
	for _, conn := range conf.AllConnections {
		n := []string{}
		for _, v := range conn.Commands.Commands {
			if strings.TrimSpace(v) != "" {
				n = append(n, v)
			}
		}
		conn.Commands.Commands = n
	}
}

func p(err error, where string) {
	if err != nil {
		color.Yellow("When " + where + ": ")
		panic(err)
	}
}
