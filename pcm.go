package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strings"

	"github.com/cfstras/go-utils/color"
	"github.com/kr/pty"
	"github.com/renstrom/fuzzysearch/fuzzy"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/html/charset"
)

type Configuration struct {
	XMLName xml.Name  `xml:"configuration"`
	Root    Container `xml:"root"`

	AllConnections map[string]Connection
}
type Container struct {
	Type        string       `xml:"type,attr"`
	Name        string       `xml:"name,attr"`
	expanded    bool         `xml:"expanded, attr"`
	Containers  []Container  `xml:"container"`
	Connections []Connection `xml:"connection"`
}
type Connection struct {
	Type string `xml:"type,attr"`
	Name string `xml:"name,attr"`

	Info    Info    `xml:"connection_info"`
	Login   Login   `xml:"login"`
	Timeout Timeout `xml:"timeout"`
	Command Command `xml:"command"`
	//TODO Options
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

type Timeout struct {
	ConnectionTimeout int `xml:"connectiontimeout"`
	LoginTimeout      int `xml:"logintimeout"`
	PasswordTimeout   int `xml:"passwordtimeout"`
	CommandTimeout    int `xml:"commandtimeout"`
}

type Command struct {
	Commands []string `xml:",any"`
}

var (
	connectionsPath string = "~/Downloads/connections.xml"
	conns           Configuration
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			color.Redln(err)
			os.Exit(1)
		}
	}()

	pathP := flag.String("connectionsPath", connectionsPath, "Path to PuTTY connections.xml")

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

	conns = loadConns()
	listConnections(&conns)
	//treePrint(conns)
	words := listWords(&conns)

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
		suggs := fuzzy.Find(input, words)
		if len(suggs) > 1 {
			color.Yellowln("Suggestions:")
			for _, v := range suggs {
				fmt.Println(v)
			}
			color.Redln("Please try again.")
		} else if len(suggs) == 0 {
			color.Redln("Nothing found for", input+". Please try again.")
		} else {
			found = suggs[0]
			break
		}
	}
	color.Yellowln("Using", found)

	conn := conns.AllConnections[found]
	fmt.Println(conn.Info)
	//fmt.Println(conn.Login)
	//fmt.Println(conn.Command)
	connect(conn)
}

func connect(c Connection) {
	cmd := &exec.Cmd{}
	cmd.Path = "/usr/bin/ssh"
	cmd.Args = []string{"-v", "-p", fmt.Sprint(c.Info.Port), "-l", c.Login.User, c.Info.Host}
	color.Yellowln(cmd.Path, cmd.Args)

	outFunc := func(pipe *os.File, name string, nextCommand func() string) {
		buf := make([]byte, 1024)
		for {
			n, err := pipe.Read(buf)
			str := string(buf[:n])
			fmt.Print(str)
			if strings.HasSuffix(str, "assword: ") || strings.HasSuffix(str, "$ ") ||
				strings.HasSuffix(str, "# ") {
				answer := nextCommand()
				if answer != "" {
					//fmt.Println("writing", answer)
					pipe.Write([]byte(answer))
					pipe.Write([]byte("\n"))
				}
			}
			if err != nil {
				fmt.Println("pipe", name, "error", err)
				fmt.Println("closing pipe", name, pipe.Close())
				return
			}
		}
	}

	inFunc := func(pipe io.WriteCloser) {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			//fmt.Println("my stdin got", n, string(buf[:n]))
			if err == io.EOF {
				pipe.Write([]byte{0x04})
			}
			if err != nil {
				fmt.Println("my stdin got error", err)
				fmt.Println("closing stdIn:", pipe.Close())
				return
			}
			_, err = pipe.Write(buf[:n])

			if err != nil {
				fmt.Println("stdin got error", err)
				fmt.Println("closing stdIn:", pipe.Close())
				return
			}
		}
	}

	state := 0
	nextCommand := func() string {
		defer func() { state++ }()
		if state == 0 {
			return c.Login.Password
		}
		ind := state - 1
		if len(c.Command.Commands) > ind {
			return c.Command.Commands[ind]
		}
		return ""
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	signalWatcher := func(out *os.File) {
		for _ = range signals {
			out.Write([]byte{0x03})
		}
	}

	pty, err := pty.Start(cmd)
	p(err, "starting ssh")
	oldState, err := terminal.MakeRaw(0)
	p(err, "making terminal raw")
	defer terminal.Restore(0, oldState)

	go outFunc(pty, "pty", nextCommand)
	go inFunc(pty)
	go signalWatcher(pty)

	err = cmd.Wait()
	if err != nil {
		fmt.Println(err)
	}
}

func DummyReader(label string, input io.Reader) (io.Reader, error) {
	return input, nil
}

func loadConns() (result Configuration) {
	u, err := user.Current()
	p(err, "getting user")
	filename := strings.Replace(connectionsPath, "~", u.HomeDir, -1)
	rd, err := os.Open(filename)
	p(err, "opening "+filename)
	defer rd.Close()
	rd2, err := charset.NewReaderLabel("utf-16", rd)
	p(err, "loading charset")

	decoder := xml.NewDecoder(rd2)
	decoder.CharsetReader = DummyReader
	p(decoder.Decode(&result), "decoding xml")

	return
}

func p(err error, where string) {
	if err != nil {
		color.Yellow("When " + where + ": ")
		panic(err)
	}
}

func treePrint(conns Configuration) {
	treeDescend(0, conns.Root)
}
func treeDescend(depth int, node Container) {
	for i, nextCont := range node.Containers {
		var nodeSym string
		if i == 0 {
			nodeSym = "┣"
		} else if i == len(node.Containers)-1 {
			if len(node.Connections) > 0 {
				nodeSym = "┡"
			} else {
				nodeSym = "┗"
			}
		} else {
			nodeSym = "┣"
		}
		fmt.Print(strings.Repeat("  ", depth), nodeSym, "━┓ ", nextCont.Name, "\n")
		treeDescend(depth+1, nextCont)
	}
	for i, conn := range node.Connections {
		var nodeSym string
		if i == len(node.Connections)-1 {
			nodeSym = "└"
		} else if len(node.Containers) != 0 {
			nodeSym = "├"
		} else {
			nodeSym = "┌"
		}
		fmt.Print(strings.Repeat("  ", depth), nodeSym, "─ ", conn.Name, "\n")
	}
}

func listConnections(conns *Configuration) {
	conns.AllConnections = make(map[string]Connection)
	descendConnections("", conns.Root, conns)
}

func descendConnections(prefix string, node Container, conns *Configuration) {
	for _, c := range node.Connections {
		key := prefix + "/" + c.Name
		conns.AllConnections[key] = c
	}
	for _, n := range node.Containers {
		descendConnections(prefix+"/"+n.Name, n, conns)
	}
}

func listWords(conns *Configuration) []string {
	words := make([]string, 0, len(conns.AllConnections))
	for k := range conns.AllConnections {
		words = append(words, k)
	}
	return words
}
