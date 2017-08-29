package hterm

import (
	"log"
	"net/http"

	"time"

	"fmt"
	"math/rand"

	"io"
	"os"

	"strconv"

	"github.com/cfstras/go-utils/math"
	"github.com/cfstras/pcm/types"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/websocket"
	"github.com/skratchdot/open-golang/open"
)

func Run() {
	fs := http.FileServer(
		&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: nil, Prefix: ""})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/socket/" {
			wsHandler(w, r)
			return
		}
		fs.ServeHTTP(w, r)
	})

	port := rand.Intn(65535-1024) + 1024
	addr := fmt.Sprint("127.0.0.1:", port)

	go startWeb(addr)
	time.Sleep(1 * time.Second)
	open.Start("http://" + addr + "/index.html")

}

func startWeb(addr string) {
	log.Fatal(http.ListenAndServe(addr, nil))
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
var connQueue = make(chan *websocket.Conn)

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("conn from", r.Host)
	connQueue <- conn
}

type WSTerminal struct {
	conn          *websocket.Conn
	width, height int
	exit          chan bool

	readQueue chan []byte
	readRet   chan bool
}

func GetConsoleConn() types.Terminal {
	conn := <-connQueue
	//TODO wait for initial size message
	term := &WSTerminal{conn, 80, 30,
		make(chan bool),
		make(chan []byte), make(chan bool)}
	go func() {
		for {
			if term.conn == nil {
				term.conn = <-connQueue
				//TODO select and timeout
			}
			mtype, b, err := term.conn.ReadMessage()
			if err != nil {
				log.Println("reading: ", err, "waiting for reconnect...")
				term.conn = nil
				time.Sleep(1 * time.Second)
				continue
			}
			if mtype == websocket.TextMessage {
				//TODO json
				log.Println("text message", string(b))
				continue
			}
			term.readQueue <- b
			// block until it has been consumed completely
			<-term.readRet
		}
	}()
	return term
}

func (t *WSTerminal) GetSize() (width, height int, err error) {
	return t.width, t.height, nil
}
func (t *WSTerminal) Stdin() io.Reader {
	return t
}
func (t *WSTerminal) Stdout() io.Writer {
	return t
}
func (t *WSTerminal) Stderr() io.Writer {
	return t
}
func (t *WSTerminal) Write(b []byte) (int, error) {
	w, err := t.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return 0, err
	}
	if _, err := w.Write([]byte{'D'}); err != nil {
		return 0, err
	}
	if _, err := w.Write(b); err != nil {
		return 0, err
	}
	w.Close()
	log.Println("term write", strconv.QuoteToASCII(string(b)))
	return len(b), nil
}
func (t *WSTerminal) Read(p []byte) (n int, err error) {
	b := <-t.readQueue
	left := 0
	if len(b) > len(p) {
		copy(p, b[:len(p)])
		left = len(b) - len(p)
		t.readQueue <- b[len(p):]
	} else {
		copy(p, b)
		t.readRet <- true
	}
	length := math.MinI(len(b), len(p))
	log.Println("term read", strconv.QuoteToASCII(string(p[:length])), ";", left, "left")
	return length, nil
}
func (t *WSTerminal) ExitRequests() <-chan bool {
	return t.exit
}
func (t *WSTerminal) Signals() <-chan os.Signal {
	return nil
	//TODO
}
func (t *WSTerminal) MakeRaw() {
	//TODO
}
func (t *WSTerminal) RestoreRaw() {
	//TODO
}
