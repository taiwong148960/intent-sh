// Package keychord parses the bounded, shell-independent key grammar used by
// adapter bindings and terminal diagnostics.
package keychord

import (
	"fmt"
	"strings"
)

const MaxTextBytes = 6

// Modifier identifies one of the two supported chord modifiers.
type Modifier uint8

const (
	modifierInvalid Modifier = iota
	ModifierAlt
	ModifierCtrl
)

// Chord is a validated key chord. Its fields are deliberately private so all
// instances originate in Parse and remain safe to render through the methods
// below.
type Chord struct {
	modifier Modifier
	key      byte
}

// Sequence is a terminal byte sequence with a fixed two-byte upper bound.
type Sequence struct {
	data   [2]byte
	length uint8
}

// Parse canonicalizes one alt+<non-whitespace-printable-ascii> or
// ctrl+<ascii-letter> chord.
func Parse(value string) (Chord, error) {
	if len(value) == 0 {
		return Chord{}, fmt.Errorf("key chord must not be empty")
	}
	if len(value) > MaxTextBytes {
		return Chord{}, fmt.Errorf("key chord must be at most %d ASCII bytes", MaxTextBytes)
	}
	for i := 0; i < len(value); i++ {
		if value[i] <= 0x20 || value[i] > 0x7e {
			return Chord{}, fmt.Errorf("key chord must contain only non-whitespace printable ASCII")
		}
	}

	separator := strings.IndexByte(value, '+')
	if separator < 0 || separator == len(value)-1 {
		return Chord{}, fmt.Errorf("key chord must use alt+<key> or ctrl+<letter>")
	}
	modifierText := strings.ToLower(value[:separator])
	keyText := value[separator+1:]
	if len(keyText) != 1 {
		return Chord{}, fmt.Errorf("key chord must contain exactly one key")
	}
	key := asciiLower(keyText[0])

	switch modifierText {
	case "alt":
		return Chord{modifier: ModifierAlt, key: key}, nil
	case "ctrl":
		if key < 'a' || key > 'z' {
			return Chord{}, fmt.Errorf("ctrl key must be one ASCII letter")
		}
		if reason := reservedControlReason(key); reason != "" {
			return Chord{}, fmt.Errorf("ctrl+%c is reserved for %s", key, reason)
		}
		return Chord{modifier: ModifierCtrl, key: key}, nil
	default:
		return Chord{}, fmt.Errorf("modifier must be alt or ctrl")
	}
}

func asciiLower(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func asciiUpper(value byte) byte {
	if value >= 'a' && value <= 'z' {
		return value - ('a' - 'A')
	}
	return value
}

func reservedControlReason(key byte) string {
	switch key {
	case 'c':
		return "cancellation and terminal interrupt"
	case 'd':
		return "terminal EOF"
	case 'j', 'm':
		return "Enter"
	case 'q', 's':
		return "terminal flow control"
	case 'y', 'z':
		return "terminal suspension"
	default:
		return ""
	}
}

// Modifier reports the validated modifier.
func (c Chord) Modifier() Modifier { return c.modifier }

// Key reports the canonical lowercase ASCII key.
func (c Chord) Key() byte { return c.key }

// Canonical returns the stable configuration form.
func (c Chord) Canonical() string {
	prefix := "alt+"
	if c.modifier == ModifierCtrl {
		prefix = "ctrl+"
	}
	return prefix + string(c.key)
}

// Display returns the bounded name used in user-facing diagnostics.
func (c Chord) Display() string {
	prefix := "Alt+"
	if c.modifier == ModifierCtrl {
		prefix = "Ctrl+"
	}
	return prefix + string(asciiUpper(c.key))
}

// TerminalSequence derives the exact bytes expected from a conventional PTY.
func (c Chord) TerminalSequence() Sequence {
	if c.modifier == ModifierCtrl {
		return Sequence{data: [2]byte{c.key & 0x1f}, length: 1}
	}
	return Sequence{data: [2]byte{0x1b, c.key}, length: 2}
}

// Bytes returns a new slice containing at most two bytes.
func (s Sequence) Bytes() []byte {
	result := make([]byte, int(s.length))
	copy(result, s.data[:s.length])
	return result
}

// Len returns the byte count, which is always one or two for a parsed chord.
func (s Sequence) Len() int { return int(s.length) }

// ZLEBinding returns a shell-safe Zsh ANSI-C literal composed solely of fixed
// hexadecimal byte escapes.
func (c Chord) ZLEBinding() string {
	return hexEscapes(c.TerminalSequence())
}

// ReadlineBinding returns a Readline quoted-key body composed solely of fixed
// hexadecimal byte escapes. The caller places it inside a fixed bind template.
func (c Chord) ReadlineBinding() string {
	return hexEscapes(c.TerminalSequence())
}

func hexEscapes(sequence Sequence) string {
	var builder strings.Builder
	builder.Grow(sequence.Len() * 4)
	for _, value := range sequence.data[:sequence.length] {
		fmt.Fprintf(&builder, "\\x%02x", value)
	}
	return builder.String()
}
