package textsafe

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTerminalReplacesControlsAndTruncatesValidUTF8(t *testing.T) {
	t.Parallel()
	got := Terminal("safe\x1b[31m\n秘密秘密", 14)
	if strings.ContainsAny(got, "\x1b\n") {
		t.Fatalf("control character remained in %q", got)
	}
	if !utf8.ValidString(got) || !strings.HasSuffix(got, "…") {
		t.Fatalf("invalid bounded result %q", got)
	}
}

func TestTerminalLeavesShortTextAlone(t *testing.T) {
	t.Parallel()
	if got := Terminal("hello", 10); got != "hello" {
		t.Fatalf("Terminal() = %q", got)
	}
}
