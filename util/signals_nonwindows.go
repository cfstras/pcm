// +build !windows

package util

func GetSigwinch() os.Signal {
	return os.SIGWINCH
}
