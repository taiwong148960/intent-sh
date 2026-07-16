package shelltest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const testedBleshVersion = "0.4.0-nightly+d69e4d5"

// TestBleshEditCommandContract is an opt-in compatibility probe for the
// separately distributed editor. Ordinary unit tests do not download or load
// ble.sh; CI and developers provide the verified path explicitly.
func TestBleshEditCommandContract(t *testing.T) {
	blesh := requireTestBlesh(t)
	bash := requireBleshMatrixBash(t)

	for _, mode := range []string{"emacs", "vi"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			initialize := fmt.Sprintf(`set -o %s; source %s --attach=none --norc --inputrc=none; bleopt char_width_mode=west; bleopt char_width_version=15.0; bleopt highlight_syntax=; bleopt highlight_filename=; bleopt highlight_variable=; bleopt complete_auto_complete=; __intent_probe_edit() { printf '\nPROBE_BEFORE|BUFFER=%%s|CURSOR=%%s|\n' "$READLINE_LINE" "$READLINE_POINT"; READLINE_LINE=REPLACED_BUFFER; READLINE_POINT=8; }; __intent_probe_dump() { printf '\nPROBE_AFTER|BUFFER=%%s|CURSOR=%%s|VERSION=%%s|ATTACHED=%%s|\n' "$READLINE_LINE" "$READLINE_POINT" "$BLE_VERSION" "${BLE_ATTACHED-}"; }; ble-bind -x 'M-g' '__intent_probe_edit'; ble-bind -x 'M-d' '__intent_probe_dump'; ble-attach`, mode, shellQuote(blesh))
			tc := shellCase{name: "bash", executable: bash, args: []string{"--noprofile", "--norc", "-i"}}
			shell := startBashWithRCAndTerminalResponses(t, tc, map[string]string{"HOME": home, "TERM": "xterm-256color", "PATH": "/usr/bin:/bin:/usr/sbin:/sbin"}, initialize)
			defer shell.close(t)
			time.Sleep(250 * time.Millisecond)
			shell.write(t, "printf '\\nPROBE_READY\\n'")
			shell.writeBytes(t, []byte{'\r'})
			shell.readUntilTimeout(t, "PROBE_READY", 30*time.Second)
			shell.readUntilTimeout(t, promptMarker, 30*time.Second)

			shell.write(t, "ORIG")
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntilTimeout(t, "PROBE_BEFORE|BUFFER=ORIG|CURSOR=4|", 30*time.Second)
			shell.writeBytes(t, []byte{'\x1b', 'd'})
			shell.readUntilTimeout(t, "PROBE_AFTER|BUFFER=REPLACED_BUFFER|CURSOR=8|VERSION="+testedBleshVersion+"|ATTACHED=1|", 30*time.Second)
		})
	}
}

func TestBleshAdapterInitialization(t *testing.T) {
	root := repositoryRoot(t)
	blesh := requireTestBlesh(t)
	bash := requireBleshMatrixBash(t)
	adapter := filepath.Join(root, "shell", "bash", "intent-sh.bash")

	for _, mode := range []string{"emacs", "vi"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			initialize := fmt.Sprintf(`set -o %s; source %s --attach=none --norc --inputrc=none; bleopt char_width_mode=west; bleopt char_width_version=15.0; bleopt highlight_syntax=; bleopt highlight_filename=; bleopt highlight_variable=; bleopt complete_auto_complete=; __intent_probe_init() { source %s; local status=$?; printf '\nADAPTER|STATUS=%%s|PROTOCOL=%%s|BACKEND=%%s|VERSION=%%s|READY=%%s|FAILURE=%%s|\n' "$status" "$INTENT_SH_ADAPTER_PROTOCOL" "$INTENT_SH_ADAPTER_BACKEND" "$INTENT_SH_ADAPTER_EDITOR_VERSION" "$INTENT_SH_ADAPTER_READY" "$INTENT_SH_ADAPTER_FAILURE"; }; ble-attach`, mode, shellQuote(blesh), shellQuote(adapter))
			tc := shellCase{name: "bash", executable: bash, args: []string{"--noprofile", "--norc", "-i"}}
			shell := startBashWithRCAndTerminalResponses(t, tc, map[string]string{"HOME": home, "TERM": "xterm-256color", "PATH": "/usr/bin:/bin:/usr/sbin:/sbin"}, initialize)
			defer shell.close(t)
			time.Sleep(250 * time.Millisecond)

			shell.write(t, "__intent_probe_init")
			shell.writeBytes(t, []byte{'\r'})
			shell.readUntilTimeout(t, "ADAPTER|STATUS=0|PROTOCOL=2|BACKEND=blesh|VERSION="+testedBleshVersion+"|READY=1|FAILURE=|", 30*time.Second)
			shell.readUntilTimeout(t, promptMarker, 30*time.Second)
			shell.write(t, "ORIG")
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntilTimeout(t, "intent-sh: binary not found on PATH", 30*time.Second)
		})
	}
}

