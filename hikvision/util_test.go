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
