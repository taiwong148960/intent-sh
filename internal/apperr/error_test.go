package apperr

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExitCodeAndMessage(t *testing.T) {
	tests := []struct {
		kind Kind
		code int
	}{
		{KindInvalidInput, ExitInvalidInput},
		{KindConfiguration, ExitConfiguration},
		{KindProviderUnavailable, ExitProviderUnavailable},
		{KindTimeout, ExitTimeout},
		{KindCancelled, ExitCancelled},
		{KindProviderOutput, ExitProviderOutput},
		{KindSafety, ExitSafety},
		{KindProtocol, ExitProtocol},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			err := Wrap(tt.kind, "test", "safe message", errors.New("SECRET_INTERNAL_CAUSE"))
			if got := ExitCode(err); got != tt.code {
				t.Fatalf("ExitCode() = %d, want %d", got, tt.code)
			}
			if got := Message(err); got != "safe message" {
				t.Fatalf("Message() = %q", got)
			}
		})
	}
}

func TestUnknownErrorIsRedacted(t *testing.T) {
	err := errors.New("SECRET_INTERNAL_CAUSE")
	if got := ExitCode(err); got != ExitInternal {
		t.Fatalf("ExitCode() = %d", got)
	}
	if got := Message(err); got != "intent-sh encountered an internal error" {
		t.Fatalf("Message() = %q", got)
	}
}

func TestSafeMessagesAreTerminalSafeAndBounded(t *testing.T) {
	message := "prefix\x1b[31m\n" + strings.Repeat("秘密", 2000)
	err := New(KindConfiguration, "test", message)
	got := Message(err)
	if strings.ContainsAny(got, "\x1b\n") {
		t.Fatalf("control character leaked in %q", got)
	}
	if len(got) > maxSafeMessageBytes+len("…") || !utf8.ValidString(got) {
		t.Fatalf("message was not safely bounded: bytes=%d valid=%v", len(got), utf8.ValidString(got))
	}
}
