package shelltest

import (
	"debug/elf"
	"debug/macho"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/taiwong148960/intent-sh/internal/config"
)

const fakeCodexScript = `#!/bin/sh
intent_test_mode=
if [ -f "$HOME/codex-test-mode" ]; then
    IFS= read -r intent_test_mode < "$HOME/codex-test-mode"
fi
case "${1-}" in
    --version)
        printf '%s\n' 'codex-cli 1.0.0-test'
        exit 0
        ;;
    login)
        if [ "${2-}" = status ]; then
            if [ "$intent_test_mode" = login-failure ]; then
                printf '%s\n' 'not logged in'
                exit 1
            fi
            printf '%s\n' 'logged in'
            exit 0
        fi
        ;;
    exec)
        if [ "${2-}" = --help ]; then
            if [ "$intent_test_mode" = capability-failure ]; then
                printf '%s\n' '--ephemeral'
                exit 0
            fi
            printf '%s\n' '--ephemeral --ignore-user-config --ignore-rules --sandbox --output-schema --output-last-message --skip-git-repo-check'
            exit 0
        fi
        ;;
esac
case "$intent_test_mode" in
    login-failure)
        printf '%s\n' 'not logged in' >&2
        exit 1
        ;;
    capability-failure)
        printf '%s\n' 'unknown option: --ignore-user-config' >&2
        exit 2
        ;;
esac
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
printf '%s' "$prompt" > "$HOME/codex-last-prompt"
case "$prompt" in
	*INTENT_CASE_RESIZE_7Q*)
		sleep 1
		printf '%s' '{"status":"ok","command":"printf RESIZED","explanation":"resize result","assumptions":[],"riskHint":"safe"}' > "$result"
		;;
    *INTENT_CASE_SLOW_7Q*)
		intent_fake_child=
		intent_fake_cancel() {
			printf 'phase=cancel-signal\nsignal=%s\n' "$1" >> "$HOME/codex-provider-phase"
			if [ -n "$intent_fake_child" ]; then
				kill -TERM "$intent_fake_child" 2>/dev/null || :
				wait "$intent_fake_child" 2>/dev/null || :
			fi
			printf 'phase=exited\n' >> "$HOME/codex-provider-phase"
			exit 130
		}
		trap 'intent_fake_cancel INT' INT
		trap 'intent_fake_cancel TERM' TERM
		printf 'phase=started\n' > "$HOME/codex-provider-phase"
		sleep 10 &
		intent_fake_child=$!
		printf '%s %s\n' "$$" "$intent_fake_child" > "$HOME/codex-provider-pid"
		if ! wait "$intent_fake_child"; then
			printf 'phase=child-interrupted\n' >> "$HOME/codex-provider-phase"
			exit 130
		fi
		trap - INT TERM
		printf 'phase=completed\n' >> "$HOME/codex-provider-phase"
		printf '%s' '{"status":"ok","command":"printf SLOW","explanation":"slow result","assumptions":[],"riskHint":"safe"}' > "$result"
		;;
    *INTENT_CASE_INVALID_7Q*)
        printf '%s' 'not-json' > "$result"
        ;;
	*INTENT_CASE_EXCESSIVE_7Q*)
		head -c 1100000 /dev/zero | tr '\0' x > "$result"
		;;
	*INTENT_CASE_CRASH_7Q*)
		exit 42
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
		intent_fake_child=
		intent_fake_cancel() {
			printf 'phase=cancel-signal\nsignal=%s\n' "$1" >> "$HOME/claude-provider-phase"
			if [ -n "$intent_fake_child" ]; then
				kill -TERM "$intent_fake_child" 2>/dev/null || :
				wait "$intent_fake_child" 2>/dev/null || :
			fi
			printf 'phase=exited\n' >> "$HOME/claude-provider-phase"
			exit 130
		}
		trap 'intent_fake_cancel INT' INT
		trap 'intent_fake_cancel TERM' TERM
		printf 'phase=started\n' > "$HOME/claude-provider-phase"
		sleep 10 &
		intent_fake_child=$!
		printf '%s %s\n' "$$" "$intent_fake_child" > "$HOME/claude-provider-pid"
		if ! wait "$intent_fake_child"; then
			printf 'phase=child-interrupted\n' >> "$HOME/claude-provider-phase"
			exit 130
		fi
		trap - INT TERM
		printf 'phase=completed\n' >> "$HOME/claude-provider-phase"
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
            *'"previous":"printf CLAUDE_ONE"'*'"generationIndex":1'*)
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
	binDir := buildMVPTools(t, root)

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
printf '%s\0' 2 ok 'printf STALE' message fake safe reason stale-request
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

func TestBashCancellationAndTeardownQualification(t *testing.T) {
	root := repositoryRoot(t)
	bash := shellCase{name: "bash", executable: testBash(), args: []string{"--noprofile", "--norc", "-i"}, script: filepath.Join(root, "shell", "bash", "intent-sh.bash")}
	requireCompatibleShell(t, bash)
	binDir := buildMVPTools(t, root)

	for _, delivery := range []string{"terminal-byte", "process-signal"} {
		t.Run(delivery, func(t *testing.T) {
			temporaryRoot := t.TempDir()
			shell, home := startMVPShellConfigured(t, bash, binDir, 10, config.ProviderCodex, []string{config.ProviderCodex, config.ProviderClaude}, map[string]string{"TMPDIR": temporaryRoot})
			defer shell.close(t)
			beforeTTY, afterTTY, trapRecord := configureCancellationProbe(t, shell, home)

			original := "CURSOR-INTENT_CASE_SLOW_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'c'})
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntilTimeout(t, "Ctrl+C to cancel", 10*time.Second)
			waitForFileText(t, filepath.Join(home, "codex-provider-phase"), "phase=started", 5*time.Second)
			if delivery == "terminal-byte" {
				shell.writeBytes(t, []byte{'\x03'})
			} else if err := shell.cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatalf("signal Bash during rewrite: %v", err)
			}
			shell.readUntilTimeout(t, "cancelled", 10*time.Second)
			shell.readUntilTimeout(t, promptMarker, 10*time.Second)
			assertShellState(t, shell, original, 4, "", 0, "")
			assertFakeProviderTeardown(t, home, "codex", "INT")
			assertCancellationProbeRestored(t, shell, beforeTTY, afterTTY, trapRecord)
			assertNoFallbackOrTarget(t, home)

			clearEditableLine(t, shell)
			shell.write(t, "INTENT_CASE_SAFE_7Q")
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntilTimeout(t, "generated one", 10*time.Second)
			assertShellState(t, shell, "printf GEN_ONE", len("printf GEN_ONE"), "INTENT_CASE_SAFE_7Q", 0, "safe")
			shell.writeBytes(t, []byte{'\x1b', 'u'})
			shell.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
			assertShellState(t, shell, "INTENT_CASE_SAFE_7Q", len("INTENT_CASE_SAFE_7Q"), "", 0, "")
			assertNoTemporaryState(t, temporaryRoot)
		})
	}

	t.Run("timeout", func(t *testing.T) {
		temporaryRoot := t.TempDir()
		shell, home := startMVPShellConfigured(t, bash, binDir, 1, config.ProviderCodex, []string{config.ProviderCodex, config.ProviderClaude}, map[string]string{"TMPDIR": temporaryRoot})
		defer shell.close(t)
		beforeTTY, afterTTY, trapRecord := configureCancellationProbe(t, shell, home)
		original := "INTENT_CASE_SLOW_7Q"
		shell.write(t, original)
		shell.writeBytes(t, []byte{'\x1b', 'g'})
		shell.readUntilTimeout(t, "Codex CLI timed out", 10*time.Second)
		shell.readUntilTimeout(t, promptMarker, 10*time.Second)
		assertShellState(t, shell, original, len(original), "", 0, "")
		assertFakeProviderTeardown(t, home, "codex", "TERM")
		assertCancellationProbeRestored(t, shell, beforeTTY, afterTTY, trapRecord)
		assertNoFallbackOrTarget(t, home)
		assertNoTemporaryState(t, temporaryRoot)
	})

	t.Run("pty-closure", func(t *testing.T) {
		temporaryRoot := t.TempDir()
		shell, home := startMVPShellConfigured(t, bash, binDir, 10, config.ProviderCodex, []string{config.ProviderCodex, config.ProviderClaude}, map[string]string{"TMPDIR": temporaryRoot})
		shell.write(t, "INTENT_CASE_SLOW_7Q")
		shell.writeBytes(t, []byte{'\x1b', 'g'})
		shell.readUntilTimeout(t, "Ctrl+C to cancel", 10*time.Second)
		waitForFileText(t, filepath.Join(home, "codex-provider-phase"), "phase=started", 5*time.Second)
		_ = shell.file.Close()
		_ = shell.cmd.Process.Signal(syscall.SIGHUP)
		waitForShellExit(t, shell)
		assertFakeProviderTeardown(t, home, "codex", "INT")
		assertNoFallbackOrTarget(t, home)
		assertNoTemporaryState(t, temporaryRoot)
	})

	t.Run("ordinary-shell-exit", func(t *testing.T) {
		temporaryRoot := t.TempDir()
		shell, home := startMVPShellConfigured(t, bash, binDir, 10, config.ProviderCodex, []string{config.ProviderCodex, config.ProviderClaude}, map[string]string{"TMPDIR": temporaryRoot})
		shell.write(t, "exit")
		shell.writeBytes(t, []byte{'\r'})
		waitForShellExit(t, shell)
		_ = shell.file.Close()
		assertNoFallbackOrTarget(t, home)
		assertNoTemporaryState(t, temporaryRoot)
	})

	t.Run("interrupted-initialization", func(t *testing.T) {
		temporaryRoot := t.TempDir()
		home := t.TempDir()
		fakeBin := t.TempDir()
		fakeInit := `#!/bin/sh
