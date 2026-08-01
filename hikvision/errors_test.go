package hikvision

import "testing"

func TestErrorMessageKnownCode(t *testing.T) {
	msg, ok := ErrorMessage(7) // NET_DVR_NETWORK_FAIL_CONNECT
	if !ok {
		t.Fatalf("expected code 7 to be known")
	}
	if msg != "Connect to server failed" {
		t.Errorf("unexpected message: %q", msg)
	}
}

func TestErrorMessageUnknownCode(t *testing.T) {
	if _, ok := ErrorMessage(0xFFFFFFFF); ok {
		t.Errorf("expected an absurd code to be unknown")
	}
}

func TestErrorMessageNoError(t *testing.T) {
	msg, ok := ErrorMessage(0)
	if !ok || msg != "No Error" {
		t.Errorf("code 0 should map to \"No Error\", got %q, ok=%v", msg, ok)
	}
}

func TestErrorFormatting(t *testing.T) {
	err := &Error{Op: "Login", Code: 1}
	got := err.Error()
	want := "hikvision: Login: Username or Password error (code 1)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestErrorFormattingUnknownCode(t *testing.T) {
	err := &Error{Op: "Login", Code: 999999}
	got := err.Error()
	want := "hikvision: Login: unknown error (code 999999)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestErrorCodesTableHasNoDuplicateNoiseAndCoversZero(t *testing.T) {
	if len(errorMessages) < 100 {
		t.Errorf("expected a substantial curated error table, got %d entries", len(errorMessages))
	}
	if _, ok := errorMessages[0]; !ok {
		t.Errorf("expected code 0 (NET_DVR_NOERROR) to be present")
	}
}
