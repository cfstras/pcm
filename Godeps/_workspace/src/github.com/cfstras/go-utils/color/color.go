package color

import (
	"fmt"

	ct "github.com/daviddengcn/go-colortext"
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

func Greenln(msg ...interface{}) {
	Colorln(ct.Green, msg...)
}

func Yellow(msg ...interface{}) {
	Color(ct.Yellow, msg...)
}

func Yellowln(msg ...interface{}) {
	Colorln(ct.Yellow, msg...)
}
