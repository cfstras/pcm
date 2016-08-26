package main

import (
	"io"
	"os"
	"os/exec"

	"github.com/kr/pty"

	"github.com/cfstras/go-utils/color"
	"github.com/cfstras/pcm/xterm"

	"net/http"
	_ "net/http/pprof"
)

var BashPaths = []string{
	"/bin/bash",
	"C:/cygwin64/bin/bash.exe",
	"C:/cygwin/bin/bash.exe",
}

func main() {
	go http.ListenAndServe(":3000", nil)

	// find bash
	var bashPath string
	for _, p := range BashPaths {
		if st, err := os.Stat(p); err == nil {
			if st.IsDir() {
				color.Redln(st.Name(), " is a directory")
				return
			}
			bashPath = p
		}
	}
	if bashPath == "" {
		color.Redln("could not find cygwin bash.")
		return
	}

	console := xterm.Terminal()
	defer console.Close()
	cmd := exec.Cmd{
		Path: bashPath,
		Args: []string{"-i", "--noediting", "--norc"}, //, "-c", "echo", "hai"},
	}

	pty, err := pty.Start(&cmd)

	//stdinPipe, err := cmd.StdinPipe()
	//stdoutPipe, err2 := cmd.StdoutPipe()
	//stderrPipe, err3 := cmd.StderrPipe()
	//if err != nil || err2 != nil || err3 != nil {
	//	color.Redln("opening bash pipes", err, err2, err3)
	//	return
	//}

	if err != nil {
		color.Redln("starting bash:", err)
		return
	}

	go forward("stdout", pty, console.Stdout())
	//go forward("stderr", stderrPipe, console.Stderr())
	go forward("stdin", console.Stdin(), pty)

	err = console.Start()
	if err != nil {
		color.Redln("starting xterm:", err)
		return
	}

	go func() {
		for _ = range console.ExitRequests() {
			cmd.Process.Kill()
		}
	}()

	cmd.Wait()
}

func forward(name string, from io.Reader, to io.Writer) {
	buf := make([]byte, 128)
	for {
		n, err := from.Read(buf)
		//color.Yellowln(name, err, string(buf[:n]))
		n, err2 := to.Write(buf[:n])
		if err != nil || err2 != nil {
			color.Redln("Error in forward", name, err, err2)
			return
		}
	}
}
