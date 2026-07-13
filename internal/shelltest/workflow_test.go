package shelltest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taiwong148960/intent-sh/internal/config"
)

const fakeCodexScript = `#!/bin/sh
result=
while [ "$#" -gt 0 ]; do
    case "$1" in
        --output-last-message)
            result=$2
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done
printf 'invoked\n' >> "$HOME/codex-invoked"
prompt=$(cat)
case "$prompt" in
    *INTENT_CASE_SLOW_7Q*)
        sleep 10
        printf '%s' '{"status":"ok","command":"printf SLOW","explanation":"slow result","assumptions":[],"riskHint":"safe"}' > "$result"
        ;;
    *INTENT_CASE_INVALID_7Q*)
        printf '%s' 'not-json' > "$result"
        ;;
    *INTENT_CASE_CLARIFY_7Q*)
        printf '%s' '{"status":"clarify","question":"Which directory should be searched?"}' > "$result"
        ;;
    *INTENT_CASE_REVIEW_7Q*)
        printf '%s%s%s' '{"status":"ok","command":"touch ' "$HOME/review-ran" '","explanation":"review result","assumptions":[],"riskHint":"review"}' > "$result"
        ;;
    *INTENT_CASE_DANGER_7Q*)
        printf '%s%s%s' '{"status":"ok","command":"touch ' "$HOME/danger-ran" '","explanation":"danger result","assumptions":[],"riskHint":"dangerous"}' > "$result"
        ;;
    *INTENT_CASE_SAFE_7Q*)
        case "$prompt" in
            *'"original":"prefix-INTENT_CASE_SAFE_7Q"'*'"previous":"printf GEN_ONE"'*'"generationIndex":1'*)
                printf '%s' '{"status":"ok","command":"printf GEN_TWO","explanation":"generated two","assumptions":[],"riskHint":"safe"}' > "$result"
                ;;
            *'"generationIndex":1'*)
                printf '%s' '{"status":"ok","command":"printf BAD_REGEN","explanation":"regeneration lost its original","assumptions":[],"riskHint":"safe"}' > "$result"
                ;;
            *)
                printf '%s' '{"status":"ok","command":"printf GEN_ONE","explanation":"generated one","assumptions":[],"riskHint":"safe"}' > "$result"
                ;;
        esac
        ;;
    *INTENT_CASE_FALLBACK_7Q*)
        printf '%s' '{"status":"ok","command":"printf CODEX_FALLBACK","explanation":"fallback via Codex","assumptions":[],"riskHint":"safe"}' > "$result"
        ;;
    *)
        printf '%s' '{"status":"ok","command":"printf DEFAULT","explanation":"generated default","assumptions":[],"riskHint":"safe"}' > "$result"
        ;;
esac
`

const fakeClaudeScript = `#!/bin/sh
printf 'invoked\n' >> "$HOME/claude-invoked"
prompt=$(cat)
printf '%s' "$prompt" > "$HOME/claude-last-prompt"
if [ -n "${DATABASE_URL-}" ] || [ -n "${INTENT_PRIVATE_SENTINEL-}" ] || [ -n "${SSH_CONNECTION-}" ]; then
    printf 'prohibited environment reached provider\n' > "$HOME/claude-env-leaked"
fi
case "$prompt" in
    *INTENT_CASE_CLAUDE_SLOW_7Q*)
        sleep 10
        printf '%s' '{"is_error":false,"structured_output":{"status":"ok","command":"printf CLAUDE_SLOW","explanation":"slow Claude result","assumptions":[],"riskHint":"safe"}}'
        ;;
    *INTENT_CASE_FALLBACK_7Q*)
        printf '%s' 'not-json-from-claude'
        ;;
    *INTENT_CASE_CLAUDE_CLARIFY_7Q*)
        printf '%s' '{"is_error":false,"structured_output":{"status":"clarify","question":"Which Claude target should be used?"}}'
        ;;
    *INTENT_CASE_CLAUDE_NOEXEC_7Q*)
        printf '%s%s%s' '{"is_error":false,"structured_output":{"status":"ok","command":"touch ' "$HOME/claude-auto-ran" '","explanation":"Claude review result","assumptions":[],"riskHint":"safe"}}'
        ;;
    *INTENT_CASE_CLAUDE_SAFE_7Q*)
        case "$prompt" in
            *'"original":"prefix-INTENT_CASE_CLAUDE_SAFE_7Q"'*'"previous":"printf CLAUDE_ONE"'*'"generationIndex":1'*)
                printf '%s' '{"is_error":false,"structured_output":{"status":"ok","command":"printf CLAUDE_TWO","explanation":"Claude generated two","assumptions":[],"riskHint":"safe"}}'
                ;;
            *'"generationIndex":1'*)
                printf '%s' '{"is_error":false,"structured_output":{"status":"ok","command":"printf CLAUDE_BAD_REGEN","explanation":"Claude lost original intent","assumptions":[],"riskHint":"safe"}}'
                ;;
            *)
                printf '%s' '{"is_error":false,"structured_output":{"status":"ok","command":"printf CLAUDE_ONE","explanation":"Claude generated one","assumptions":[],"riskHint":"safe"}}'
                ;;
        esac
        ;;
    *)
        printf '%s' '{"is_error":false,"structured_output":{"status":"ok","command":"printf CLAUDE_DEFAULT","explanation":"Claude generated default","assumptions":[],"riskHint":"safe"}}'
        ;;
esac
`