intent_init_child=
intent_init_stop() {
    if [ -n "$intent_init_child" ]; then
        kill -TERM "$intent_init_child" 2>/dev/null || :
        wait "$intent_init_child" 2>/dev/null || :
    fi
    printf 'phase=exited\n' >> "$HOME/init-phase"
    exit 129
}
trap intent_init_stop HUP INT TERM
printf 'phase=started\n' > "$HOME/init-phase"
sleep 10 &
intent_init_child=$!
printf '%s %s\n' "$$" "$intent_init_child" > "$HOME/init-pids"
wait "$intent_init_child"
printf 'touch %s\n' "$HOME/init-target"
`
		if err := os.WriteFile(filepath.Join(fakeBin, "intent-sh"), []byte(fakeInit), 0o755); err != nil {
			t.Fatal(err)
		}
		shell := startShellWith(t, bash, map[string]string{
			"HOME": home, "TMPDIR": temporaryRoot,
			"PATH": fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		}, "")
		shell.write(t, `eval "$(intent-sh init bash)"`)
		shell.writeBytes(t, []byte{'\r'})
		waitForFileText(t, filepath.Join(home, "init-phase"), "phase=started", 5*time.Second)
		_ = shell.file.Close()
		_ = shell.cmd.Process.Signal(syscall.SIGHUP)
		waitForShellExit(t, shell)
		waitForFileText(t, filepath.Join(home, "init-phase"), "phase=exited", 5*time.Second)
		assertPIDMarkerStopped(t, filepath.Join(home, "init-pids"))
		if _, err := os.Stat(filepath.Join(home, "init-target")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("interrupted initialization executed its target: %v", err)
		}
		assertNoTemporaryState(t, temporaryRoot)
	})
}

func TestNativeProviderFailureMatrixInPTY(t *testing.T) {
	root := repositoryRoot(t)
	baseTools := buildMVPTools(t, root)
	cases := []struct {
		name      string
		prompt    string
		mode      string
		removeCLI bool
		timeout   int
		wantCodex bool
	}{
		{name: "unavailable-executable", prompt: "INTENT_CASE_PROVIDER_UNAVAILABLE_7Q", removeCLI: true, timeout: 5},
		{name: "login-failure", prompt: "INTENT_CASE_PROVIDER_LOGIN_7Q", mode: "login-failure", timeout: 5},
		{name: "capability-failure", prompt: "INTENT_CASE_PROVIDER_CAPABILITY_7Q", mode: "capability-failure", timeout: 5},
		{name: "timeout-fallback", prompt: "INTENT_CASE_SLOW_7Q", timeout: 1, wantCodex: true},
		{name: "malformed-result-fallback", prompt: "INTENT_CASE_INVALID_7Q", timeout: 5, wantCodex: true},
		{name: "excessive-output-fallback", prompt: "INTENT_CASE_EXCESSIVE_7Q", timeout: 5, wantCodex: true},
		{name: "provider-crash-fallback", prompt: "INTENT_CASE_CRASH_7Q", timeout: 5, wantCodex: true},
	}

	for _, shellCase := range nativeConformanceShells(root) {
		shellCase := shellCase
		t.Run(shellCase.name, func(t *testing.T) {
			requireCompatibleShell(t, shellCase)
			for _, testCase := range cases {
				testCase := testCase
				t.Run(testCase.name, func(t *testing.T) {
					tools := cloneMVPTools(t, baseTools)
					if testCase.removeCLI {
						if err := os.Remove(filepath.Join(tools, "codex")); err != nil {
							t.Fatal(err)
						}
					}
					shell, home := startMVPShellConfigured(t, shellCase, tools, testCase.timeout, config.ProviderAuto, []string{config.ProviderCodex, config.ProviderClaude}, nil)
					defer shell.close(t)
					if testCase.mode != "" {
						if err := os.WriteFile(filepath.Join(home, "codex-test-mode"), []byte(testCase.mode+"\n"), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					shell.write(t, testCase.prompt)
					shell.writeBytes(t, []byte{'\x1b', 'g'})
					shell.readUntilTimeout(t, "Claude generated default", 15*time.Second)
					assertShellState(t, shell, "printf CLAUDE_DEFAULT", len("printf CLAUDE_DEFAULT"), testCase.prompt, 0, "safe")
					if _, err := os.Stat(filepath.Join(home, "claude-invoked")); err != nil {
						t.Fatalf("fallback Claude fixture was not invoked: %v", err)
					}
					_, codexErr := os.Stat(filepath.Join(home, "codex-invoked"))
					if testCase.wantCodex && codexErr != nil {
						t.Fatalf("primary Codex fixture was not invoked: %v", codexErr)
					}
					if !testCase.wantCodex && !errors.Is(codexErr, os.ErrNotExist) {
						t.Fatalf("failed Codex probe unexpectedly generated: %v", codexErr)
					}
				})
			}

			t.Run("explicit-provider-no-fallback", func(t *testing.T) {
				tools := cloneMVPTools(t, baseTools)
				shell, home := startMVPShellConfigured(t, shellCase, tools, 5, config.ProviderCodex, []string{config.ProviderCodex, config.ProviderClaude}, nil)
				defer shell.close(t)
				original := "INTENT_CASE_CRASH_7Q"
				shell.write(t, original)
				shell.writeBytes(t, []byte{'\x1b', 'g'})
				shell.readUntilTimeout(t, "Codex CLI", 10*time.Second)
				assertShellState(t, shell, original, len(original), "", 0, "")
				if _, err := os.Stat(filepath.Join(home, "claude-invoked")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("explicit provider failure invoked fallback: %v", err)
				}
			})
		})
	}
}

func cloneMVPTools(t *testing.T, source string) string {
	t.Helper()
	destination := t.TempDir()
	for _, name := range []string{"intent-sh", "codex", "claude"} {
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destination, name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return destination
}

func configureCancellationProbe(t *testing.T, shell *runningShell, home string) (string, string, string) {
	t.Helper()
	beforeTTY := filepath.Join(home, "tty-before")
	afterTTY := filepath.Join(home, "tty-after")
	trapRecord := filepath.Join(home, "trap-after")
	command := "trap '__intent_test_restored_int=1' INT; stty -g > " + shellQuote(beforeTTY) + `; __intent_test_cursor(){ READLINE_POINT=4; }; bind -x '"\ec":__intent_test_cursor'; printf '\nCANCEL_PROBE_READY\n'`
	shell.write(t, command)
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntilTimeout(t, "CANCEL_PROBE_READY", 5*time.Second)
	shell.readUntilTimeout(t, promptMarker, 5*time.Second)
	return beforeTTY, afterTTY, trapRecord
}

func assertCancellationProbeRestored(t *testing.T, shell *runningShell, beforeTTY, afterTTY, trapRecord string) {
	t.Helper()
	clearEditableLine(t, shell)
	command := "stty -g > " + shellQuote(afterTTY) + "; trap -p INT > " + shellQuote(trapRecord) + `; printf '\nORIGINAL_TRAP_VALUE=%s\n' "${__intent_test_restored_int-}"`
	shell.write(t, command)
	shell.writeBytes(t, []byte{'\r'})
	output := shell.readUntilTimeout(t, "ORIGINAL_TRAP_VALUE=", 5*time.Second)
	shell.readUntilTimeout(t, promptMarker, 5*time.Second)
	if strings.Contains(output, "ORIGINAL_TRAP_VALUE=1") {
		t.Fatal("the caller's INT trap ran instead of being restored")
	}
	before, err := os.ReadFile(beforeTTY)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(afterTTY)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(before)) != strings.TrimSpace(string(after)) {
		t.Fatalf("TTY state changed across provider teardown: before=%q after=%q", before, after)
	}
	restoredTrap, err := os.ReadFile(trapRecord)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(restoredTrap), "__intent_test_restored_int=1") {
		t.Fatalf("caller INT trap was not restored: %q", restoredTrap)
	}
}

func assertFakeProviderTeardown(t *testing.T, home, providerName, signalName string) {
	t.Helper()
	phasePath := filepath.Join(home, providerName+"-provider-phase")
	waitForFileText(t, phasePath, "phase=exited", 5*time.Second)
	phases, err := os.ReadFile(phasePath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(phases)
	if strings.Count(text, "phase=cancel-signal") != 1 || !strings.Contains(text, "signal="+signalName) || strings.Contains(text, "phase=completed") {
		t.Fatalf("unexpected bounded provider phases: %q", text)
	}
	assertPIDMarkerStopped(t, filepath.Join(home, providerName+"-provider-pid"))
}

func assertPIDMarkerStopped(t *testing.T, path string) {
	t.Helper()
	pids, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range strings.Fields(string(pids)) {
		pid, parseErr := strconv.Atoi(field)
		if parseErr != nil {
			t.Fatalf("invalid provider pid marker %q", field)
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			killErr := syscall.Kill(pid, 0)
			if errors.Is(killErr, syscall.ESRCH) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("provider process %d survived teardown: %v", pid, killErr)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func waitForFileText(t *testing.T, path, expected string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), expected) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("bounded phase marker %q did not contain %q", path, expected)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertNoFallbackOrTarget(t *testing.T, home string) {
	t.Helper()
	for _, name := range []string{"claude-invoked", "danger-ran", "review-ran"} {
		if _, err := os.Stat(filepath.Join(home, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("provider teardown created forbidden marker %s: %v", name, err)
		}
	}
}

func assertNoTemporaryState(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "intent-sh-bash.") || strings.HasPrefix(entry.Name(), "intent-sh-cancel.") || strings.HasPrefix(entry.Name(), "intent-sh-provider-") {
			t.Fatalf("provider teardown left temporary state %s", entry.Name())
		}
	}
}

func waitForShellExit(t *testing.T, shell *runningShell) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- shell.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = shell.cmd.Process.Kill()
		<-done
		t.Fatal("shell did not exit after terminal teardown")
	}
}

func TestNativeEditorsUnicodeCursorRoundTripInPTY(t *testing.T) {
	root := repositoryRoot(t)
	binDir := buildMVPTools(t, root)
	cases := []struct {
		shell        shellCase
		setCursor    string
		nativeCursor int
	}{
		{
			shell:        shellCase{name: "zsh", executable: "zsh", args: []string{"-f", "-i"}, script: filepath.Join(root, "shell", "zsh", "intent-sh.zsh")},
			setCursor:    `function __intent_sh_test_cursor() { CURSOR=3; }; zle -N intent-sh-test-cursor __intent_sh_test_cursor; bindkey '^[c' intent-sh-test-cursor`,
			nativeCursor: 3,
		},
		{
			shell:        shellCase{name: "bash", executable: testBash(), args: []string{"--noprofile", "--norc", "-i"}, script: filepath.Join(root, "shell", "bash", "intent-sh.bash")},
			setCursor:    `__intent_sh_test_cursor(){ READLINE_POINT=6; }; bind -x '"\ec":__intent_sh_test_cursor'`,
			nativeCursor: 6,
		},
	}
	for _, tc := range cases {
		t.Run(tc.shell.name, func(t *testing.T) {
			requireCompatibleShell(t, tc.shell)
			shell, home := startMVPShell(t, tc.shell, binDir, 5)
			defer shell.close(t)
			shell.write(t, tc.setCursor)
			shell.writeBytes(t, []byte{'\r'})
			shell.readUntilTimeout(t, promptMarker, 30*time.Second)

			original := "前e\u0301後INTENT_CASE_SAFE_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'c'})
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntilTimeout(t, "generated one", 30*time.Second)
			promptData, err := os.ReadFile(filepath.Join(home, "codex-last-prompt"))
			if err != nil {
				t.Fatalf("read Unicode provider prompt: %v", err)
			}
			for _, want := range []string{`"buffer":"` + original + `"`, `"cursor":6`} {
				if !strings.Contains(string(promptData), want) {
					t.Fatalf("Unicode provider prompt omitted %q: %s", want, promptData)
				}
			}
			shell.writeBytes(t, []byte{'\x1b', 'u'})
			shell.readUntilTimeout(t, "restored the original buffer", 30*time.Second)
			assertShellState(t, shell, original, tc.nativeCursor, "", 0, "")
		})
	}
}

func buildMVPTools(t *testing.T, root string) string {
	t.Helper()
	requireQualificationArchitecture(t)
	binDir := t.TempDir()
	intentBinary := filepath.Join(binDir, "intent-sh")
	prebuilt := os.Getenv("INTENT_SH_TEST_BINARY")
	if prebuilt == "" {
		if os.Getenv("INTENT_SH_REQUIRE_PREBUILT") == "1" {
			t.Fatal("INTENT_SH_TEST_BINARY is required for artifact qualification")
		}
		build := exec.Command("go", "build", "-o", intentBinary, "./cmd/intent-sh")
		build.Dir = root
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build intent-sh: %v: %s", err, output)
		}
	} else {
		copyPrebuiltBinary(t, prebuilt, intentBinary)
	}
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(fakeCodexScript), 0o755); err != nil {
		t.Fatalf("write fake Codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte(fakeClaudeScript), 0o755); err != nil {
		t.Fatalf("write fake Claude: %v", err)
	}
	return binDir
}

func requireQualificationArchitecture(t *testing.T) {
	t.Helper()
	expected := os.Getenv("INTENT_SH_REQUIRE_GOARCH")
	if expected == "" {
		if qualificationIsStrict() {
			t.Fatal("INTENT_SH_REQUIRE_GOARCH is required in strict integration qualification")
		}
		return
	}
	if expected != runtime.GOARCH {
		qualificationSkipf(t, "qualification requires architecture %s; host is %s", expected, runtime.GOARCH)
	}
}

func copyPrebuiltBinary(t *testing.T, source, destination string) {
	t.Helper()
	info, err := os.Lstat(source)
	if err != nil {
		t.Fatalf("inspect INTENT_SH_TEST_BINARY: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		t.Fatal("INTENT_SH_TEST_BINARY must be a regular executable file")
	}
	if err := validateNativeBinaryArchitecture(source); err != nil {
		t.Fatalf("validate INTENT_SH_TEST_BINARY: %v", err)
	}
	input, err := os.Open(source)
	if err != nil {
		t.Fatalf("open INTENT_SH_TEST_BINARY: %v", err)
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		_ = input.Close()
		t.Fatalf("create disposable intent-sh binary: %v", err)
	}
	_, copyErr := io.Copy(output, io.LimitReader(input, 128<<20))
	inputCloseErr := input.Close()
	outputCloseErr := output.Close()
	if copyErr != nil || inputCloseErr != nil || outputCloseErr != nil {
		t.Fatal("copy INTENT_SH_TEST_BINARY into disposable test path")
	}
	versionCommand := exec.Command(destination, "version")
	if coverageDirectory, coverageErr := qualificationExecutableCoverageDirectory(); coverageErr != nil {
		t.Fatal(coverageErr)
	} else if coverageDirectory != "" {
		versionCommand.Env = replaceEnvironment(os.Environ(), map[string]string{"GOCOVERDIR": coverageDirectory})
	}
	versionOutput, err := versionCommand.CombinedOutput()
	if err != nil || len(versionOutput) == 0 || len(versionOutput) > 1024 {
		t.Fatal("INTENT_SH_TEST_BINARY failed its bounded version probe")
	}
}

func validateNativeBinaryArchitecture(path string) error {
	switch runtime.GOOS {
	case "darwin":
		file, err := macho.Open(path)
		if err != nil {
			return fmt.Errorf("open Mach-O executable: %w", err)
		}
		defer file.Close()
		expected := map[string]macho.Cpu{"amd64": macho.CpuAmd64, "arm64": macho.CpuArm64}[runtime.GOARCH]
		if expected == 0 || file.Cpu != expected {
			return fmt.Errorf("Mach-O architecture does not match %s", runtime.GOARCH)
		}
	case "linux":
		file, err := elf.Open(path)
		if err != nil {
			return fmt.Errorf("open ELF executable: %w", err)
		}
		defer file.Close()
		expected := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}[runtime.GOARCH]
		if expected == 0 || file.Machine != expected {
			return fmt.Errorf("ELF architecture does not match %s", runtime.GOARCH)
		}
	default:
		return fmt.Errorf("unsupported qualification host %s", runtime.GOOS)
	}
	return nil
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
		command = `__intent_sh_test_dump(){ printf '\nSTATE|BUFFER=%s|CURSOR=%s|ORIGINAL=%s|INDEX=%s|RISK=%s|\n' "$READLINE_LINE" "$READLINE_POINT" "$__intent_sh_original_buffer" "$__intent_sh_generation_index" "$__intent_sh_risk"; }; __intent_sh_test_clear(){ READLINE_LINE=; READLINE_POINT=0; }; bind -x '"\ed":__intent_sh_test_dump'; bind -x '"\ek":__intent_sh_test_clear'`
	} else {
		command = `function __intent_sh_test_dump() { printf '\nSTATE|BUFFER=%s|CURSOR=%s|ORIGINAL=%s|INDEX=%s|RISK=%s|\n' "$BUFFER" "$CURSOR" "$__intent_sh_original_buffer" "$__intent_sh_generation_index" "$__intent_sh_risk"; }; function __intent_sh_test_clear() { BUFFER=; CURSOR=0; }; zle -N intent-sh-test-dump __intent_sh_test_dump; zle -N intent-sh-test-clear __intent_sh_test_clear; bindkey '^[d' intent-sh-test-dump; bindkey '^[k' intent-sh-test-clear`
	}
	shell.write(t, command)
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntil(t, promptMarker)
	shell.clearSequence = []byte{'\x1b', 'k'}
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
	waitForPathTimeout(t, path, 2*time.Second, shells...)
}

func waitForPathTimeout(t *testing.T, path string, timeout time.Duration, shells ...*runningShell) {
	t.Helper()
	deadline := time.Now().Add(timeout)
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
