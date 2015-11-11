package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"

	"github.com/cfstras/pcm/Godeps/_workspace/src/github.com/cfstras/go-utils/color"
	"github.com/cfstras/pcm/Godeps/_workspace/src/github.com/kr/pty"
	"github.com/cfstras/pcm/Godeps/_workspace/src/github.com/renstrom/fuzzysearch/fuzzy"
	"github.com/cfstras/pcm/Godeps/_workspace/src/golang.org/x/crypto/ssh/terminal"
	"github.com/cfstras/pcm/Godeps/_workspace/src/golang.org/x/net/html/charset"
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

type Configuration struct {
	XMLName xml.Name  `xml:"configuration"`
	Root    Container `xml:"root"`

	AllConnections map[string]*Connection
}

type _node struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type Node interface {
}

type Container struct {
	_node

	Expanded    bool         `xml:"expanded,attr"`
	Containers  []Container  `xml:"container"`
	Connections []Connection `xml:"connection"`
}

type Connection struct {
	_node

	Expanded bool    `xml:""`
	Info     Info    `xml:"connection_info"`
	Login    Login   `xml:"login"`
	Timeout  Timeout `xml:"timeout"`
	Commands Command `xml:"command"`
	Options  Options `xml:"options"`
}

type Info struct {
	Name        string `xml:"name"`
	Protocol    string `xml:"protocol"`
	Host        string `xml:"host"`
	Port        uint16 `xml:"port"`
	Session     string `xml:"session"`
	Commandline string `xml:"commandline"`
	Description string `xml:"description"`
}

type Login struct {
	User     string `xml:"login"`
	Password string `xml:"password"`
}

// Timeouts are in milliseconds
type Timeout struct {
	ConnectionTimeout int `xml:"connectiontimeout"`
	LoginTimeout      int `xml:"logintimeout"`
	PasswordTimeout   int `xml:"passwordtimeout"`
	CommandTimeout    int `xml:"commandtimeout"`
}

type Command struct {
	Commands []string `xml:",any"`
}

type Options struct {
	LoginMacros  bool `xml:"loginmacros"`
	PostCommands bool `xml:"postCommands"`
	EndlineChar  int  `xml:"endlinechar"`
}

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

	pathP := flag.String("connectionsPath", connectionsPath, "Path to PuTTY connections.xml")
	verbose := false
	useFuzzySimple := false
	flag.BoolVar(&verbose, "verbose", false, "Display more info, such as hostnames and passwords")
	flag.BoolVar(&verbose, "v", false, "Display more info, such as hostnames and passwords")
	flag.BoolVar(&useFuzzySimple, "simple", false, "Use simple interface")

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

	var conn *Connection
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
	connect(conn)
}

