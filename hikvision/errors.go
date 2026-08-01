package hikvision

/*
#include "shim.h"
*/
import "C"

import "fmt"

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
