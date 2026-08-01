// Package hikvision is a cross-platform Go wrapper around Hikvision's native
// HCNetSDK (Windows/Linux, amd64). See shim.h for why a C++ shim sits between
// this package and the vendor header.
package hikvision

/*
#cgo windows CPPFLAGS: -I${SRCDIR}/../internal/sdklib/windows_amd64/include
#cgo windows LDFLAGS: -L${SRCDIR}/../internal/sdklib/windows_amd64/lib -lHCNetSDK -lstdc++
#cgo linux CPPFLAGS: -I${SRCDIR}/../internal/sdklib/linux_amd64/include
#cgo linux LDFLAGS: -L${SRCDIR}/../internal/sdklib/linux_amd64/lib -lhcnetsdk -lstdc++ -Wl,-rpath,${SRCDIR}/../internal/sdklib/linux_amd64/lib
#include <stdlib.h>
#include "shim.h"
*/
import "C"
