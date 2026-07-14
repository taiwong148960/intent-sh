package keychord

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseAndDerive(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input     string
		canonical string
		display   string
		bytes     []byte
		zle       string
		readline  string
	}{
		{input: "alt+g", canonical: "alt+g", display: "Alt+G", bytes: []byte{0x1b, 'g'}, zle: `\x1b\x67`, readline: `\x1b\x67`},
		{input: "ALT+U", canonical: "alt+u", display: "Alt+U", bytes: []byte{0x1b, 'u'}, zle: `\x1b\x75`, readline: `\x1b\x75`},
		{input: "alt++", canonical: "alt++", display: "Alt++", bytes: []byte{0x1b, '+'}, zle: `\x1b\x2b`, readline: `\x1b\x2b`},
		{input: "alt+'", canonical: "alt+'", display: "Alt+'", bytes: []byte{0x1b, '\''}, zle: `\x1b\x27`, readline: `\x1b\x27`},
		{input: "ctrl+G", canonical: "ctrl+g", display: "Ctrl+G", bytes: []byte{0x07}, zle: `\x07`, readline: `\x07`},
		{input: "ctrl+x", canonical: "ctrl+x", display: "Ctrl+X", bytes: []byte{0x18}, zle: `\x18`, readline: `\x18`},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			chord, err := Parse(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := chord.Canonical(); got != test.canonical {
				t.Fatalf("Canonical() = %q, want %q", got, test.canonical)
			}
			if got := chord.Display(); got != test.display {
				t.Fatalf("Display() = %q, want %q", got, test.display)
			}
			sequence := chord.TerminalSequence()
			if got := sequence.Bytes(); !bytes.Equal(got, test.bytes) || sequence.Len() != len(test.bytes) {
				t.Fatalf("TerminalSequence() = %v/%d, want %v", got, sequence.Len(), test.bytes)
			}
			if got := chord.ZLEBinding(); got != test.zle {
				t.Fatalf("ZLEBinding() = %q, want %q", got, test.zle)
			}
			if got := chord.ReadlineBinding(); got != test.readline {
				t.Fatalf("ReadlineBinding() = %q, want %q", got, test.readline)
			}
		})
	}
}

func TestParseRejectsMalformedReservedAndNonASCII(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"":           "must not be empty",
		"g":          "must use",
		"meta+g":     "modifier",
		"shift+g":    "at most",
		"alt+":       "must use",
		"alt+gg":     "exactly one",
		"alt+ g":     "printable ASCII",
		"alt+ ":      "printable ASCII",
		"alt+\n":     "printable ASCII",
		"alt+é":      "printable ASCII",
		"ctrl+1":     "ASCII letter",
		"ctrl+é":     "at most",
		"ctrl+enter": "ASCII bytes",
		"ctrl+c":     "cancellation",
		"ctrl+d":     "EOF",
		"ctrl+j":     "Enter",
		"ctrl+m":     "Enter",
		"ctrl+q":     "flow control",
		"ctrl+s":     "flow control",
		"ctrl+y":     "suspension",
		"ctrl+z":     "suspension",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(input)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("Parse(%q) error = %v, want substring %q", input, err, want)
			}
		})
	}
}

func TestShellBindingsContainNoUntrustedSyntax(t *testing.T) {
	t.Parallel()
	for key := byte(0x21); key <= 0x7e; key++ {
		chord, err := Parse("alt+" + string(key))
		if err != nil {
			t.Fatalf("Parse(%q): %v", "alt+"+string(key), err)
		}
		for _, binding := range []string{chord.ZLEBinding(), chord.ReadlineBinding()} {
			if len(binding) > 8 || strings.ContainsAny(binding, "'\"`;:$(){}[]\n\r") {
				t.Fatalf("unsafe binding for %q: %q", chord.Canonical(), binding)
			}
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"alt+g", "ALT+'", "ctrl+x", "ctrl+c", "alt+é", "$(touch /tmp/nope)"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		chord, err := Parse(input)
		if err != nil {
			return
		}
		if len(chord.Canonical()) > MaxTextBytes || len(chord.Display()) > MaxTextBytes || chord.TerminalSequence().Len() > 2 {
			t.Fatalf("parsed result exceeded bounds: %#v", chord)
		}
		parsedAgain, err := Parse(chord.Canonical())
		if err != nil || parsedAgain != chord {
			t.Fatalf("canonical value did not round trip: %#v, %v", parsedAgain, err)
		}
	})
}
