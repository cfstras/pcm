package main

/*
#cgo CFLAGS: -fPIC
#cgo CPPFLAGS: -fPIC
#cgo LDFLAGS: -L. -lv8 -lv8wrapper -lstdc++ -pthread

#include <stdlib.h>
#include "v8wrapper.h"

*/
import "C"

import (
	"fmt"
	"unsafe"
)

func main() {
	fmt.Println("Hello, insanity!")
	fmt.Println(RunV8(`['Oh', 'my!'].join(', ')`))
}

func RunV8(script string) string {

	// convert Go string to nul terminated C-string
	cstr := C.CString(script)
	defer C.free(unsafe.Pointer(cstr))

	// run script and convert returned C-string to Go string
	rcstr := C.runv8(cstr)
	defer C.free(unsafe.Pointer(rcstr))

	return C.GoString(rcstr)
}
