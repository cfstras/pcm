package lock

import (
	"syscall"

	"github.com/pkg/errors"
)

func (f *File) internalFlock(how int) (*File, error) {
	err := syscall.Flock(int(f.fd), how)
	if err != nil && err.(syscall.Errno) == syscall.EWOULDBLOCK {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "Calling flock()")
	}
	f.locked = true
	return f, nil
}

func (f *File) internalTryLock() (*File, error) {
	return f.internalFlock(syscall.LOCK_EX | syscall.LOCK_NB)
}

func (f *File) internalLock() (*File, error) {
	return f.internalFlock(syscall.LOCK_EX)
}

func (f *File) internalUnlock() error {
	return syscall.Flock(int(f.fd), syscall.LOCK_UN)
}
