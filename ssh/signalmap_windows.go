// +build windows

package ssh

import (
	"os"
	"syscall"

	"golang.org/x/crypto/ssh"
)

// maps from OS signals to signals sent to SSH
var signalMap map[os.Signal]ssh.Signal = map[os.Signal]ssh.Signal{
	syscall.SIGABRT: ssh.SIGABRT,
	syscall.SIGALRM: ssh.SIGALRM,
	syscall.SIGFPE:  ssh.SIGFPE,
	syscall.SIGHUP:  ssh.SIGHUP,
	syscall.SIGILL:  ssh.SIGILL,
	syscall.SIGINT:  ssh.SIGINT,
	syscall.SIGKILL: ssh.SIGKILL,
	syscall.SIGPIPE: ssh.SIGPIPE,
	syscall.SIGQUIT: ssh.SIGQUIT,
	syscall.SIGSEGV: ssh.SIGSEGV,
	syscall.SIGTERM: ssh.SIGTERM,
	//	syscall.SIGUSR1:  ssh.SIGUSR1,
	//	syscall.SIGUSR2:  ssh.SIGUSR2,
	//	syscall.SIGWINCH: "", // ignore
}