func TestMVPRewriteWorkflowInPTY(t *testing.T) {
	root := repositoryRoot(t)
	binDir := t.TempDir()
	intentBinary := filepath.Join(binDir, "intent-sh")
	build := exec.Command("go", "build", "-o", intentBinary, "./cmd/intent-sh")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build intent-sh: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(fakeCodexScript), 0o755); err != nil {
		t.Fatalf("write fake Codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte(fakeClaudeScript), 0o755); err != nil {
		t.Fatalf("write fake Claude: %v", err)
	}

	cases := []shellCase{
		{name: "bash", executable: testBash(), args: []string{"--noprofile", "--norc", "-i"}, script: filepath.Join(root, "shell", "bash", "intent-sh.bash")},
		{name: "zsh", executable: "zsh", args: []string{"-f", "-i"}, script: filepath.Join(root, "shell", "zsh", "intent-sh.zsh")},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/empty-local-rejection", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, home := startMVPShell(t, tc, binDir, 5)
			defer shell.close(t)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "enter a command or intent before requesting a rewrite")
			if _, err := os.Stat(filepath.Join(home, "codex-invoked")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("empty input invoked provider: %v", err)
			}
			assertShellState(t, shell, "", 0, "", 0, "")
		})

		t.Run(tc.name+"/lifecycle", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, _ := startMVPShell(t, tc, binDir, 5)
			defer shell.close(t)

			original := "prefix-INTENT_CASE_SAFE_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "generated one")
			assertShellState(t, shell, "printf GEN_ONE", 14, original, 0, "safe")

			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "generated two")
			assertShellState(t, shell, "printf GEN_TWO", 14, original, 1, "safe")

			shell.writeBytes(t, []byte{'\x1b', 'u'})
			shell.readUntil(t, "restored the original buffer")
			assertShellState(t, shell, original, len(original), "", 0, "")
		})

		t.Run(tc.name+"/cursor-and-manual-edit", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, _ := startMVPShell(t, tc, binDir, 5)
			defer shell.close(t)

			original := "INTENT_CASE_SAFE_7QXYZ"
			shell.write(t, original)
			for range 3 {
				shell.writeBytes(t, []byte{'\x1b', '[', 'D'})
			}
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "generated one")
			shell.writeBytes(t, []byte{'\x1b', 'u'})
			shell.readUntil(t, "restored the original buffer")
			assertShellState(t, shell, original, len(original)-3, "", 0, "")

			// Move to the end, rewrite again, edit, and prove undo cannot clobber it.
			shell.writeBytes(t, []byte{'\x05'})
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "generated one")
			shell.write(t, "X")
			shell.writeBytes(t, []byte{'\x1b', 'u'})
			shell.readUntil(t, "buffer was edited; undo did not overwrite it")
			assertShellState(t, shell, "printf GEN_ONEX", 15, "", 0, "")

			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "generated default")
			assertShellState(t, shell, "printf DEFAULT", 14, "printf GEN_ONEX", 0, "safe")
		})

		t.Run(tc.name+"/clarification", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, _ := startMVPShell(t, tc, binDir, 5)
			defer shell.close(t)
			original := "INTENT_CASE_CLARIFY_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "Which directory should be searched?")
			assertShellState(t, shell, original, len(original), "", 0, "")
		})

		t.Run(tc.name+"/malformed-provider-output", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, _ := startMVPShell(t, tc, binDir, 5)
			defer shell.close(t)
			original := "INTENT_CASE_INVALID_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "Codex CLI returned an invalid structured result")
			assertShellState(t, shell, original, len(original), "", 0, "")
		})

		t.Run(tc.name+"/review-one-enter", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, home := startMVPShell(t, tc, binDir, 5)
			defer shell.close(t)
			marker := filepath.Join(home, "review-ran")
			shell.write(t, "INTENT_CASE_REVIEW_7Q")
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "REVIEW:")
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("review command ran during generation: %v", err)
			}
			shell.writeBytes(t, []byte{'\r'})
			waitForPath(t, marker)
		})

		t.Run(tc.name+"/danger-two-enter", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, home := startMVPShell(t, tc, binDir, 5)
			defer shell.close(t)
			marker := filepath.Join(home, "danger-ran")
			shell.write(t, "INTENT_CASE_DANGER_7Q")
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "DANGEROUS:")
			shell.writeBytes(t, []byte{'\r'})
			shell.readUntil(t, "Press Enter again to execute.")
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("first Enter executed dangerous result: %v", err)
			}
			shell.writeBytes(t, []byte{'\r'})
			waitForPath(t, marker)
		})

		if tc.name == "bash" {
			t.Run(tc.name+"/private-continuation-cannot-bypass-new-danger", func(t *testing.T) {
				requireCompatibleShell(t, tc)
				shell, home := startMVPShell(t, tc, binDir, 5)
				defer shell.close(t)

				// Safe acceptance maps the private continuation to accept-line
				// for that macro invocation.
				shell.write(t, "INTENT_CASE_SAFE_7Q")
				shell.writeBytes(t, []byte{'\x1b', 'g'})
				shell.readUntil(t, "generated one")
				shell.writeBytes(t, []byte{'\r'})
				shell.readUntil(t, promptMarker)

				marker := filepath.Join(home, "danger-ran")
				shell.write(t, "INTENT_CASE_DANGER_7Q")
				shell.writeBytes(t, []byte{'\x1b', 'g'})
				shell.readUntil(t, "DANGEROUS:")
				shell.writeBytes(t, []byte{'\x1e'})
				time.Sleep(100 * time.Millisecond)
				if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("private continuation executed a newly generated dangerous command: %v", err)
				}
				assertShellState(t, shell, "touch "+marker, len("touch "+marker), "INTENT_CASE_DANGER_7Q", 0, "dangerous")

				shell.writeBytes(t, []byte{'\r'})
				shell.readUntil(t, "Press Enter again to execute.")
				shell.writeBytes(t, []byte{'\r'})
				waitForPath(t, marker, shell)
			})
		}

		t.Run(tc.name+"/timeout", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, _ := startMVPShell(t, tc, binDir, 1)
			defer shell.close(t)
			original := "INTENT_CASE_SLOW_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "Codex CLI timed out")
			assertShellState(t, shell, original, len(original), "", 0, "")
		})

		t.Run(tc.name+"/cancellation", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, _ := startMVPShell(t, tc, binDir, 10)
			defer shell.close(t)
			original := "INTENT_CASE_SLOW_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "Ctrl+C to cancel")
			shell.writeBytes(t, []byte{'\x03'})
			shell.readUntil(t, "cancelled")
			if tc.name == "bash" {
				shell.readUntil(t, promptMarker)
			}
			assertShellState(t, shell, original, len(original), "", 0, "")
		})

		if tc.name == "bash" {
			t.Run(tc.name+"/signal-cancellation", func(t *testing.T) {
				requireCompatibleShell(t, tc)
				shell, _ := startMVPShell(t, tc, binDir, 10)
				defer shell.close(t)
				original := "INTENT_CASE_SLOW_7Q"
				shell.write(t, original)
				shell.writeBytes(t, []byte{'\x1b', 'g'})
				shell.readUntil(t, "Ctrl+C to cancel")
				if err := shell.cmd.Process.Signal(os.Interrupt); err != nil {
					t.Fatalf("signal Bash during rewrite: %v", err)
				}
				shell.readUntil(t, "cancelled")
				shell.readUntil(t, promptMarker)
				assertShellState(t, shell, original, len(original), "", 0, "")
			})
		}
	}

	for _, tc := range cases {
		t.Run(tc.name+"/claude-lifecycle", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, home := startMVPShellConfigured(t, tc, binDir, 5, config.ProviderClaude, []string{config.ProviderClaude, config.ProviderCodex}, nil)
			defer shell.close(t)

			original := "prefix-INTENT_CASE_CLAUDE_SAFE_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "Claude generated one")
			assertShellState(t, shell, "printf CLAUDE_ONE", 17, original, 0, "safe")

			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "Claude generated two")
			assertShellState(t, shell, "printf CLAUDE_TWO", 17, original, 1, "safe")

			shell.writeBytes(t, []byte{'\x1b', 'u'})
			shell.readUntil(t, "restored the original buffer")
			assertShellState(t, shell, original, len(original), "", 0, "")
			if _, err := os.Stat(filepath.Join(home, "claude-invoked")); err != nil {
				t.Fatalf("fake Claude was not invoked: %v", err)
			}
		})

		t.Run(tc.name+"/claude-clarification", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, _ := startMVPShellConfigured(t, tc, binDir, 5, config.ProviderClaude, []string{config.ProviderClaude, config.ProviderCodex}, nil)
			defer shell.close(t)
			original := "INTENT_CASE_CLAUDE_CLARIFY_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "Which Claude target should be used?")
			assertShellState(t, shell, original, len(original), "", 0, "")
		})

		t.Run(tc.name+"/claude-cancellation", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, _ := startMVPShellConfigured(t, tc, binDir, 10, config.ProviderClaude, []string{config.ProviderClaude, config.ProviderCodex}, nil)
			defer shell.close(t)
			original := "INTENT_CASE_CLAUDE_SLOW_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "Ctrl+C to cancel")
			shell.writeBytes(t, []byte{'\x03'})
			shell.readUntil(t, "cancelled")
			if tc.name == "bash" {
				shell.readUntil(t, promptMarker)
			}
			assertShellState(t, shell, original, len(original), "", 0, "")
		})

		t.Run(tc.name+"/auto-fallback-claude-to-codex", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			shell, home := startMVPShellConfigured(t, tc, binDir, 5, config.ProviderAuto, []string{config.ProviderClaude, config.ProviderCodex}, nil)
			defer shell.close(t)
			original := "INTENT_CASE_FALLBACK_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "fallback via Codex")
			assertShellState(t, shell, "printf CODEX_FALLBACK", 21, original, 0, "safe")
			for _, marker := range []string{"claude-invoked", "codex-invoked"} {
				if _, err := os.Stat(filepath.Join(home, marker)); err != nil {
					t.Fatalf("fallback marker %s missing: %v", marker, err)
				}
			}
		})

		t.Run(tc.name+"/claude-privacy-and-no-auto-execution", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			extraEnv := map[string]string{
				"DATABASE_URL":            "SECRET_DATABASE_URL_SENTINEL",
				"INTENT_PRIVATE_SENTINEL": "SECRET_ARBITRARY_ENV_SENTINEL",
				"SSH_CONNECTION":          "SECRET_REMOTE_ADDRESS_SENTINEL",
			}
			shell, home := startMVPShellConfigured(t, tc, binDir, 5, config.ProviderClaude, []string{config.ProviderClaude, config.ProviderCodex}, extraEnv)
			defer shell.close(t)
			original := "INTENT_CASE_CLAUDE_NOEXEC_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "REVIEW:")
			marker := filepath.Join(home, "claude-auto-ran")
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Claude result executed without Enter: %v", err)
			}
			assertShellState(t, shell, "touch "+marker, len("touch "+marker), original, 0, "review")

			promptData, err := os.ReadFile(filepath.Join(home, "claude-last-prompt"))
			if err != nil {
				t.Fatalf("read captured Claude prompt: %v", err)
			}
			promptText := string(promptData)
			if len(promptData) > 64*1024 {
				t.Fatalf("captured prompt was unexpectedly unbounded: %d bytes", len(promptData))
			}
			for _, want := range []string{`"buffer":"` + original + `"`, `"remote":true`, `"shell":"` + tc.name + `"`} {
				if !strings.Contains(promptText, want) {
					t.Fatalf("captured prompt omitted %q", want)
				}
			}
			for _, secret := range extraEnv {
				if strings.Contains(promptText, secret) {
					t.Fatalf("captured prompt leaked prohibited value %q", secret)
				}
			}
			if _, err := os.Stat(filepath.Join(home, "claude-env-leaked")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("prohibited environment reached provider: %v", err)
			}

			shell.writeBytes(t, []byte{'\x1b', 'u'})
			shell.readUntil(t, "restored the original buffer")
			shell.writeBytes(t, []byte{'\x15'})
		})
	}

	for _, tc := range cases {
		t.Run(tc.name+"/stale-response", func(t *testing.T) {
			requireCompatibleShell(t, tc)
			fakeDir := t.TempDir()
			fake := `#!/bin/sh
cat >/dev/null
printf '%s\0' 1 ok 'printf STALE' message fake safe reason stale-request
`
			if err := os.WriteFile(filepath.Join(fakeDir, "intent-sh"), []byte(fake), 0o755); err != nil {
				t.Fatal(err)
			}
			env := map[string]string{"PATH": fakeDir + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME": t.TempDir()}
			shell := startShellWith(t, tc, env, "source "+shellQuote(tc.script))
			defer shell.close(t)
			configureStateDump(t, shell)
			original := "stale input"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntil(t, "ignored a stale adapter response")
			assertShellState(t, shell, original, len(original), "", 0, "")
		})
	}
}

