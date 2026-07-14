package shelltest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taiwong148960/intent-sh/internal/config"
	"github.com/taiwong148960/intent-sh/internal/keychord"
	"github.com/taiwong148960/intent-sh/internal/provider"
)

type terminalConformanceCase struct {
	shell        shellCase
	rewriteValue string
	undoValue    string
	rewriteBytes []byte
	undoBytes    []byte
	term         string
	enter        []byte
	rows         uint16
	cols         uint16
}

func newTerminalConformanceCase(t *testing.T, shell shellCase, rewriteValue, undoValue, termName string, enter []byte, rows, cols uint16) terminalConformanceCase {
	t.Helper()
	rewrite, err := keychord.Parse(rewriteValue)
	if err != nil {
		t.Fatal(err)
	}
	undo, err := keychord.Parse(undoValue)
	if err != nil {
		t.Fatal(err)
	}
	return terminalConformanceCase{
		shell: shell, rewriteValue: rewrite.Canonical(), undoValue: undo.Canonical(),
		rewriteBytes: rewrite.TerminalSequence().Bytes(), undoBytes: undo.TerminalSequence().Bytes(),
		term: termName, enter: append([]byte(nil), enter...), rows: rows, cols: cols,
	}
}

func nativeConformanceShells(root string) []shellCase {
	return []shellCase{
		{name: "zsh", executable: "zsh", args: []string{"-f", "-i"}, script: filepath.Join(root, "shell", "zsh", "intent-sh.zsh")},
		{name: "bash", executable: testBash(), args: []string{"--noprofile", "--norc", "-i"}, script: filepath.Join(root, "shell", "bash", "intent-sh.bash")},
	}
}

func startTerminalConformanceShell(t *testing.T, matrix terminalConformanceCase, binDir, providerMode string, priority []string) (*runningShell, string, string) {
	t.Helper()
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	cfg := config.Defaults()
	cfg.Provider = providerMode
	cfg.Priority = append([]string(nil), priority...)
	cfg.TimeoutSeconds = 5
	cfg.RewriteKey = matrix.rewriteValue
	cfg.UndoKey = matrix.undoValue
	configPath := filepath.Join(xdg, "intent-sh", "config.toml")
	if err := config.WriteAt(configPath, cfg); err != nil {
		t.Fatalf("write terminal conformance config: %v", err)
	}
	env := map[string]string{
		"HOME": home, "XDG_CONFIG_HOME": xdg, "SHELL": matrix.shell.executable,
		"PATH": binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	}
	shell := startShellWithPTYOptions(t, matrix.shell, env, `eval "$(intent-sh init `+matrix.shell.name+`)"`, terminalPTYOptions{
		term: matrix.term, rows: matrix.rows, cols: matrix.cols, respondTerminalQueries: true,
	})
	configureStateDump(t, shell)
	return shell, home, configPath
}

func TestNativeTerminalConformanceLifecycleMatrix(t *testing.T) {
	root := repositoryRoot(t)
	binDir := buildMVPTools(t, root)
	for _, shell := range nativeConformanceShells(root) {
		for _, chordCase := range []struct {
			name          string
			rewrite, undo string
			term          string
			enter         []byte
			rows, cols    uint16
		}{
			{name: "default-alt-cr", rewrite: "alt+g", undo: "alt+u", term: "dumb", enter: []byte{'\r'}, rows: 24, cols: 80},
			{name: "custom-ctrl-lf", rewrite: "ctrl+x", undo: "ctrl+b", term: "xterm-256color", enter: []byte{'\n'}, rows: 40, cols: 132},
		} {
			t.Run(shell.name+"/"+chordCase.name, func(t *testing.T) {
				requireCompatibleShell(t, shell)
				matrix := newTerminalConformanceCase(t, shell, chordCase.rewrite, chordCase.undo, chordCase.term, chordCase.enter, chordCase.rows, chordCase.cols)
				runNativeConformanceLifecycle(t, matrix, binDir)
			})
		}
	}
}

func runNativeConformanceLifecycle(t *testing.T, matrix terminalConformanceCase, binDir string) {
	t.Helper()
	shell, home, _ := startTerminalConformanceShell(t, matrix, binDir, config.ProviderAuto, []string{provider.NameClaude, provider.NameCodex})
	defer shell.close(t)
	runConformanceLifecycleOnShell(t, shell, home, matrix)
}

