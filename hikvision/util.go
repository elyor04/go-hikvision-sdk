package hikvision

/*
#include "shim.h"
*/
import "C"

import "unsafe"

// cString reads a fixed-size C char array embedded in a shim.h POD struct
// into a Go string. Safe because shim.cpp always zero-initializes these
// structs before filling them in, so the array is guaranteed NUL-terminated
// within maxLen.
func cString(p *C.char, maxLen int) string {
	base := unsafe.Slice((*byte)(unsafe.Pointer(p)), maxLen)
	n := maxLen
	for i, c := range base {
		if c == 0 {
			n = i
			break
		}
	}
	return string(base[:n])
}
