package hikvision

import "testing"

func TestCString(t *testing.T) {
	got := testCString("hello", 16)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestCStringEmpty(t *testing.T) {
	got := testCString("", 8)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestCStringFillsEntireBuffer(t *testing.T) {
	got := testCStringRaw([]byte("AAAA"))
	if got != "AAAA" {
		t.Errorf("got %q, want %q (no NUL terminator within buffer)", got, "AAAA")
	}
}

func TestCStringTruncatesToBufferSize(t *testing.T) {
	// testCString reserves the last byte for NUL, matching cSetString's
	// truncation behavior on the write side.
	got := testCString("hello world", 6)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestCSetStringFits(t *testing.T) {
	got, fit := testCSetString("hello", 16)
	if !fit || got != "hello" {
		t.Errorf("got %q, fit=%v, want %q, fit=true", got, fit, "hello")
	}
}

func TestCSetStringExactFit(t *testing.T) {
	// "hello" is 5 bytes; +1 for the NUL terminator is exactly bufLen.
	got, fit := testCSetString("hello", 6)
	if !fit || got != "hello" {
		t.Errorf("got %q, fit=%v, want %q, fit=true", got, fit, "hello")
	}
}

func TestCSetStringTooLongReportsNoFit(t *testing.T) {
	// cSetString must report fit=false rather than silently truncating -
	// Login relies on this to reject an over-length credential instead of
	// authenticating with a different, wrong (truncated) one.
	got, fit := testCSetString("hello world", 6)
	if fit {
		t.Errorf("fit = true, want false (%q + NUL doesn't fit in %d bytes)", "hello world", 6)
	}
	if got != "" {
		t.Errorf("got %q, want empty (buffer must be left untouched on no-fit)", got)
	}
}