func TestBleshOrdinaryAcceptanceContract(t *testing.T) {
	blesh := requireTestBlesh(t)
	bash := requireBleshMatrixBash(t)
	marker := filepath.Join(t.TempDir(), "accepted")
	home := t.TempDir()
	initialize := fmt.Sprintf(`set -o emacs; source %s --attach=none --norc --inputrc=none; bleopt char_width_mode=west; bleopt char_width_version=15.0; bleopt highlight_syntax=; bleopt highlight_filename=; bleopt highlight_variable=; bleopt complete_auto_complete=; __intent_probe_top_level() { printf '\nBLESH_FUNCTION_RAN\n'; }; ble-attach`, shellQuote(blesh))
	tc := shellCase{name: "bash", executable: bash, args: []string{"--noprofile", "--norc", "-i"}}
	shell := startBashWithRCAndTerminalResponses(t, tc, map[string]string{"HOME": home, "TERM": "xterm-256color", "PATH": "/usr/bin:/bin:/usr/sbin:/sbin"}, initialize)
	defer shell.close(t)
	time.Sleep(250 * time.Millisecond)
	shell.write(t, "printf '\\nBLESH_READY\\n'")
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntilTimeout(t, "BLESH_READY", 30*time.Second)
	shell.readUntilTimeout(t, promptMarker, 30*time.Second)
	shell.write(t, "__intent_probe_top_level")
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntilTimeout(t, "BLESH_FUNCTION_RAN", 30*time.Second)
	shell.readUntilTimeout(t, promptMarker, 30*time.Second)
	shell.write(t, "touch "+shellQuote(marker))
	shell.writeBytes(t, []byte{'\r'})
	waitForPathTimeout(t, marker, 30*time.Second, shell)
}

func TestBleshAcceptAdviceDelegationContract(t *testing.T) {
	blesh := requireTestBlesh(t)
	bash := requireBleshMatrixBash(t)
	marker := filepath.Join(t.TempDir(), "accepted-through-advice")
	home := t.TempDir()
	initialize := fmt.Sprintf(`set -o emacs; source %s --attach=none --norc --inputrc=none; bleopt char_width_mode=west; bleopt char_width_version=15.0; bleopt highlight_syntax=; bleopt highlight_filename=; bleopt highlight_variable=; bleopt complete_auto_complete=; ble/function#advice around ble/widget/default/accept-line 'ble/function#advice/do'; ble-attach`, shellQuote(blesh))
	tc := shellCase{name: "bash", executable: bash, args: []string{"--noprofile", "--norc", "-i"}}
	shell := startBashWithRCAndTerminalResponses(t, tc, map[string]string{"HOME": home, "TERM": "xterm-256color", "PATH": "/usr/bin:/bin:/usr/sbin:/sbin"}, initialize)
	defer shell.close(t)
	time.Sleep(250 * time.Millisecond)
	shell.write(t, "printf '\\nBLESH_ADVICE_READY\\n'")
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntilTimeout(t, "BLESH_ADVICE_READY", 30*time.Second)
	shell.readUntilTimeout(t, promptMarker, 30*time.Second)
	shell.write(t, "touch "+shellQuote(marker))
	shell.writeBytes(t, []byte{'\r'})
	waitForPathTimeout(t, marker, 30*time.Second, shell)
}

