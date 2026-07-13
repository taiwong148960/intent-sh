// Package smoketest contains opt-in checks that may contact real provider services.
package smoketest

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	contextinfo "github.com/taiwong148960/intent-sh/internal/context"
	"github.com/taiwong148960/intent-sh/internal/prompt"
	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/internal/provider"
	"github.com/taiwong148960/intent-sh/internal/safety"
	"github.com/taiwong148960/intent-sh/internal/textsafe"
)

// TestRealProviderSmoke is skipped unless INTENT_SH_REAL_PROVIDER_SMOKE names
// one or both providers (for example, "codex" or "claude,codex"). It never
// logs prompts, commands, model output, environment values, or credentials.
func TestRealProviderSmoke(t *testing.T) {
	selected := selectedProviders(os.Getenv("INTENT_SH_REAL_PROVIDER_SMOKE"))
	if len(selected) == 0 {
		t.Skip("set INTENT_SH_REAL_PROVIDER_SMOKE=claude,codex to contact real providers")
	}

	for _, name := range selected {
		name := name
		t.Run(name, func(t *testing.T) {
			adapter := realProvider(t, name)
			disposable := t.TempDir()
			t.Chdir(disposable)
			marker := filepath.Join(disposable, "target-command-ran")

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			probe, err := adapter.Probe(ctx)
			if err != nil {
				t.Fatalf("%s compatibility/login probe failed: %v", name, err)
			}
			t.Logf("%s compatible CLI version: %s", name, textsafe.Terminal(probe.Version, 120))

			promptText, err := prompt.Build(prompt.Input{
				Buffer: "Print the current working directory with the single harmless read-only command pwd.",
				Cursor: len("Print the current working directory with the single harmless read-only command pwd."),
				Environment: contextinfo.Environment{
					OS: runtime.GOOS, Arch: runtime.GOARCH, Shell: safety.ShellZsh,
					ShellVersion: "5.9", CWD: disposable,
				},
			})
			if err != nil {
				t.Fatal("build bounded smoke prompt")
			}
			value, err := adapter.Generate(ctx, provider.Request{Prompt: promptText, Timeout: 90 * time.Second})
			if err != nil {
				t.Fatalf("%s harmless generation failed: %v", name, err)
			}
			if value.Status != protocol.ProviderStatusOK {
				t.Fatalf("%s returned a non-command smoke result", name)
			}
			decision, err := (safety.Engine{}).Evaluate(ctx, value.Command, safety.ShellZsh, value.RiskHint)
			if err != nil {
				t.Fatalf("%s smoke command failed local validation: %v", name, err)
			}
			if decision.Level != safety.LevelSafe {
				t.Fatalf("%s smoke command was not locally read-only", name)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("%s caused target-command side effects: %v", name, err)
			}
		})
	}
}

func selectedProviders(value string) []string {
	seen := map[string]bool{}
	var selected []string
	for _, part := range strings.Split(value, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if (name == provider.NameClaude || name == provider.NameCodex) && !seen[name] {
			seen[name] = true
			selected = append(selected, name)
		}
	}
	return selected
}

func realProvider(t *testing.T, name string) provider.Provider {
	t.Helper()
	switch name {
	case provider.NameClaude:
		return provider.Claude{}
	case provider.NameCodex:
		return provider.Codex{}
	default:
		t.Fatalf("unsupported real smoke provider %q", name)
		return nil
	}
}
