// Package textsafe provides bounded terminal-safe text transformations.
package textsafe

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Terminal replaces control characters and truncates at a valid UTF-8 boundary.
// The ellipsis is additional to maxBytes, matching the project's protocol limits.
func Terminal(value string, maxBytes int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	if maxBytes < 0 {
		maxBytes = 0
	}
	if len(value) <= maxBytes {
		return value
	}
	cut := maxBytes
	for cut > 0 && cut < len(value) && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut] + "…"
}
