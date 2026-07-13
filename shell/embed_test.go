package shellassets

import (
	"strings"
	"testing"

	"github.com/taiwong148960/intent-sh/internal/apperr"
)

func TestEmbeddedScriptsMatchProtocol(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"bash", "zsh"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			script, err := Script(name, ProtocolVersion)
			if err != nil {
				t.Fatalf("Script() error = %v", err)
			}
			if !strings.Contains(script, "__intent_sh_protocol_version="+ProtocolVersion) {
				t.Fatalf("%s script omitted protocol marker", name)
			}
		})
	}
}

func TestEmbeddedScriptsRejectMismatchAndUnknownShell(t *testing.T) {
	t.Parallel()
	if _, err := Script("zsh", "999"); apperr.KindOf(err) != apperr.KindProtocol {
		t.Fatalf("mismatch kind = %q", apperr.KindOf(err))
	}
	if _, err := Script("fish", ProtocolVersion); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("unknown shell kind = %q", apperr.KindOf(err))
	}
}
