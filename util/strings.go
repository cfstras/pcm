package util

import (
	"fmt"
	"io"
	"strings"
)

func DebugString(buf []byte) string {
	s := string(buf)
	s = strings.Replace(s, "\n", `\n`, -1)
	s = strings.Replace(s, "\r", `\r`, -1)
	return fmt.Sprint("(", len(buf), ") ", s)
}

type ReplaceWriter struct {
	Writer   io.Writer
	Replacer *strings.Replacer
}

func (w ReplaceWriter) Write(p []byte) (n int, err error) {
	_, err = w.Writer.Write([]byte(w.Replacer.Replace(string(p))))
	return len(p), err
}