func fuzzySimple(conf *Configuration, searchFor string) *Connection {
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

func connect(c *Connection) {
	cmd := &exec.Cmd{}
	cmd.Path = "/usr/bin/ssh"
	cmd.Args = []string{"-v", "-p", fmt.Sprint(c.Info.Port), "-l", c.Login.User, c.Info.Host}
	color.Yellowln(cmd.Path, cmd.Args)

	outFunc := func(pipe *os.File, name string, hasNextCommand func() bool,
		nextCommand func() string) {
		buf := make([]byte, 1024)
		for {
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				return
			}
			n, err := pipe.Read(buf)
			str := string(buf[:n])
			fmt.Print(str)
			if hasNextCommand() && (strings.HasSuffix(str, "assword: ") || strings.HasSuffix(str, "$ ") ||
				strings.HasSuffix(str, "# ")) {
				answer := nextCommand()
				//fmt.Println("writing", answer)
				pipe.Write([]byte(answer))
				pipe.Write([]byte("\n"))
			}
			if strings.HasSuffix(str, "Are you sure you want to continue connecting (yes/no)? ") {
				pipe.Write([]byte("yes\n"))
			}
			if err != nil {
				if err != io.EOF {
					return
				}
				fmt.Println("pipe", name, "error", err)
				return
			}
		}
	}

	stopWaiting := func(startWait chan<- bool) {
		select {
		case startWait <- true:
		default:
		}
	}

	inFunc := func(pipe io.WriteCloser, startWait chan bool) {
		input := make(chan []byte, 32)
		buffers := make(chan []byte, 32)
		for i := 0; i < 32; i++ {
			buffers <- make([]byte, 1024)
		}
		go func(pipe io.WriteCloser, input chan []byte, startWait chan bool, buffers chan []byte) {
			for {
				buf := <-buffers
				n, err := os.Stdin.Read(buf)
				//fmt.Println("my stdin got", n, string(buf[:n]))

				if err != nil && err != io.EOF {
					fmt.Println("my stdin got error", err)
					input <- nil
					stopWaiting(startWait)
					return
				}
				var write []byte
				if err == io.EOF {
					write = []byte{0x04}
					input <- nil
					stopWaiting(startWait)
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
						fmt.Println("stdin got error", err)
						input <- nil
						stopWaiting(startWait)
						return
					}
				}
				input <- buf[:n]
			}
		}(pipe, input, startWait, buffers)

		<-startWait
		for {
			buf := <-input
			defer func(buf []byte) {
				if cap(buf) > 256 {
					select {
					case buffers <- buf:
					}
				}
			}(buf)
			if buf == nil {
				fmt.Println("closing stdIn:", pipe.Close())
				return
			}
			_, err := pipe.Write(buf)

			if err != nil {
				fmt.Println("stdin got error", err)
				fmt.Println("closing stdIn:", pipe.Close())
				return
			}
		}
	}

	startWait := make(chan bool)
	state := 0
	hasNextCommand := func() bool {
		return state < 1+len(c.Commands.Commands)
	}
	nextCommand := func() string {
		if !hasNextCommand() {
			return ""
		}
		defer func() {
			state++
			if state >= len(c.Commands.Commands)+1 {
				stopWaiting(startWait)
			}
		}()
		if state == 0 {
			return c.Login.Password
		}
		return c.Commands.Commands[state-1]
	}

	sendSize := func(out *os.File, cmd *exec.Cmd) {
		var row, col C.int
		C.getsize(&row, &col)
		C.setsize(C.int(out.Fd()), row, col)
		cmd.Process.Signal(syscall.SIGWINCH)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGWINCH)

	signalWatcher := func(out *os.File, cmd *exec.Cmd) {
		for s := range signals {
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
	oldState, err := terminal.MakeRaw(0)
	p(err, "making terminal raw")
	defer terminal.Restore(0, oldState)

	go outFunc(pty, "pty", hasNextCommand, nextCommand)
	go inFunc(pty, startWait)
	go signalWatcher(pty, cmd)
	sendSize(pty, cmd)

	err = cmd.Wait()
	if err != nil {
		fmt.Println(err)
	}
}

func DummyReader(label string, input io.Reader) (io.Reader, error) {
	return input, nil
}

func loadConns() (result Configuration) {
	filename := connectionsPath
	if strings.Contains(filename, "~") {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			panic("Error: $HOME not set")
		}
		filename = strings.Replace(filename, "~", homeDir+"/", -1)
	}
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

func deleteEmptyCommands(conf *Configuration) {
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

func treePrint(target *[]string, index map[int]Node, node *Container) {
	if node == nil {
		return
	}
	treeDescend(target, index, "", "/", node)
	return
}

func treeDescend(target *[]string, index map[int]Node, prefix string,
	pathPrefix string, node *Container) {
	if !node.Expanded {
		return
	}
	for i := range node.Containers {
		nextCont := &node.Containers[i]
		nextPathPrefix := pathPrefix + nextCont.Name + "/"

		var nodeSym string
		var newPrefix string
		var expand string
		if i == len(node.Containers)-1 {
			if len(node.Connections) > 0 {
				nodeSym = "┡"
				newPrefix = "│ "
			} else {
				nodeSym = "┗"
				newPrefix = "  "
			}
		} else {
			if len(node.Containers) > 0 {
				nodeSym = "┣"
				newPrefix = "┃ "
			} else if len(node.Containers) == 0 {
				nodeSym = "┗"
				newPrefix = "  "
			} else {
				nodeSym = "┣"
				newPrefix = "┃ "
			}
		}
		if nextCont.Expanded {
			if len(nextCont.Containers) > 0 {
				expand = "━┓ ▼ "
			} else {
				expand = "━┑ ▼ "
			}
		} else {
			expand = "━┅ ▶ "
		}
		index[len(*target)] = nextCont
		*target = append(*target, prefix+nodeSym+expand+nextCont.Name)
		treeDescend(target, index, prefix+newPrefix, nextPathPrefix, nextCont)
	}
	for i := range node.Connections {
		conn := &node.Connections[i]

		var nodeSym string
		if i == len(node.Connections)-1 {
			nodeSym = "└"
		} else if len(node.Connections) != 0 {
			nodeSym = "├"
		} else {
			nodeSym = "┌"
		}
		index[len(*target)] = conn
		*target = append(*target, prefix+nodeSym+"─ "+conn.Name)
	}
}

func listConnections(config *Configuration,
	includeDescription bool) map[string]*Connection {

	conns := make(map[string]*Connection)
	descendConnections("", &config.Root, conns, includeDescription)
	return conns
}

func descendConnections(prefix string, node *Container,
	conns map[string]*Connection, includeDescription bool) {
	for i := range node.Connections {
		c := &node.Connections[i]
		key := prefix + "/" + c.Name
		if includeDescription {
			key += "  " + c.Info.Description
		}
		conns[key] = c
	}
	for i := range node.Containers {
		n := &node.Containers[i]
		descendConnections(prefix+"/"+n.Name, n, conns,
			includeDescription)
	}
}

func listWords(conns map[string]*Connection) []string {
	words := make([]string, 0, len(conns))
	for k := range conns {
		words = append(words, k)
	}
	return words
}
