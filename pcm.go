package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"sort"
	"strings"

	"golang.org/x/text/encoding/unicode"

	"github.com/cfstras/go-utils/color"
	"github.com/cfstras/go-utils/lock"
	"github.com/cfstras/pcm/ssh"
	"github.com/cfstras/pcm/types"
	"github.com/cfstras/pcm/util"
	"github.com/renstrom/fuzzysearch/fuzzy"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/html/charset"
)

var (
	connectionsPath string = "~/Downloads/connections.xml"
)

type StringList []string

func (l StringList) Len() int           { return len(l) }
func (l StringList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l StringList) Less(i, j int) bool { return strings.Compare(l[i], l[j]) < 0 }

var DEBUG = false

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
	useOwnSSH := false
	flag.BoolVar(&verbose, "verbose", false, "Display more info, such as hostnames and passwords")
	flag.BoolVar(&verbose, "v", false, "Display more info, such as hostnames and passwords")
	flag.BoolVar(&useFuzzySimple, "simple", false, "Use simple interface")
	flag.BoolVar(&useOwnSSH, "ssh", true, "Use golang ssh client instead of os-client")
	flag.BoolVar(&DEBUG, "debug", false, "enable debug server on :3000")
	flag.Parse()
	if pathP != nil {
		connectionsPath = *pathP
	}
	connectionsPath = replaceHome(connectionsPath)

	if flag.NArg() > 1 {
		flag.Usage()
		color.Yellowln("Usage: pcm [search term]")
		panic("Only one arg allowed.")
	}
	var searchFor string
	if flag.NArg() == 1 {
		searchFor = flag.Arg(0)
	}
	if DEBUG {
		go http.ListenAndServe(":3000", nil)
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
		f := util.GetCommandFunc(conn, nil, func() *string { return nil })
		for {
			if v := f(); v == nil {
				break
			} else {
				fmt.Println(v)
			}
		}
	}
	//fmt.Println(conn.Login)
	//fmt.Println(conn.Command)

	var console types.Terminal = &consoleTerminal{
		exit: make(chan bool),
	}
	oldState, err := util.SetupTerminal()
	p(err, "making terminal raw")
	defer util.RestoreTerminal(oldState)
	var changed bool
	if useOwnSSH {
		changed = ssh.Connect(conn, console, func() *string { return nil },
			func(a *types.Connection) {
				saveConn(&conf, conn)
			})
	} else {
		changed = connect(conn, console, func() *string { return nil })
		if changed {
			saveConns(&conf)
		}
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
		signal.Notify(c.signals, os.Interrupt, util.GetSigwinch())
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

func DummyReader(label string, input io.Reader) (io.Reader, error) {
	return input, nil
}

func replaceHome(in string) string {
	if strings.Contains(in, "~") {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = os.Getenv("USERPROFILE")
		}
		if homeDir == "" {
			panic("Error: $HOME not set")
		}
		in = strings.Replace(in, "~", homeDir+"/", -1)
	}
	return in
}

func loadConns() (result types.Configuration) {
	filename := connectionsPath
	rd, err := os.Open(filename)
	p(err, "opening "+filename)
	defer rd.Close()
	rd2, err := charset.NewReaderLabel("utf-16", rd)
	p(err, "loading charset")

	decoder := xml.NewDecoder(rd2)
	decoder.CharsetReader = DummyReader
	p(decoder.Decode(&result), "decoding xml")

	result.AllConnections = listConnections(&result, false)

	result.Root.Expanded = true
	return
}

func saveConn(conf *types.Configuration, conn *types.Connection) {
	filename := connectionsPath
	color.Yellowln("Saving connections.xml...\r")
	flock, err := lock.Try(filename, true)
	if err != nil {
		color.Redln("Error: ", err)
		return
	}
	defer flock.Unlock()
	currentConf := loadConns()
	_, exists := currentConf.AllConnections[conn.Path()]
	if !exists {
		i := 0
		for k := range currentConf.AllConnections {
			if i == 3 {
				break
			}
			fmt.Println("example:", k)
			i++
		}
		fmt.Println("searching:", conn.Path())
		color.Redln("Not saving a connection that was already deleted...\r")
		return
	}
	*currentConf.AllConnections[conn.Path()] = *conn
	saveConns(&currentConf)
	color.Yellowln("done.\r")
}

func saveConns(conf *types.Configuration) {
	filename := connectionsPath
	tmp := filename + ".tmp"
	wr, err := os.Create(tmp)
	p(err, "opening "+filename)
	defer func() {
		if err := os.Rename(tmp, filename); err != nil {
			p(os.Remove(filename), "deleting old connections.xml")
			p(os.Rename(tmp, filename), "overwriting connections.xml")
		}
	}()
	defer wr.Close()

	encoding := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM)
	textEncoder := encoding.NewEncoder()

	writer := textEncoder.Writer(wr)
	fmt.Fprintln(writer, `<?xml version="1.0" encoding="utf-16"?>
<!-- ****************************************************************-->
<!-- *                                                              *-->
<!-- * PuTTY Configuration Manager save file - All right reserved.  *-->
<!-- *                                                              *-->
<!-- ****************************************************************-->
<!-- The following lines can be modified at your own risks.  -->`)

	encoder := xml.NewEncoder(writer)
	encoder.Indent("", "  ")
	p(encoder.Encode(&conf), "encoding xml")
}

func p(err error, where string) {
	if err != nil {
		color.Yellow("When " + where + ": ")
		panic(err)
	}
}
