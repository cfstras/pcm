package lock

import (
	"os"

	"github.com/pkg/errors"
)

var (
	ErrorNotLocked = errors.New("File is not locked")
)

type File struct {
	locked bool
	fd     uintptr
}

func Try(path string, blocking bool) (*File, error) {
	f := File{}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		file, err = os.Create(path)
		if err != nil {
			return nil, errors.Wrap(err, "Could not create file")
		}
	} else if err != nil {
		return nil, err
	}
	f.fd = file.Fd()
	if blocking {
		return f.internalLock()
	}
	return f.internalTryLock()
}

func (f *File) Unlock() error {
	if !f.locked {
		return ErrorNotLocked
	}
	if err := f.internalUnlock(); err != nil {
		return err
	}
	f.locked = false
	return nil
}
