package color

import (
	"fmt"

	ct "github.com/cfstras/pcm/Godeps/_workspace/src/github.com/daviddengcn/go-colortext"
)

func Color(color ct.Color, msg ...interface{}) {
	ct.ChangeColor(color, false, ct.None, false)
	fmt.Print(msg...)
	ct.ResetColor()
}

func Colorln(color ct.Color, msg ...interface{}) {
	ct.ChangeColor(color, false, ct.None, false)
	fmt.Println(msg...)
	ct.ResetColor()
}

func Redln(msg ...interface{}) {
	Colorln(ct.Red, msg...)
}

func Yellow(msg ...interface{}) {
	Color(ct.Yellow, msg...)
}

func Yellowln(msg ...interface{}) {
	Colorln(ct.Yellow, msg...)
}
