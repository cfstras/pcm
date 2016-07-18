package ssh

import (
	"bytes"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/cfstras/go-utils/color"
	"github.com/cfstras/pcm/types"
)

var signalMap map[os.Signal]ssh.Signal = map[os.Signal]ssh.Signal{
	syscall.SIGABRT:  ssh.SIGABRT,
	syscall.SIGALRM:  ssh.SIGALRM,
	syscall.SIGFPE:   ssh.SIGFPE,
	syscall.SIGHUP:   ssh.SIGHUP,
	syscall.SIGILL:   ssh.SIGILL,
	syscall.SIGINT:   ssh.SIGINT,
	syscall.SIGKILL:  ssh.SIGKILL,
	syscall.SIGPIPE:  ssh.SIGPIPE,
	syscall.SIGQUIT:  ssh.SIGQUIT,
	syscall.SIGSEGV:  ssh.SIGSEGV,
	syscall.SIGTERM:  ssh.SIGTERM,
	syscall.SIGUSR1:  ssh.SIGUSR1,
	syscall.SIGUSR2:  ssh.SIGUSR2,
	syscall.SIGWINCH: "", // ignore
}

type answerHandler func(answer string)

type instance struct {
	terminal  types.Terminal
	conn      *types.Connection
	questions chan answerHandler

	session *ssh.Session
	changed bool
}

func Connect(conn *types.Connection, terminal types.Terminal, moreCommands func() *string) bool {
	inst := &instance{
		terminal: terminal, conn: conn,
		questions: make(chan answerHandler, 1),
	}
	return inst.connect(moreCommands)
}

func (inst *instance) connect(moreCommands func() *string) bool {
	config := &ssh.ClientConfig{
		User:            inst.conn.Login.User,
		Auth:            []ssh.AuthMethod{ssh.Password(inst.conn.Login.Password)},
		HostKeyCallback: inst.hostKeyCallback,
		Timeout:         20 * time.Second,
	}

	addr := fmt.Sprint(inst.conn.Info.Host, ":", inst.conn.Info.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		color.Redln("Connecting to", addr, ":", err)
		return inst.changed
	}

	// Each ClientConn can support multiple interactive sessions,
	// represented by a Session.
	inst.session, err = client.NewSession()
	if err != nil {
		color.Redln("Failed to create session:", err)
		return inst.changed
	}
	defer inst.session.Close()

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	exit := make(chan bool, 1)
	go func() {
		for e := range inst.terminal.ExitRequests() {
			exit <- e
		}
	}()

	var procExit int32 = 0
	shellOutFunc := func(stdErrOut io.Reader, stdin io.Writer, name string, nextCommand func() *string,
		startWait *sync.Cond) {
		buf := make([]byte, 1024)
		for {
			if atomic.LoadInt32(&procExit) != 0 {
				return
			}
			n, err := stdErrOut.Read(buf)
			if err != nil {
				if err == io.EOF {
					exit <- true
					return
				}
				fmt.Fprintln(inst.terminal.Stderr(), "ssh", name, "error", err)
				exit <- true
				return
			}

			_, err = inst.terminal.Stdout().Write(buf[:n])
			if err != nil {
				if err == io.EOF {
					exit <- true
					return
				}
				fmt.Fprintln(inst.terminal.Stderr(), "ssh", name, "error", err)
				return
			}
			//fmt.Fprint(inst.terminal.Stderr(), str)

			str := string(buf[:n])
			if strings.HasSuffix(str, "assword: ") || strings.HasSuffix(str, "$ ") ||
				strings.HasSuffix(str, "# ") {
				if answer := nextCommand(); answer != nil {
					stdin.Write([]byte(*answer))
					stdin.Write([]byte("\n"))
				}
			}

		}
	}
	inFunc := func(sshStdin io.WriteCloser, startWait *sync.Cond) {
		inputBufChan := make(chan []byte, 32)
		buffers := &sync.Pool{
			New: func() interface{} { return make([]byte, 1024) },
		}

		go func() {
			writeRightNow := make([]byte, 0, 3)
			for {
				buf := buffers.Get().([]byte)
				n, err := inst.terminal.Stdin().Read(buf)
				buf = buf[:n]
				//fmt.Fprintln(inst.terminal.Stdout(), "my stdin got", n, string(buf[:n]))

				if err != nil && err != io.EOF {
					fmt.Fprintln(inst.terminal.Stdout(), "my stdin got error", err)
					inputBufChan <- nil
					startWait.Broadcast()
					return
				}
				writeRightNow = writeRightNow[:0]
				if err == io.EOF {
					writeRightNow = append(writeRightNow, 0x04)
					inputBufChan <- nil
					startWait.Broadcast()
					return
				}
				// ctrl+c, ctrl+d, ctrl+z
				for _, c := range []byte{0x04, 0x03, 0x1a} {
					if bytes.Contains(buf, []byte{c}) {
						writeRightNow = append(writeRightNow, c)
					}
				}

				if newLinePos := bytes.Index(buf, []byte{'\n'}); newLinePos != -1 {
					// see if we have any answer handlers
					select {
					case handler := <-inst.questions:
						answer := string(buf[:newLinePos])
						handler(answer)
						// munch the string
						buf = buf[newLinePos+1:]
						if len(buf) == 0 {
							if cap(buf) > 256 {
								buffers.Put(buf)
							}
							continue
						}
					default:
						// do nothing
					}
				}

				if len(writeRightNow) > 0 {
					_, err = sshStdin.Write(writeRightNow)
					if err != nil {
						fmt.Fprintln(inst.terminal.Stderr(), "stdin got error", err)
						inputBufChan <- nil
						startWait.Broadcast()
						return
					}
				} else {
					inputBufChan <- buf
				}
			}
		}()

		// wait for the start commands to finish
		startWait.L.Lock()
		startWait.Wait()
		startWait.L.Unlock()
		for buf := range inputBufChan {
			if buf == nil {
				fmt.Fprintln(inst.terminal.Stderr(), "closing stdin:", sshStdin.Close())
				return
			}
			_, err := sshStdin.Write(buf)

			if err != nil {
				fmt.Fprintln(inst.terminal.Stderr(), "stdin got error", err)
				fmt.Fprintln(inst.terminal.Stderr(), "closing stdin:", sshStdin.Close())
				return
			}
			if cap(buf) > 256 {
				buffers.Put(buf)
			}
		}
	}

	defer func() {
		atomic.StoreInt32(&procExit, 1)
	}()

	startWait := sync.NewCond(&sync.Mutex{})
	commands := make(chan string, len(inst.conn.Commands.Commands)+1)
	for _, v := range inst.conn.Commands.Commands {
		if v != "" {
			commands <- v
		}
	}

	nextCommand := func() *string {
		select {
		case v := <-commands:
			return &v
		default:
			if c := moreCommands(); c != nil {
				return c
			} else {
				startWait.Broadcast()
				return nil
			}
		}
	}

	sshStdin, err := inst.session.StdinPipe()
	if err != nil {
		color.Redln("Error opening stdin pipe", err)
		return inst.changed
	}
	go inFunc(sshStdin, startWait)

	sshStdout, err := inst.session.StdoutPipe()
	if err != nil {
		color.Redln("Error opening stdout pipe", err)
		return inst.changed
	}
	go shellOutFunc(sshStdout, sshStdin, "stdout", nextCommand, startWait)

	sshStderr, err := inst.session.StderrPipe()
	if err != nil {
		color.Redln("Error opening stderr pipe", err)
		return inst.changed
	}
	go shellOutFunc(sshStderr, sshStdin, "stderr", nextCommand, startWait)

	// Request pseudo terminal
	if err := inst.session.RequestPty("xterm", 80, 40, modes); err != nil {
		color.Redln("request for pseudo terminal failed:", err)
		return inst.changed
	}
	// Start remote shell
	if err := inst.session.Shell(); err != nil {
		color.Redln("failed to start shell:", err)
		return inst.changed
	}
	inst.SendWindowSize()

	signalWatcher := func() {
		for s := range inst.terminal.Signals() {
			sshSignal, ok := signalMap[s]
			if !ok {
				color.Yellowln("Unknown signal", s)
			} else if sshSignal != "" {
				inst.session.Signal(sshSignal)
			}
			if s == syscall.SIGWINCH {
				inst.SendWindowSize()
			} else if s == syscall.SIGTERM {
				exit <- true
			}
		}
	}
	go signalWatcher()

	<-exit
	atomic.StoreInt32(&procExit, 1)
	return inst.changed
}

