package shellassets

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestConfiguredAdaptersExposeBindingsAndRejectDifferentReevaluation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "zsh", args: []string{"-f", "-i", "-c"}},
		{name: "bash", args: []string{"--noprofile", "--norc", "-i", "-c"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binary := configuredAdapterTestShell(t, test.name)
			first, err := ScriptWithBindings(test.name, ProtocolVersion, "ctrl+x", "alt+'")
			if err != nil {
				t.Fatal(err)
			}
			second, err := ScriptWithBindings(test.name, ProtocolVersion, "alt+g", "alt+u")
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			firstPath := filepath.Join(dir, "first")
			secondPath := filepath.Join(dir, "second")
			if err := os.WriteFile(firstPath, []byte(first), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(secondPath, []byte(second), 0o600); err != nil {
				t.Fatal(err)
			}
			command := `source "$1" || exit; printf 'MARKERS|%s|%s|\n' "$INTENT_SH_ADAPTER_REWRITE_KEY" "$INTENT_SH_ADAPTER_UNDO_KEY"; source "$1" || exit; source "$2"`
			args := append(append([]string(nil), test.args...), command, "intent-sh-binding-test", firstPath, secondPath)
			output, runErr := exec.Command(binary, args...).CombinedOutput()
			if runErr == nil {
				t.Fatalf("different binding re-evaluation unexpectedly succeeded:\n%s", output)
			}
			for _, want := range []string{"MARKERS|ctrl+x|alt+'|", "different rewrite or undo bindings are already active"} {
				if !strings.Contains(string(output), want) {
					t.Fatalf("output omitted %q:\n%s", want, output)
				}
			}
		})
	}
}

func configuredAdapterTestShell(t *testing.T, name string) string {
	t.Helper()
	if name != "bash" {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is not installed", name)
		}
		return path
	}
	for _, candidate := range []string{os.Getenv("INTENT_SH_TEST_BASH"), "bash", "/opt/homebrew/bin/bash", "/usr/local/bin/bash"} {
		if candidate == "" {
			continue
		}
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		if exec.Command(path, "-c", "(( BASH_VERSINFO[0] >= 4 ))").Run() == nil {
			return path
		}
	}
	t.Skip("configured native Readline test requires Bash 4.0+")
	return ""
}

func TestConfiguredScriptsRenderOnlyDerivedBindings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		shell string
		want  []string
	}{
		{shell: "zsh", want: []string{`typeset __intent_sh_requested_rewrite_binding=$'\x18'`, `typeset __intent_sh_requested_undo_binding=$'\x1b\x27'`}},
		{shell: "bash", want: []string{`__intent_sh_requested_rewrite_binding='\x18'`, `__intent_sh_requested_undo_binding='\x1b\x27'`}},
	}
	for _, test := range tests {
		t.Run(test.shell, func(t *testing.T) {
			script, err := ScriptWithBindings(test.shell, ProtocolVersion, "CTRL+X", "alt+'")
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !strings.Contains(script, want) {
					t.Fatalf("%s script omitted %q", test.shell, want)
				}
			}
			for _, prohibited := range []string{"__INTENT_SH_REWRITE_CANONICAL__", "__INTENT_SH_UNDO_CANONICAL__", "__INTENT_SH_REWRITE_BINDING__", "__INTENT_SH_UNDO_BINDING__", "CTRL+X", "alt+'"} {
				if strings.Contains(script, prohibited) {
					t.Fatalf("%s script retained unrendered or executable input %q", test.shell, prohibited)
				}
			}
			if !strings.Contains(script, `$'\x63\x74\x72\x6c\x2b\x78'`) || !strings.Contains(script, `$'\x61\x6c\x74\x2b\x27'`) {
				t.Fatalf("%s script omitted canonical bounded session markers", test.shell)
			}
		})
	}
}

func TestConfiguredScriptsRejectInvalidBindingsBeforeRendering(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		rewrite string
		undo    string
		want    string
	}{
		{rewrite: "ctrl+c", undo: "alt+u", want: "rewrite_key"},
		{rewrite: "alt+g", undo: "alt+gg", want: "undo_key"},
		{rewrite: "ALT+G", undo: "alt+g", want: "distinct"},
	} {
		script, err := ScriptWithBindings("zsh", ProtocolVersion, test.rewrite, test.undo)
		if err == nil || script != "" || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("ScriptWithBindings(%q, %q) = %q, %v", test.rewrite, test.undo, script, err)
		}
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
