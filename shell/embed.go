// Package shellassets embeds adapters that are version-matched to the binary.
package shellassets

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/keychord"
)

const ProtocolVersion = "2"

//go:embed bash/intent-sh.bash
var bashScript string

//go:embed zsh/intent-sh.zsh
var zshScript string

func Script(shell, binaryProtocol string) (string, error) {
	return ScriptWithBindings(shell, binaryProtocol, "alt+g", "alt+u")
}

// ScriptWithBindings validates and renders bounded binding values into fixed
// adapter placeholders. No configuration text is evaluated as shell source.
func ScriptWithBindings(shell, binaryProtocol, rewriteValue, undoValue string) (string, error) {
	if binaryProtocol != ProtocolVersion {
		return "", apperr.New(apperr.KindProtocol, "initialize adapter", fmt.Sprintf("embedded adapter protocol %s is incompatible with the binary protocol", ProtocolVersion))
	}
	rewrite, err := keychord.Parse(rewriteValue)
	if err != nil {
		return "", apperr.New(apperr.KindConfiguration, "initialize adapter", "rewrite_key is invalid: "+err.Error())
	}
	undo, err := keychord.Parse(undoValue)
	if err != nil {
		return "", apperr.New(apperr.KindConfiguration, "initialize adapter", "undo_key is invalid: "+err.Error())
	}
	if rewrite == undo {
		return "", apperr.New(apperr.KindConfiguration, "initialize adapter", "rewrite_key and undo_key must be distinct")
	}
	var script string
	switch shell {
	case "bash":
		script = bashScript
	case "zsh":
		script = zshScript
	default:
		return "", apperr.New(apperr.KindInvalidInput, "initialize adapter", "supported shells are bash and zsh")
	}
	marker := "__intent_sh_protocol_version=" + ProtocolVersion
	if !strings.Contains(script, marker) {
		return "", apperr.New(apperr.KindProtocol, "initialize adapter", "embedded shell adapter protocol marker is missing")
	}
	replacements := map[string]string{
		"__INTENT_SH_REWRITE_CANONICAL__": shellASCIILiteral(rewrite.Canonical()),
		"__INTENT_SH_UNDO_CANONICAL__":    shellASCIILiteral(undo.Canonical()),
		"__INTENT_SH_REWRITE_BINDING__":   rewrite.ZLEBinding(),
		"__INTENT_SH_UNDO_BINDING__":      undo.ZLEBinding(),
	}
	for placeholder, replacement := range replacements {
		if !strings.Contains(script, placeholder) {
			return "", apperr.New(apperr.KindProtocol, "initialize adapter", "embedded shell adapter binding placeholder is missing")
		}
		script = strings.ReplaceAll(script, placeholder, replacement)
	}
	return script, nil
}

func shellASCIILiteral(value string) string {
	var builder strings.Builder
	builder.Grow(3 + len(value)*4)
	builder.WriteString("$'")
	for index := 0; index < len(value); index++ {
		fmt.Fprintf(&builder, "\\x%02x", value[index])
	}
	builder.WriteByte('\'')
	return builder.String()
}
