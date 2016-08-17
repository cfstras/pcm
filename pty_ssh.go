// +build !windows

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/cfstras/pcm/types"
	"github.com/cfstras/pcm/util"
	"github.com/kr/pty"
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

func connect(c *types.Connection, terminal types.Terminal, moreCommands func() *string) bool {

	cmd := &exec.Cmd{}
	cmd.Path = "/usr/bin/ssh"
	cmd.Args = []string{"-v", "-p", fmt.Sprint(c.Info.Port), "-l", c.Login.User, c.Info.Host}
	var procExit int32 = 0

	outFunc := func(pipe *os.File, name string, nextCommand func() *string,
		startWait *sync.Cond) {
		buf := make([]byte, 1024)
		wrotePassword := false
		for {
			if atomic.LoadInt32(&procExit) != 0 {
				return
			}
			n, err := pipe.Read(buf)
			str := string(buf[:n])
			fmt.Fprint(terminal.Stdout(), str)
			if strings.HasSuffix(str, "assword: ") && !wrotePassword && c.Options.LoginMacro {
				pipe.Write([]byte(c.Login.Password))
				pipe.Write([]byte("\n"))
				wrotePassword = true
			} else if strings.HasSuffix(str, "assword: ") || strings.HasSuffix(str, "$ ") ||
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
	nextCommand := util.GetCommandFunc(c, startWait, moreCommands)

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

	p(terminal.Start(), "initializing terminal")
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