func TestBleshAcceptAdviceRuntimeDelegationContract(t *testing.T) {
	blesh := requireTestBlesh(t)
	bash := requireBleshMatrixBash(t)
	marker := filepath.Join(t.TempDir(), "accepted-through-runtime-advice")
	home := t.TempDir()
	initialize := fmt.Sprintf(`set -o emacs; source %s --attach=none --norc --inputrc=none; bleopt char_width_mode=west; bleopt char_width_version=15.0; bleopt highlight_syntax=; bleopt highlight_filename=; bleopt highlight_variable=; bleopt complete_auto_complete=; __intent_probe_install_advice() { ble/function#advice around ble/widget/default/accept-line 'ble/function#advice/do'; printf '\nRUNTIME_ADVICE_READY\n'; }; ble-bind -x M-i __intent_probe_install_advice; ble-attach`, shellQuote(blesh))
	tc := shellCase{name: "bash", executable: bash, args: []string{"--noprofile", "--norc", "-i"}}
	shell := startBashWithRCAndTerminalResponses(t, tc, map[string]string{"HOME": home, "TERM": "xterm-256color", "PATH": "/usr/bin:/bin:/usr/sbin:/sbin"}, initialize)
	defer shell.close(t)
	time.Sleep(250 * time.Millisecond)
	shell.writeBytes(t, []byte{'\x1b', 'i'})
	shell.readUntilTimeout(t, "RUNTIME_ADVICE_READY", 30*time.Second)
	shell.readUntilTimeout(t, promptMarker, 30*time.Second)
	shell.write(t, "touch "+shellQuote(marker))
	shell.writeBytes(t, []byte{'\r'})
	waitForPathTimeout(t, marker, 30*time.Second, shell)
}

func TestBleshInitializationRefusesConflictsWithoutPartialBindings(t *testing.T) {
	blesh := requireTestBlesh(t)
	bash := requireBleshMatrixBash(t)
	adapter := filepath.Join(repositoryRoot(t), "shell", "bash", "intent-sh.bash")

	tests := []struct {
		name       string
		prebind    string
		conflict   string
		invoke     []byte
		preserved  string
		acceptLine bool
	}{
		{name: "rewrite", prebind: "ble-bind -x M-g __intent_conflict", conflict: "M-g", invoke: []byte{'\x1b', 'g'}, preserved: "CUSTOM_CONFLICT"},
		{name: "undo", prebind: "ble-bind -x M-u __intent_conflict", conflict: "M-u", invoke: []byte{'\x1b', 'u'}, preserved: "CUSTOM_CONFLICT"},
		{name: "accept-advice", prebind: "ble/function#advice around ble/widget/default/accept-line 'ble/function#advice/do'", conflict: "accept-line", acceptLine: true, preserved: "ACCEPT_CONFLICT_PRESERVED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			probePath := filepath.Join(home, "probe.bash")
			probe := fmt.Sprintf(`__intent_conflict() { printf '\nCUSTOM_CONFLICT\n'; }
%s
source "$INTENT_SH_TEST_ADAPTER"
__intent_probe_status=$?
printf '\nCONFLICT|STATUS=%%s|BACKEND=%%s|READY=%%s|FAILURE=%%s|KEY=%%s|\n' "$__intent_probe_status" "$INTENT_SH_ADAPTER_BACKEND" "$INTENT_SH_ADAPTER_READY" "$INTENT_SH_ADAPTER_FAILURE" "$INTENT_SH_ADAPTER_CONFLICTS"
unset __intent_probe_status
`, test.prebind)
			if err := os.WriteFile(probePath, []byte(probe), 0o600); err != nil {
				t.Fatalf("write conflict probe: %v", err)
			}
			initialize := fmt.Sprintf(`set -o emacs; source %s --attach=none --norc --inputrc=none; bleopt char_width_mode=west; bleopt char_width_version=15.0; bleopt highlight_syntax=; bleopt highlight_filename=; bleopt highlight_variable=; bleopt complete_auto_complete=; ble-attach`, shellQuote(blesh))
			tc := shellCase{name: "bash", executable: bash}
			env := map[string]string{
				"HOME":                         home,
				"INTENT_SH_TEST_ADAPTER":       adapter,
				"INTENT_SH_TEST_INIT_CONFLICT": probePath,
				"PATH":                         "/usr/bin:/bin:/usr/sbin:/sbin",
				"TERM":                         "xterm-256color",
			}
			shell := startBashWithRCAndTerminalResponses(t, tc, env, initialize)
			defer shell.close(t)
			time.Sleep(250 * time.Millisecond)
			shell.write(t, `. "$INTENT_SH_TEST_INIT_CONFLICT"`)
			shell.writeBytes(t, []byte{'\r'})
			want := "CONFLICT|STATUS=1|BACKEND=blesh|READY=0|FAILURE=binding_conflict|KEY=" + test.conflict + "|"
			shell.readUntilTimeout(t, want, 30*time.Second)
			shell.readUntilTimeout(t, promptMarker, 30*time.Second)

			if test.acceptLine {
				shell.write(t, "printf '\\n"+test.preserved+"\\n'")
				shell.writeBytes(t, []byte{'\r'})
			} else {
				shell.writeBytes(t, test.invoke)
			}
			shell.readUntilTimeout(t, test.preserved, 30*time.Second)
		})
	}
}