type winchMsg struct {
	width  uint32
	height uint32
	xpix   uint32
	ypix   uint32
}

func (inst *instance) SendWindowSize() {
	if w, h, err := inst.terminal.GetSize(); err != nil {
		color.Redln("Error getting term size:", err)
	} else {
		msg := ssh.Marshal(&winchMsg{uint32(w), uint32(h), 0, 0})
		_, err = inst.session.SendRequest("window-change", false, msg)
		if err != nil {
			color.Redln("Error sending winch:", err)
		}
	}
}

func (inst *instance) hostKeyCallback(hostname string, remote net.Addr, key ssh.PublicKey) error {
	oldPublicKey, err := hex.DecodeString(inst.conn.Options.SSHPublicKey)
	if err != nil {
		return errors.New("XML is corrupt: " + err.Error())
	}
	newPublicKey := key.Marshal()
	//TODO correctly marshal/unmarshal into xml

	newPublicMD5 := md5.Sum(newPublicKey)
	newPublicString := hex.EncodeToString(newPublicMD5[:])

	if len(oldPublicKey) == 0 {
		color.Yellowln("Registering new SSH Public Key", key.Type(),
			newPublicString)
		inst.conn.Options.SSHPublicKey = hex.EncodeToString(newPublicKey)
		inst.changed = true
		return nil
	}

	oldPublicMD5 := md5.Sum(oldPublicKey)
	oldPublicString := hex.EncodeToString(oldPublicMD5[:])

	same := subtle.ConstantTimeCompare(newPublicKey, oldPublicKey)
	if same == 1 {
		return nil
	}
	color.Redln("-----POSSIBLE ATTACK-----\nSSH key changed! expected (md5):",
		oldPublicString,
		"got:", newPublicString, "type:", key.Type())
	inst.terminal.Stderr().Write([]byte("Accept change [Ny]? "))

	buf := make([]byte, 128)
	n, err := inst.terminal.Stdin().Read(buf)
	if err != nil {
		color.Yellowln("Error reading answer:", err)
		return err
	}
	inst.terminal.Stderr().Write([]byte{'\n'})

	text := strings.ToLower(string(buf[:n]))
	if text == "y" || text == "yes" {
		inst.conn.Options.SSHPublicKey = hex.EncodeToString(newPublicKey)
		inst.changed = true
		color.Yellowln("Saving new public key to connections.xml on exit.")
		return nil
	}
	return errors.New("Public key not accepted")
}
