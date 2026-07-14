// Package shellassets embeds adapters that are version-matched to the binary.
package shellassets

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/taiwong148960/intent-sh/internal/apperr"
)

const ProtocolVersion = "2"

//go:embed bash/intent-sh.bash
var bashScript string

//go:embed zsh/intent-sh.zsh
var zshScript string

func Script(shell, binaryProtocol string) (string, error) {
	if binaryProtocol != ProtocolVersion {
		return "", apperr.New(apperr.KindProtocol, "initialize adapter", fmt.Sprintf("embedded adapter protocol %s is incompatible with the binary protocol", ProtocolVersion))
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
	return script, nil
}