func TestModernBashNativeAndBleshAcceptanceContract(t *testing.T) {
	modernBash := requireModernBash(t)
	blesh := requireTestBlesh(t)
	adapter := filepath.Join(repositoryRoot(t), "shell", "bash", "intent-sh.bash")

	for _, backend := range []string{"readline", "blesh"} {
		for _, mode := range []string{"emacs", "vi"} {
			t.Run(backend+"/"+mode, func(t *testing.T) {
				tc := shellCase{name: "bash", executable: modernBash}
				env := map[string]string{"HOME": t.TempDir(), "PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "TERM": "xterm-256color"}
				var shell *runningShell
				if backend == "readline" {
					initialize := fmt.Sprintf("set -o %s; source %s", mode, shellQuote(adapter))
					shell = startBashWithRCAndTerminalResponses(t, tc, env, initialize)
				} else {
					initialize := fmt.Sprintf(`set -o %s; source %s --attach=none --norc --inputrc=none; bleopt char_width_mode=west; bleopt char_width_version=15.0; bleopt highlight_syntax=; bleopt highlight_filename=; bleopt highlight_variable=; bleopt complete_auto_complete=; ble-attach`, mode, shellQuote(blesh))
					shell = startBashWithRCAndTerminalResponses(t, tc, env, initialize)
					time.Sleep(250 * time.Millisecond)
					shell.write(t, "source "+shellQuote(adapter))
					shell.writeBytes(t, []byte{'\n'})
					shell.readUntilTimeout(t, promptMarker, 30*time.Second)
				}
				defer shell.close(t)

				shell.write(t, `printf '\nBACKEND|%s|READY=%s|\n' "$INTENT_SH_ADAPTER_BACKEND" "$INTENT_SH_ADAPTER_READY"`)
				shell.writeBytes(t, []byte{'\n'})
				shell.readUntilTimeout(t, "BACKEND|"+backend+"|READY=1|", 30*time.Second)
				shell.readUntilTimeout(t, promptMarker, 30*time.Second)

				acceptance := []struct {
					name string
					key  byte
				}{
					{name: "return", key: '\r'},
					{name: "control-m", key: 0x0d},
					{name: "control-j", key: 0x0a},
				}
				for index, item := range acceptance {
					marker := filepath.Join(t.TempDir(), fmt.Sprintf("accepted-%s-%d", item.name, index))
					shell.write(t, "touch "+shellQuote(marker))
					shell.writeBytes(t, []byte{item.key})
					waitForPathTimeout(t, marker, 30*time.Second, shell)
					shell.readUntilTimeout(t, promptMarker, 30*time.Second)
				}
			})
		}
	}
}

func TestBleshInitializationFailuresLeaveNoPartialIntegration(t *testing.T) {
	blesh := requireTestBlesh(t)
	bash := requireBash32(t)
	adapter := filepath.Join(repositoryRoot(t), "shell", "bash", "intent-sh.bash")
	base := fmt.Sprintf(`set -o emacs; source %s --attach=none --norc --inputrc=none; bleopt char_width_mode=west; bleopt char_width_version=15.0; bleopt highlight_syntax=; bleopt highlight_filename=; bleopt highlight_variable=; bleopt complete_auto_complete=;`, shellQuote(blesh))
	tests := []struct {
		name             string
		initialize       string
		beforeSource     string
		sourceInProbe    bool
		afterSource      string
		failure          string
		backend          string
		attachAfterProbe bool
	}{
		{
			name: "unsupported-version", initialize: base + " ble-attach",
			beforeSource: "BLE_VERSION=0.4.0-unsupported", sourceInProbe: true,
			failure: "incompatible_version", backend: "blesh",
		},
		{
			name: "not-attached", initialize: base,
			sourceInProbe: true, failure: "not_attached", backend: "blesh", attachAfterProbe: true,
		},
		{
			name:       "wrong-load-order",
			initialize: fmt.Sprintf(`set -o emacs; source %s; source %s --attach=none --norc --inputrc=none; bleopt char_width_mode=west; bleopt char_width_version=15.0; bleopt highlight_syntax=; bleopt highlight_filename=; bleopt highlight_variable=; bleopt complete_auto_complete=; ble-attach`, shellQuote(adapter), shellQuote(blesh)),
			failure:    "missing_blesh", backend: "none",
		},
		{
			name: "api-incomplete", initialize: base + " ble-attach",
			beforeSource: `__intent_negative_saved_edit=$(declare -f ble/widget/.EDIT_COMMAND); unset -f ble/widget/.EDIT_COMMAND`, sourceInProbe: true,
			afterSource: `eval "$__intent_negative_saved_edit"; unset __intent_negative_saved_edit`,
			failure:     "missing_api", backend: "blesh",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			fakeDir := t.TempDir()
			providerMarker := filepath.Join(home, "provider-invoked")
			targetMarker := filepath.Join(home, "generated-target-ran")
			fake := `#!/bin/sh
touch "$INTENT_NEG_PROVIDER"
cat >/dev/null
printf '%s\0' 2 ok "touch '$INTENT_NEG_TARGET'" generated fake safe reason stale
`
			if err := os.WriteFile(filepath.Join(fakeDir, "intent-sh"), []byte(fake), 0o755); err != nil {
				t.Fatal(err)
			}

			probePath := filepath.Join(home, "negative-probe.bash")
			sourceLine := "__intent_negative_status=1"
			if test.sourceInProbe {
				sourceLine = `source "$INTENT_NEG_ADAPTER"; __intent_negative_status=$?`
			}
			probe := fmt.Sprintf(`%s
%s
%s
__intent_negative_partial=0
case "$(ble-bind -P -m emacs 2>/dev/null)" in
  *__intent_sh_rewrite*|*__intent_sh_undo*) __intent_negative_partial=1 ;;
esac
if declare -F ble/function#advice/around:ble/widget/default/accept-line >/dev/null 2>&1; then
  __intent_negative_partial=1
fi
printf '\nNEGATIVE|STATUS=%%s|BACKEND=%%s|READY=%%s|FAILURE=%%s|PARTIAL=%%s|\n' "$__intent_negative_status" "$INTENT_SH_ADAPTER_BACKEND" "$INTENT_SH_ADAPTER_READY" "$INTENT_SH_ADAPTER_FAILURE" "$__intent_negative_partial"
__intent_negative_existing() { printf '\nNEG_EXISTING_BINDING\n'; }
__intent_negative_dump() { printf '\nNEG_STATE|BUFFER=%%s|CURSOR=%%s|\n' "$READLINE_LINE" "$READLINE_POINT"; }
ble-bind -x M-g __intent_negative_existing
ble-bind -x M-d __intent_negative_dump
`, test.beforeSource, sourceLine, test.afterSource)
			if err := os.WriteFile(probePath, []byte(probe), 0o600); err != nil {
				t.Fatal(err)
			}

			env := map[string]string{
				"HOME": home, "TERM": "xterm-256color",
				"PATH":               fakeDir + string(os.PathListSeparator) + "/usr/bin:/bin:/usr/sbin:/sbin",
				"INTENT_NEG_ADAPTER": adapter, "INTENT_NEG_PROBE": probePath,
				"INTENT_NEG_PROVIDER": providerMarker, "INTENT_NEG_TARGET": targetMarker,
			}
			tc := shellCase{name: "bash", executable: bash}
			shell := startBashWithRCAndTerminalResponses(t, tc, env, test.initialize)
			defer shell.close(t)
			time.Sleep(250 * time.Millisecond)
			shell.write(t, `. "$INTENT_NEG_PROBE"`)
			shell.writeBytes(t, []byte{'\r'})
			want := "NEGATIVE|STATUS=1|BACKEND=" + test.backend + "|READY=0|FAILURE=" + test.failure + "|PARTIAL=0|"
			shell.readUntilTimeout(t, want, 30*time.Second)
			shell.readUntilTimeout(t, promptMarker, 30*time.Second)

			if test.attachAfterProbe {
				shell.write(t, "ble-attach")
				shell.writeBytes(t, []byte{'\r'})
				shell.readUntilTimeout(t, promptMarker, 30*time.Second)
				time.Sleep(250 * time.Millisecond)
				shell.write(t, "ble-bind -x M-g __intent_negative_existing; ble-bind -x M-d __intent_negative_dump")
				shell.writeBytes(t, []byte{'\n'})
				shell.readUntilTimeout(t, promptMarker, 30*time.Second)
			}

			original := "KEEP_INPUT_7Q"
			shell.write(t, original)
			shell.writeBytes(t, []byte{'\x1b', 'g'})
			shell.readUntilTimeout(t, "NEG_EXISTING_BINDING", 30*time.Second)
			shell.writeBytes(t, []byte{'\x1b', 'd'})
			shell.readUntilTimeout(t, "NEG_STATE|BUFFER="+original+"|CURSOR="+fmt.Sprint(len(original))+"|", 30*time.Second)
			if _, err := os.Stat(providerMarker); !os.IsNotExist(err) {
				t.Fatalf("failed initialization invoked provider: %v", err)
			}
			if _, err := os.Stat(targetMarker); !os.IsNotExist(err) {
				t.Fatalf("failed initialization executed generated target: %v", err)
			}
		})
	}
}

