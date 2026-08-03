package hikvision

/*
#include "shim.h"
*/
import "C"

import (
	"fmt"
	"runtime"
)

// Error wraps a failure from the underlying HCNetSDK. Code is the value
// NET_DVR_GetLastError() returned immediately after the failing call - per
// HCNetSDK's own documentation this must be read synchronously right after
// the call that failed, since it reflects the last operation's outcome
// rather than being tied to a specific call.
type Error struct {
	// Op names the package-level operation that failed, e.g. "Login", "RealPlay".
	Op string
	// Code is the raw NET_DVR error code (0 means the SDK didn't report one).
	Code uint32
}

func (e *Error) Error() string {
	if msg, ok := errorMessages[e.Code]; ok {
		return fmt.Sprintf("hikvision: %s: %s (code %d)", e.Op, msg, e.Code)
	}
	return fmt.Sprintf("hikvision: %s: unknown error (code %d)", e.Op, e.Code)
}

// ErrorMessage returns the known human-readable message for a raw NET_DVR
// error code, or false if this package doesn't have a mapping for it (the
// full HCNetSDK error space is large; only the common status/network/login
// codes are curated here - see errors_codes_gen.go).
func ErrorMessage(code uint32) (string, bool) {
	msg, ok := errorMessages[code]
	return msg, ok
}

// lastError builds an *Error for a failing operation named op, using
// NET_DVR_GetLastError() for the code.
func lastError(op string) error {
	return &Error{Op: op, Code: uint32(C.hik_get_last_error())}
}

// HCNetSDK documents NET_DVR_GetLastError() as returning the *calling
// thread's* last error, not a process-global one - callers are expected to
// invoke it immediately after the failing call, on the same thread. Go does
// not otherwise guarantee that two consecutive cgo calls made by the same
// goroutine (the SDK call itself, and the immediately following
// NET_DVR_GetLastError() inside lastError) run on the same OS thread -
// nothing prevents the goroutine from being rescheduled onto a different M
// between them. Without pinning, a concurrent SDK call from another
// goroutine (landing on the thread Go happens to reschedule us onto) or a
// genuine migration can substitute the wrong error code: a spurious
// success/failure or a misattributed error, invisible to `go test -race`
// since no Go memory is raced on - it's entirely inside the C library's own
// thread-local state. sdkCall0/sdkCallHandle exist so every call site that
// makes an SDK call and then conditionally calls lastError does both under
// the same locked OS thread; use them (or bracket manually with
// runtime.LockOSThread/UnlockOSThread) rather than calling lastError bare
// after an arbitrary gap.

// sdkCall0 runs fn - an HCNetSDK wrapper using the "0 on success" convention
// - and, on failure, captures its error under the same locked OS thread as
// the call itself so the error genuinely belongs to fn. See the comment
// above lastError.
func sdkCall0(op string, fn func() C.int32_t) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if fn() != 0 {
		return lastError(op)
	}
	return nil
}

// sdkCallHandle runs fn - an HCNetSDK wrapper returning a handle/ID
// (negative on failure) - and, on failure, captures its error under the
// same locked OS thread as the call itself. See the comment above
// lastError.
func sdkCallHandle(op string, fn func() C.int32_t) (int32, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	h := int32(fn())
	if h < 0 {
		return h, lastError(op)
	}
	return h, nil
}