// runConformanceLifecycleOnShell is shared by direct-PTY and isolated tmux
// qualification. It asserts editor state through a test-only shell widget and
// marker files; it never reads terminal screen contents or tmux panes.
func runConformanceLifecycleOnShell(t *testing.T, shell *runningShell, home string, matrix terminalConformanceCase) {
	t.Helper()
	original := "prefix-INTENT_CASE_CLAUDE_SAFE_7Q"
	shell.write(t, original)
	shell.writeBytes(t, matrix.rewriteBytes)
	shell.readUntilTimeout(t, "Claude generated one", 30*time.Second)
	assertShellState(t, shell, "printf CLAUDE_ONE", len("printf CLAUDE_ONE"), original, 0, "safe")
	shell.writeBytes(t, matrix.rewriteBytes)
	shell.readUntilTimeout(t, "Claude generated two", 30*time.Second)
	assertShellState(t, shell, "printf CLAUDE_TWO", len("printf CLAUDE_TWO"), original, 1, "safe")
	shell.writeBytes(t, matrix.undoBytes)
	shell.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
	assertShellState(t, shell, original, len(original), "", 0, "")
	clearEditableLine(t, shell)

	shell.write(t, "INTENT_CASE_CLAUDE_SAFE_7Q")
	shell.writeBytes(t, matrix.rewriteBytes)
	shell.readUntilTimeout(t, "Claude generated one", 30*time.Second)
	shell.write(t, "X")
	shell.writeBytes(t, matrix.undoBytes)
	shell.readUntilTimeout(t, "buffer was edited; undo did not overwrite it", 10*time.Second)
	assertShellState(t, shell, "printf CLAUDE_ONEX", len("printf CLAUDE_ONEX"), "", 0, "")
	clearEditableLine(t, shell)

	clarify := "INTENT_CASE_CLAUDE_CLARIFY_7Q"
	shell.write(t, clarify)
	shell.writeBytes(t, matrix.rewriteBytes)
	shell.readUntilTimeout(t, "Which Claude target should be used?", 30*time.Second)
	assertShellState(t, shell, clarify, len(clarify), "", 0, "")
	clearEditableLine(t, shell)

	cancelled := "INTENT_CASE_CLAUDE_SLOW_7Q"
	shell.write(t, cancelled)
	shell.writeBytes(t, matrix.rewriteBytes)
	shell.readUntilTimeout(t, "Ctrl+C to cancel", 10*time.Second)
	shell.writeBytes(t, []byte{0x03})
	shell.readUntilTimeout(t, "cancelled", 10*time.Second)
	assertShellState(t, shell, cancelled, len(cancelled), "", 0, "")
	if _, err := os.Stat(filepath.Join(home, "codex-invoked")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancellation incorrectly invoked fallback provider: %v", err)
	}
	clearEditableLine(t, shell)

	fallback := "INTENT_CASE_FALLBACK_7Q"
	shell.write(t, fallback)
	shell.writeBytes(t, matrix.rewriteBytes)
	shell.readUntilTimeout(t, "fallback via Codex", 30*time.Second)
	assertShellState(t, shell, "printf CODEX_FALLBACK", len("printf CODEX_FALLBACK"), fallback, 0, "safe")
	if _, err := os.Stat(filepath.Join(home, "claude-invoked")); err != nil {
		t.Fatalf("fallback did not invoke Claude first: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "codex-invoked")); err != nil {
		t.Fatalf("fallback did not invoke Codex second: %v", err)
	}
	shell.writeBytes(t, matrix.undoBytes)
	shell.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
	clearEditableLine(t, shell)

	shell.write(t, "intent-sh config set provider codex >/dev/null")
	shell.writeBytes(t, matrix.enter)
	shell.readUntilTimeout(t, promptMarker, 10*time.Second)

	reviewMarker := filepath.Join(home, "review-ran")
	shell.write(t, "INTENT_CASE_REVIEW_7Q")
	shell.writeBytes(t, matrix.rewriteBytes)
	shell.readUntilTimeout(t, "REVIEW:", 30*time.Second)
	if _, err := os.Stat(reviewMarker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("review command executed automatically: %v", err)
	}
	shell.writeBytes(t, matrix.enter)
	waitForPathTimeout(t, reviewMarker, 10*time.Second, shell)
	shell.readUntilTimeout(t, promptMarker, 10*time.Second)

	dangerMarker := filepath.Join(home, "danger-ran")
	shell.write(t, "INTENT_CASE_DANGER_7Q")
	shell.writeBytes(t, matrix.rewriteBytes)
	shell.readUntilTimeout(t, "DANGEROUS:", 30*time.Second)
	if _, err := os.Stat(dangerMarker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dangerous command executed automatically: %v", err)
	}
	shell.writeBytes(t, matrix.enter)
	shell.readUntilTimeout(t, "Press Enter again to execute.", 10*time.Second)
	if _, err := os.Stat(dangerMarker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("first Enter executed dangerous command: %v", err)
	}
	shell.writeBytes(t, matrix.enter)
	waitForPathTimeout(t, dangerMarker, 10*time.Second, shell)
}

func TestTERMResizeAndUnicodeFailureConformance(t *testing.T) {
	root := repositoryRoot(t)
	binDir := buildMVPTools(t, root)
	for _, shellCase := range nativeConformanceShells(root) {
		for _, termName := range []string{"dumb", "xterm-256color", "screen-256color"} {
			t.Run(shellCase.name+"/"+termName, func(t *testing.T) {
				requireCompatibleShell(t, shellCase)
				matrix := newTerminalConformanceCase(t, shellCase, "alt+g", "alt+u", termName, []byte{'\r'}, 28, 96)
				shell, _, _ := startTerminalConformanceShell(t, matrix, binDir, config.ProviderCodex, []string{provider.NameCodex})
				defer shell.close(t)

				nativeCursor := installUnicodeCursorWidget(t, shell)
				original := "前e\u0301後INTENT_CASE_INVALID_7Q"
				shell.write(t, original)
				shell.writeBytes(t, []byte{0x1b, 'c'})
				shell.writeBytes(t, matrix.rewriteBytes)
				shell.readUntilTimeout(t, "Codex CLI returned an invalid structured result", 30*time.Second)
				assertShellState(t, shell, original, nativeCursor, "", 0, "")
				clearEditableLine(t, shell)

				resizeOriginal := "INTENT_CASE_RESIZE_7Q"
				shell.write(t, resizeOriginal)
				shell.writeBytes(t, matrix.rewriteBytes)
				shell.readUntilTimeout(t, "Ctrl+C to cancel", 10*time.Second)
				shell.resize(t, 45, 144)
				shell.readUntilTimeout(t, "resize result", 30*time.Second)
				assertShellState(t, shell, "printf RESIZED", len("printf RESIZED"), resizeOriginal, 0, "safe")
				shell.writeBytes(t, matrix.undoBytes)
				shell.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
				assertShellState(t, shell, resizeOriginal, len(resizeOriginal), "", 0, "")
			})
		}
	}
}

func TestBindingMismatchAndConcurrentSessionsKeepBufferStateLocal(t *testing.T) {
	root := repositoryRoot(t)
	binDir := buildMVPTools(t, root)
	for _, shellCase := range nativeConformanceShells(root) {
		t.Run(shellCase.name+"/binding-mismatch", func(t *testing.T) {
			requireCompatibleShell(t, shellCase)
			matrix := newTerminalConformanceCase(t, shellCase, "ctrl+x", "ctrl+b", "xterm-256color", []byte{'\r'}, 30, 100)
			shell, _, configPath := startTerminalConformanceShell(t, matrix, binDir, config.ProviderCodex, []string{provider.NameCodex})
			defer shell.close(t)
			installReinitializeWidget(t, shell)
			cfg := config.Defaults()
			cfg.Provider = config.ProviderCodex
			if err := config.WriteAt(configPath, cfg); err != nil {
				t.Fatal(err)
			}
			shell.write(t, "KEEP")
			shell.writeBytes(t, []byte{0x1b, 'r'})
			shell.readUntilTimeout(t, "different rewrite or undo bindings are already active", 10*time.Second)
			assertShellState(t, shell, "KEEP", 4, "", 0, "")
		})

		t.Run(shellCase.name+"/concurrent-sessions", func(t *testing.T) {
			requireCompatibleShell(t, shellCase)
			matrix := newTerminalConformanceCase(t, shellCase, "alt+g", "alt+u", "screen-256color", []byte{'\r'}, 30, 100)
			first, firstHome, _ := startTerminalConformanceShell(t, matrix, binDir, config.ProviderCodex, []string{provider.NameCodex})
			defer first.close(t)
			second, secondHome, _ := startTerminalConformanceShell(t, matrix, binDir, config.ProviderCodex, []string{provider.NameCodex})
			defer second.close(t)

			first.write(t, "FIRST-INTENT_CASE_SAFE_7Q")
			first.writeBytes(t, matrix.rewriteBytes)
			first.readUntilTimeout(t, "generated one", 30*time.Second)
			second.write(t, "SECOND-INTENT_CASE_SAFE_7Q")
			second.writeBytes(t, matrix.rewriteBytes)
			second.readUntilTimeout(t, "generated one", 30*time.Second)
			assertShellState(t, first, "printf GEN_ONE", len("printf GEN_ONE"), "FIRST-INTENT_CASE_SAFE_7Q", 0, "safe")
			assertShellState(t, second, "printf GEN_ONE", len("printf GEN_ONE"), "SECOND-INTENT_CASE_SAFE_7Q", 0, "safe")
			clearEditableLine(t, first)
			clearEditableLine(t, second)

			first.write(t, "INTENT_CASE_DANGER_7Q")
			first.writeBytes(t, matrix.rewriteBytes)
			first.readUntilTimeout(t, "DANGEROUS:", 30*time.Second)
			second.write(t, "INTENT_CASE_DANGER_7Q")
			second.writeBytes(t, matrix.rewriteBytes)
			second.readUntilTimeout(t, "DANGEROUS:", 30*time.Second)
			first.writeBytes(t, matrix.enter)
			first.readUntilTimeout(t, "Press Enter again to execute.", 10*time.Second)
			second.writeBytes(t, matrix.enter)
			second.readUntilTimeout(t, "Press Enter again to execute.", 10*time.Second)
			firstMarker := filepath.Join(firstHome, "danger-ran")
			secondMarker := filepath.Join(secondHome, "danger-ran")
			if _, err := os.Stat(firstMarker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("first session executed on first Enter: %v", err)
			}
			if _, err := os.Stat(secondMarker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("second session executed on first Enter: %v", err)
			}
			first.writeBytes(t, matrix.enter)
			waitForPathTimeout(t, firstMarker, 10*time.Second, first)
			if _, err := os.Stat(secondMarker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("first session acceptance changed second session: %v", err)
			}
			second.writeBytes(t, matrix.enter)
			waitForPathTimeout(t, secondMarker, 10*time.Second, second)
		})
	}
}

func clearEditableLine(t *testing.T, shell *runningShell) {
	t.Helper()
	shell.writeBytes(t, []byte{0x01, 0x0b})
}

func installUnicodeCursorWidget(t *testing.T, shell *runningShell) int {
	t.Helper()
	command := `function __intent_sh_test_cursor() { CURSOR=3; }; zle -N intent-sh-test-cursor __intent_sh_test_cursor; bindkey '^[c' intent-sh-test-cursor`
	cursor := 3
	if shell.name == "bash" {
		command = `__intent_sh_test_cursor(){ READLINE_POINT=6; }; bind -x '"\ec":__intent_sh_test_cursor'`
		cursor = 6
	}
	shell.write(t, command)
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntilTimeout(t, promptMarker, 10*time.Second)
	return cursor
}

func installReinitializeWidget(t *testing.T, shell *runningShell) {
	t.Helper()
	command := `function __intent_test_reinit() { eval "$(intent-sh init zsh)"; }; zle -N intent-test-reinit __intent_test_reinit; bindkey '^[r' intent-test-reinit`
	if shell.name == "bash" {
		command = `__intent_test_reinit(){ eval "$(intent-sh init bash)"; }; bind -x '"\er":__intent_test_reinit'`
	}
	shell.write(t, command)
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntilTimeout(t, promptMarker, 10*time.Second)
}

func assertNoRawTerminalIdentity(prompt string) bool {
	for _, marker := range []string{"TERM", "TERM_PROGRAM", "WT_SESSION", "TMUX"} {
		if strings.Contains(prompt, marker) {
			return false
		}
	}
	return true
}