func TestBash32MissingBleshIsInertInPTY(t *testing.T) {
	bash := requireBash32(t)
	adapter := filepath.Join(repositoryRoot(t), "shell", "bash", "intent-sh.bash")
	home := t.TempDir()
	fakeDir := t.TempDir()
	customMarker := filepath.Join(home, "existing-binding-ran")
	providerMarker := filepath.Join(home, "provider-invoked")
	targetMarker := filepath.Join(home, "generated-target-ran")
	fake := `#!/bin/sh
touch "$INTENT_NEG_PROVIDER"
cat >/dev/null
printf '%s\0' 2 ok "touch '$INTENT_NEG_TARGET'" generated fake safe reason stale
`
	if err := os.WriteFile(filepath.Join(fakeDir, "intent-sh"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	initialize := fmt.Sprintf(`__intent_missing_existing() { touch "$INTENT_NEG_CUSTOM"; }; bind -x '"\eg":__intent_missing_existing'; source %s`, shellQuote(adapter))
	tc := shellCase{name: "bash", executable: bash}
	shell := startBashWithRCAndTerminalResponses(t, tc, map[string]string{
		"HOME": home, "TERM": "xterm-256color",
		"PATH":              fakeDir + string(os.PathListSeparator) + "/usr/bin:/bin:/usr/sbin:/sbin",
		"INTENT_NEG_CUSTOM": customMarker, "INTENT_NEG_PROVIDER": providerMarker, "INTENT_NEG_TARGET": targetMarker,
	}, initialize)
	defer shell.close(t)
	shell.write(t, `printf '\nMISSING|BACKEND=%s|READY=%s|FAILURE=%s|\n' "$INTENT_SH_ADAPTER_BACKEND" "$INTENT_SH_ADAPTER_READY" "$INTENT_SH_ADAPTER_FAILURE"`)
	shell.writeBytes(t, []byte{'\r'})
	shell.readUntilTimeout(t, "MISSING|BACKEND=none|READY=0|FAILURE=missing_blesh|", 30*time.Second)
	shell.readUntilTimeout(t, promptMarker, 30*time.Second)

	shell.write(t, "KEEP_INPUT_7Q")
	shell.writeBytes(t, []byte{'\x1b', 'g'})
	waitForPathTimeout(t, customMarker, 30*time.Second, shell)
	if _, err := os.Stat(providerMarker); !os.IsNotExist(err) {
		t.Fatalf("missing ble.sh invoked provider: %v", err)
	}
	if _, err := os.Stat(targetMarker); !os.IsNotExist(err) {
		t.Fatalf("missing ble.sh executed generated target: %v", err)
	}
}

func requireTestBlesh(t *testing.T) string {
	t.Helper()
	path := os.Getenv("INTENT_SH_TEST_BLESH")
	if path == "" {
		qualificationSkipf(t, "set INTENT_SH_TEST_BLESH to the checksum-verified ble.sh script to run the external compatibility matrix")
	}
	if len(path) > 500 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		t.Fatal("INTENT_SH_TEST_BLESH must be one bounded absolute clean path")
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal("inspect INTENT_SH_TEST_BLESH")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > 16<<20 {
		t.Fatal("INTENT_SH_TEST_BLESH must be a bounded regular file")
	}
	return path
}

func requireBash32(t *testing.T) string {
	t.Helper()
	candidates := []string{os.Getenv("INTENT_SH_TEST_BASH32")}
	if candidate := os.Getenv("INTENT_SH_TEST_BASH"); candidate != "" {
		candidates = append(candidates, candidate)
	}
	candidates = append(candidates, "/bin/bash", "bash")
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		output, err := exec.Command(path, "-c", `printf '%s.%s' "${BASH_VERSINFO[0]}" "${BASH_VERSINFO[1]}"`).Output()
		if err == nil && string(output) == "3.2" {
			return path
		}
	}
	qualificationSkipf(t, "the ble.sh compatibility matrix requires Bash 3.2; set INTENT_SH_TEST_BASH32 to its path")
	return ""
}

func requireBleshMatrixBash(t *testing.T) string {
	t.Helper()
	switch os.Getenv("INTENT_SH_TEST_BLESH_BASH_MODE") {
	case "", "bash32":
		return requireBash32(t)
	case "modern":
		return requireModernBash(t)
	default:
		t.Fatal("INTENT_SH_TEST_BLESH_BASH_MODE must be bash32 or modern")
		return ""
	}
}

func requireModernBash(t *testing.T) string {
	t.Helper()
	candidates := []string{os.Getenv("INTENT_SH_TEST_BASH"), "bash", "/opt/homebrew/bin/bash", "/usr/local/bin/bash"}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		path, err := exec.LookPath(candidate)
		if err == nil && bashMajor(t, path) >= 4 {
			return path
		}
	}
	qualificationSkipf(t, "modern Bash acceptance coverage requires Bash 4.0+; set INTENT_SH_TEST_BASH to its path")
	return ""
}
