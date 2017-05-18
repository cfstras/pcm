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
	"golang.org/x/crypto/ssh/agent"

	"github.com/cfstras/go-utils/color"
	"github.com/cfstras/pcm/types"
	"github.com/cfstras/pcm/util"
)

type answerHandler func(answer string)

type instance struct {
	terminal types.Terminal
	conn     *types.Connection
	// Answer handlers: push a handler to questions to capture the next line of
	// input
	questions   chan answerHandler
	saveChanges func(*types.Connection)

	session *ssh.Session
	changed bool
}

func Connect(conn *types.Connection, terminal types.Terminal,
	moreCommands func() *string, saveChanges func(*types.Connection)) bool {

	inst := &instance{
		terminal: terminal, conn: conn,
		questions:   make(chan answerHandler, 1),
		saveChanges: saveChanges,
	}
	return inst.connect(moreCommands)
}

type detectingSigner struct {
	inner   ssh.Signer
	didSign bool
}

func (s *detectingSigner) PublicKey() ssh.PublicKey {
	return s.inner.PublicKey()
}

func (s *detectingSigner) Filename() string {
	asString := fmt.Sprintf(" %s", s.PublicKey())
	split := strings.SplitN(asString, " ", 4)
	if len(split) == 4 {
		return split[3]
	} else {
		return asString[:10] + "..."
	}
}
func (s *detectingSigner) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	s.didSign = true
	color.Greenln("Key used:", s.Filename(), "\r")
	return s.inner.Sign(rand, data)
}

func openAgentSocket(sock string) (ssh.AuthMethod, *[]*detectingSigner) {
	var wrappedSigners *[]*detectingSigner
	wrappedSigners = &[]*detectingSigner{}
	if sshAgent, err := net.Dial("unix", sock); err == nil {
		return ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			signers, err := agent.NewClient(sshAgent).Signers()
			if len(signers) == 0 {
				color.Redln("Warning: no SSH keys in agent.")
			} else {
				keys := ""
				for i, s := range signers {
					if keys != "" {
						keys += "; "
					}
					ws := &detectingSigner{s, false}
					keys += ws.Filename()
					*wrappedSigners = append(*wrappedSigners, ws)
					signers[i] = ws // replace in interface list
				}
				color.Yellowln("SSH keys registered:", keys, "\r")
			}
			return signers, err
		}), wrappedSigners
	}
	color.Yellowln("Warning: no SSH-agent found.")
	return nil, nil
}

func (inst *instance) connect(moreCommands func() *string) bool {
	config := &ssh.ClientConfig{
		User:            inst.conn.Login.User,
		Auth:            []ssh.AuthMethod{ssh.Password(inst.conn.Login.Password)},
		HostKeyCallback: inst.hostKeyCallback,
		Timeout:         20 * time.Second,
	}
	var detectingSigners *[]*detectingSigner
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock != "" {
		agentAuth, signers := openAgentSocket(sock)
		if agentAuth != nil && signers != nil {
			config.Auth = append(config.Auth, agentAuth)
			detectingSigners = signers
		}
	} else {
		color.Redln("Warning: no SSH agent running.\r")
	}

	addr := fmt.Sprint(inst.conn.Info.Host, ":", inst.conn.Info.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		color.Redln("Connecting to", addr, ":", err, "\r")
		return inst.changed
	}
	//if sock != "" { // agent forwarding
	//	agent.ForwardToRemote(client, sock)
	//}

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

	var procExit int32
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
					//fmt.Println("\r\n--- Got", str[len(str)-4:],
					//	"- writing next cmd", *answer, "---\r")
					//TODO wait for this to be sent
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
			var lastChars [3]byte
			for {
				buf := buffers.Get().([]byte)
				n, err := inst.terminal.Stdin().Read(buf)
				buf = buf[:n]
				//fmt.Fprintln(inst.terminal.Stdout(), "my stdin got", util.DebugString(buf))
				if err != nil && err != io.EOF {
					fmt.Fprintln(inst.terminal.Stdout(), "my stdin got error", err)
					inputBufChan <- nil
					startWait.Broadcast()
					return
				}
				writeRightNow = writeRightNow[:0]
				if err == io.EOF {
					writeRightNow = append(writeRightNow, 'D'&0x1f)
					inputBufChan <- nil
					startWait.Broadcast()
					return
				}
				// ctrl+c, ctrl+d, ctrl+z
				for _, c := range []byte{'C' & 0x1f, 'D' & 0x1f, 'Z' & 0x1f} {
					if bytes.Contains(buf, []byte{c}) {
						writeRightNow = append(writeRightNow, c)
					}
				}

				for i := len(buf) - 3; i < len(buf); i++ {
					if i < 0 {
						continue
					}
					lastChars[0], lastChars[1], lastChars[2] = lastChars[1],
						lastChars[2], buf[i]
				}
				if lastChars == [3]byte{'\r', '~', '.'} ||
					lastChars == [3]byte{'C' & 0x1f, '~', '.'} ||
					lastChars == [3]byte{'Z' & 0x1f, '~', '.'} {

					color.Redln("Got ~C... aborting.")
					inputBufChan <- nil
					startWait.Broadcast()
					return
				}

				// handle questions
				if newLinePos := bytes.Index(buf, []byte{'\r'}); newLinePos != -1 {
					// see if we have any answer handlers
					select {
					case handler := <-inst.questions:
						answer := string(buf[:newLinePos])
						handler(answer)
						// munch the string
						buf = buf[newLinePos+1:]
						if len(buf) == 0 {
							if cap(buf) > 256 {
								buffers.Put(buf[:cap(buf)])
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
				exit <- true
				return
			}
			trans := util.TransformInput(buf)
			//fmt.Fprintln(inst.terminal.Stderr(), "sending", util.DebugString(trans))
			_, err := sshStdin.Write(trans)

			if err != nil {
				fmt.Fprintln(inst.terminal.Stderr(), "stdin got error", err)
				fmt.Fprintln(inst.terminal.Stderr(), "closing stdin:", sshStdin.Close())
				exit <- true
				return
			}
			if cap(buf) > 256 {
				buffers.Put(buf[:cap(buf)])
			}
		}
	}

	defer func() {
		atomic.StoreInt32(&procExit, 1)
	}()

	startWait := sync.NewCond(&sync.Mutex{})
	nextCommand := func(c *types.Connection, startWait *sync.Cond, moreCommands util.CommandFunc) util.CommandFunc {
		passthrough := util.GetCommandFunc(inst.conn, startWait, moreCommands)
		checkedAuths := false
		return func() *string {
			if !checkedAuths && detectingSigners != nil {
				checkedAuths = true
				//color.Yellowln("checking whether we used ssh key...\r")
				for _, a := range *detectingSigners {
					if a != nil && a.didSign {
						//color.Yellowln("we did! -- using password as first 'command'\r")
						return &inst.conn.Login.Password
					} /* else {
						color.Yellowln("nope:", a, "\r")
					}*/
				}
			} /*else {
				color.Yellowln("detectingSigners:", detectingSigners, "checkedAuths:", checkedAuths, "\r")
			}*/
			return passthrough()
		}
	}(inst.conn, startWait, moreCommands)

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
			if s == util.GetSigwinch() {
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
			newPublicString, "\r")
		inst.conn.Options.SSHPublicKey = hex.EncodeToString(newPublicKey)
		inst.saveChanges(inst.conn)
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
		"got:", newPublicString, "type:", key.Type(), "\r")
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