func startMVPShell(t *testing.T, tc shellCase, binDir string, timeout int) (*runningShell, string) {
	return startMVPShellConfigured(t, tc, binDir, timeout, config.ProviderCodex, []string{config.ProviderCodex, config.ProviderClaude}, nil)
}

func startMVPShellConfigured(t *testing.T, tc shellCase, binDir string, timeout int, mode string, priority []string, extraEnv map[string]string) (*runningShell, string) {
	t.Helper()
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	cfg := config.Defaults()
	cfg.Provider = mode
	cfg.Priority = append([]string(nil), priority...)
	cfg.TimeoutSeconds = timeout
	configPath := filepath.Join(xdg, "intent-sh", "config.toml")
	if err := config.WriteAt(configPath, cfg); err != nil {
		t.Fatalf("write PTY config: %v", err)
	}
	env := map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": xdg,
		"PATH":            binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	}
	for key, value := range extraEnv {
		env[key] = value
	}
	shell := startShellWith(t, tc, env, `eval "$(intent-sh init `+tc.name+`)"`)
	configureStateDump(t, shell)
	return shell, home
}

func configureStateDump(t *testing.T, shell *runningShell) {
	t.Helper()
	var command string
	if shell.name == "bash" {
		command = `__intent_sh_test_dump(){ printf '\nSTATE|BUFFER=%s|CURSOR=%s|ORIGINAL=%s|INDEX=%s|RISK=%s|\n' "$READLINE_LINE" "$READLINE_POINT" "$__intent_sh_original_buffer" "$__intent_sh_generation_index" "$__intent_sh_risk"; }; bind -x '"\ed":__intent_sh_test_dump'`
	} else {
		command = `function __intent_sh_test_dump() { printf '\nSTATE|BUFFER=%s|CURSOR=%s|ORIGINAL=%s|INDEX=%s|RISK=%s|\n' "$BUFFER" "$CURSOR" "$__intent_sh_original_buffer" "$__intent_sh_generation_index" "$__intent_sh_risk"; }; zle -N intent-sh-test-dump __intent_sh_test_dump; bindkey '^[d' intent-sh-test-dump`
	}
	shell.write(t, command)
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntil(t, promptMarker)
}

func assertShellState(t *testing.T, shell *runningShell, buffer string, cursor int, original string, index int, risk string) {
	t.Helper()
	shell.writeBytes(t, []byte{'\x1b', 'd'})
	want := fmt.Sprintf("STATE|BUFFER=%s|CURSOR=%d|ORIGINAL=%s|INDEX=%d|RISK=%s|", buffer, cursor, original, index, risk)
	output := shell.readUntil(t, want)
	if !strings.Contains(output, want) {
		t.Fatalf("state output = %q, want %q", output, want)
	}
}

func waitForPath(t *testing.T, path string, shells ...*runningShell) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if len(shells) > 0 && shells[0] != nil {
		drain:
			for {
				select {
				case chunk := <-shells[0].chunks:
					shells[0].pending += string(chunk)
				default:
					break drain
				}
			}
		}
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat marker %q: %v", path, err)
		}
		if time.Now().After(deadline) {
			output := ""
			if len(shells) > 0 && shells[0] != nil {
				output = shells[0].pending
			}
			t.Fatalf("marker %q was not created; output=%q", path, output)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
