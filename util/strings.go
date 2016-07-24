package util

import (
	"fmt"
	"strings"
)

func DebugString(buf []byte) string {
	s := string(buf)
	s = strings.Replace(s, "\n", `\n`, -1)
	s = strings.Replace(s, "\r", `\r`, -1)
	return fmt.Sprint("(", len(buf), ") ", s)
}
