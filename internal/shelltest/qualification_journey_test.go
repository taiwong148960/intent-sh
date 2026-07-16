package shelltest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taiwong148960/intent-sh/internal/config"
)

func TestNativeSetupCustomProbeResetDowngradeAndRemovalJourney(t *testing.T) {
	root := repositoryRoot(t)
	binDir := buildMVPTools(t, root)
	intentBinary := filepath.Join(binDir, "intent-sh")
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	configPath := filepath.Join(xdg, "intent-sh", "config.toml")
	startupPath := filepath.Join(home, ".zshrc")
	environment := map[string]string{
		"HOME": home, "XDG_CONFIG_HOME": xdg, "SHELL": "/bin/zsh",
		"PATH": binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	}

	show := runQualificationCommand(t, intentBinary, environment, "config", "show")
	for _, want := range []string{`rewrite_key = 'alt+g'`, `undo_key = 'alt+u'`} {
		if !strings.Contains(show, want) {
			t.Fatalf("default config output omitted %q: %s", want, show)
		}
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config show created a file: %v", err)
	}
	setup := runQualificationCommand(t, intentBinary, environment, "setup", "zsh")
	for _, want := range []string{`eval "$(intent-sh init zsh)"`, "Alt+G: rewrite", "Alt+U: restore", "doctor --keys", "No startup file was modified"} {
		if !strings.Contains(setup, want) {
			t.Fatalf("setup journey omitted %q: %s", want, setup)
		}
	}
	if _, err := os.Stat(startupPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("setup created a startup file: %v", err)
	}

	for _, setting := range [][2]string{{"provider", "codex"}, {"rewrite_key", "ctrl+x"}, {"undo_key", "ctrl+r"}} {
		runQualificationCommand(t, intentBinary, environment, "config", "set", setting[0], setting[1])
	}
	if _, err := os.Stat(startupPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config set changed the startup file: %v", err)
	}

	shellCase := shellCase{name: "zsh", executable: "zsh", args: []string{"-f", "-i"}, script: filepath.Join(root, "shell", "zsh", "intent-sh.zsh")}
	requireCompatibleShell(t, shellCase)
	shell := startShellWithPTYOptions(t, shellCase, environment, `eval "$(intent-sh init zsh)"`, terminalPTYOptions{
		term: "xterm-256color", rows: 32, cols: 110, respondTerminalQueries: true,
	})
	configureStateDump(t, shell)

	shell.write(t, "intent-sh doctor --keys")
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntilTimeout(t, "press Ctrl+X now", 20*time.Second)
	shell.writeBytes(t, []byte{0x18})
	shell.readUntilTimeout(t, "press Ctrl+R now", 10*time.Second)
	shell.writeBytes(t, []byte{0x12})
	shell.readUntilTimeout(t, "press Enter now", 10*time.Second)
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntilTimeout(t, "press Ctrl+C now", 10*time.Second)
	shell.writeBytes(t, []byte{0x03})
	for _, want := range []string{
		"PASS terminal.keys.tty", "PASS terminal.keys.rewrite", "PASS terminal.keys.undo",
		"PASS terminal.keys.enter", "PASS terminal.keys.cancel", "PASS terminal.keys.restore",
		"READY intent-sh can serve rewrites",
	} {
		shell.readUntilTimeout(t, want, 20*time.Second)
	}
	shell.readUntilTimeout(t, promptMarker, 10*time.Second)
	if _, err := os.Stat(filepath.Join(home, "codex-invoked")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("key probe invoked the provider: %v", err)
	}

	original := "QUALIFY-CUSTOM-INTENT_CASE_SAFE_7Q"
	shell.write(t, original)
	shell.writeBytes(t, []byte{0x18})
	shell.readUntilTimeout(t, "generated one", 30*time.Second)
	assertShellState(t, shell, "printf GEN_ONE", len("printf GEN_ONE"), original, 0, "safe")
	shell.writeBytes(t, []byte{0x12})
	shell.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
	assertShellState(t, shell, original, len(original), "", 0, "")
	clearEditableLine(t, shell)
	shell.close(t)

	for _, setting := range [][2]string{{"rewrite_key", "alt+g"}, {"undo_key", "alt+u"}} {
		runQualificationCommand(t, intentBinary, environment, "config", "set", setting[0], setting[1])
	}
	shell = startShellWithPTYOptions(t, shellCase, environment, `eval "$(intent-sh init zsh)"`, terminalPTYOptions{
		term: "xterm-256color", rows: 32, cols: 110, respondTerminalQueries: true,
	})
	configureStateDump(t, shell)
	original = "QUALIFY-DEFAULT-INTENT_CASE_SAFE_7Q"
	shell.write(t, original)
	shell.writeBytes(t, []byte{0x1b, 'g'})
	shell.readUntilTimeout(t, "generated one", 30*time.Second)
	shell.writeBytes(t, []byte{0x1b, 'u'})
	shell.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
	assertShellState(t, shell, original, len(original), "", 0, "")
	clearEditableLine(t, shell)
	shell.close(t)

	// Simulate the documented cleanup before installing a strict older binary:
	// remove only the two new keys and prove current defaults fill them back in.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(line, "rewrite_key ") || strings.HasPrefix(line, "undo_key ") {
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(configPath, []byte(strings.Join(kept, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RewriteKey != "alt+g" || cfg.UndoKey != "alt+u" {
		t.Fatalf("downgrade cleanup did not restore defaults: %#v", cfg)
	}

	unrelated := "export QUALIFICATION_USER_LINE=kept"
	activation := `eval "$(intent-sh init zsh)"`
	if err := os.WriteFile(startupPath, []byte(unrelated+"\n"+activation+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	startup, err := os.ReadFile(startupPath)
	if err != nil {
		t.Fatal(err)
	}
	remaining := strings.Replace(string(startup), activation+"\n", "", 1)
	if err := os.WriteFile(startupPath, []byte(remaining), 0o600); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(remaining) != unrelated {
		t.Fatalf("removal changed an unrelated startup line: %q", remaining)
	}
	if err := os.Remove(intentBinary); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(configPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(intentBinary); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("binary removal failed: %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config removal failed: %v", err)
	}
}

func runQualificationCommand(t *testing.T, binary string, environment map[string]string, args ...string) string {
	t.Helper()
	command := exec.Command(binary, args...)
	if coverageDirectory, err := qualificationExecutableCoverageDirectory(); err != nil {
		t.Fatal(err)
	} else if coverageDirectory != "" {
		environment = cloneEnvironmentMap(environment)
		environment["GOCOVERDIR"] = coverageDirectory
	}
	command.Env = replaceEnvironment(os.Environ(), environment)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run qualification command %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func cloneEnvironmentMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source)+1)
	for key, value := range source {
		result[key] = value
	}
	return result
}
