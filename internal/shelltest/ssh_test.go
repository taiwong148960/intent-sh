package shelltest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/taiwong148960/intent-sh/internal/config"
	"github.com/taiwong148960/intent-sh/internal/provider"
	"github.com/taiwong148960/intent-sh/internal/textsafe"
)

const (
	sshRemoteMarker      = "INTENT_SSH_MARKER_SECRET_7Q"
	sshClientTermMarker  = "INTENT_CLIENT_TERM_SECRET_7Q"
	sshCredentialMarker  = "INTENT_LOCAL_CREDENTIAL_SECRET_7Q"
	sshTerminalOnlyValue = "INTENT_TERMINAL_SCREEN_ONLY_SECRET_7Q"
)

var (
	sshTargetPattern = regexp.MustCompile(`^[A-Za-z0-9_.@:%+\[\]-]{1,255}$`)
	sshPathPattern   = regexp.MustCompile(`^/[A-Za-z0-9_./+@%-]{1,500}$`)
)

// sshSmokeHarness uses only an explicitly configured target and the caller's
// existing non-interactive SSH authentication. It stages an ephemeral test
// bundle, installs no package or credential, and removes the remote directory
// on completion.
type sshSmokeHarness struct {
	path       string
	target     string
	options    []string
	remoteRoot string
}

func newSSHSmokeHarness(t *testing.T) *sshSmokeHarness {
	t.Helper()
	target := strings.TrimSpace(os.Getenv("INTENT_SH_TEST_SSH_TARGET"))
	if target == "" {
		t.Skip("INTENT_SH_TEST_SSH_TARGET is not set; opt-in SSH smoke test skipped")
	}
	if !safeSSHTarget(target) {
		t.Fatalf("INTENT_SH_TEST_SSH_TARGET must be one bounded host or user@host token")
	}
	path, err := exec.LookPath("ssh")
	if err != nil {
		t.Fatal("ssh is required when INTENT_SH_TEST_SSH_TARGET is set")
	}
	harness := &sshSmokeHarness{
		path:    path,
		target:  target,
		options: sshSafetyOptions(),
	}
	remoteRoot := strings.TrimSpace(harness.output(t, `umask 077; base=${TMPDIR:-/tmp}; base=${base%/}; d=$(mktemp -d "$base/intent-sh-ssh.XXXXXX") || exit 1; printf '%s\n' "$d"`))
	if !safeRemoteTempPath(remoteRoot) {
		t.Fatalf("remote mktemp returned an unsafe path")
	}
	harness.remoteRoot = remoteRoot
	t.Cleanup(func() {
		cmd := harness.command(false, "rm -rf -- "+shellQuote(remoteRoot))
		_ = cmd.Run()
	})
	return harness
}

func safeSSHTarget(value string) bool {
	return sshTargetPattern.MatchString(value) && !strings.HasPrefix(value, "-")
}

func sshSafetyOptions() []string {
	return []string{
		"-o", "BatchMode=yes",
		"-o", "NumberOfPasswordPrompts=0",
		"-o", "StrictHostKeyChecking=yes",
		"-o", "UpdateHostKeys=no",
		"-o", "ControlMaster=no",
		"-o", "ClearAllForwardings=yes",
		"-o", "ForwardAgent=no",
		"-o", "ForwardX11=no",
		"-o", "PermitLocalCommand=no",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=10",
	}
}

func safeRemoteTempPath(value string) bool {
	if !sshPathPattern.MatchString(value) || path.Clean(value) != value || value == "/" {
		return false
	}
	base := path.Base(value)
	if !strings.HasPrefix(base, "intent-sh-ssh.") || len(strings.TrimPrefix(base, "intent-sh-ssh.")) < 6 {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "." || component == ".." {
			return false
		}
	}
	return true
}

func (harness *sshSmokeHarness) command(tty bool, remoteCommand string) *exec.Cmd {
	args := append([]string(nil), harness.options...)
	if tty {
		args = append(args, "-tt")
	} else {
		args = append(args, "-T")
	}
	args = append(args, harness.target, remoteCommand)
	return exec.Command(harness.path, args...)
}

func (harness *sshSmokeHarness) output(t *testing.T, remoteCommand string) string {
	t.Helper()
	cmd := harness.command(false, remoteCommand)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("run opt-in SSH command: %v: %s", err, harness.boundedError(stderr.String()))
	}
	if len(output) > 4096 {
		t.Fatal("opt-in SSH command returned excessive output")
	}
	return string(output)
}

func (harness *sshSmokeHarness) boundedError(value string) string {
	value = strings.ReplaceAll(value, harness.target, "<target>")
	if harness.remoteRoot != "" {
		value = strings.ReplaceAll(value, harness.remoteRoot, "<remote-temp>")
	}
	return boundedTestOutput(value)
}

func boundedTestOutput(value string) string {
	return textsafe.Terminal(strings.TrimSpace(value), 512)
}

func (harness *sshSmokeHarness) succeeds(t *testing.T, remoteCommand string) bool {
	t.Helper()
	cmd := harness.command(false, remoteCommand)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false
	}
	if stderr.Len() > 0 {
		t.Fatalf("run opt-in SSH assertion: %v: %s", err, harness.boundedError(stderr.String()))
	}
	t.Fatalf("run opt-in SSH assertion: %v", err)
	return false
}

func (harness *sshSmokeHarness) pathExists(t *testing.T, remotePath string) bool {
	t.Helper()
	if !sshPathPattern.MatchString(remotePath) || path.Clean(remotePath) != remotePath || !strings.HasPrefix(remotePath, harness.remoteRoot+"/") {
		t.Fatal("refusing to inspect a path outside the SSH smoke directory")
	}
	return harness.succeeds(t, "test -e "+shellQuote(remotePath))
}

func (harness *sshSmokeHarness) waitForPath(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !harness.pathExists(t, path) {
		if time.Now().After(deadline) {
			t.Fatalf("remote marker was not created: %s", filepath.Base(path))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (harness *sshSmokeHarness) remotePlatform(t *testing.T) (string, string) {
	t.Helper()
	fields := strings.Fields(harness.output(t, `uname -s; uname -m`))
	if len(fields) != 2 {
		t.Fatal("remote uname output was not recognized")
	}
	goos := map[string]string{"Darwin": "darwin", "Linux": "linux"}[fields[0]]
	goarch := map[string]string{"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}[fields[1]]
	if goos == "" || goarch == "" {
		t.Fatalf("remote platform %s/%s is outside the supported SSH smoke boundary", fields[0], fields[1])
	}
	return goos, goarch
}

func (harness *sshSmokeHarness) stage(t *testing.T, root string) {
	t.Helper()
	goos, goarch := harness.remotePlatform(t)
	bundle := t.TempDir()
	binDir := filepath.Join(bundle, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatalf("create SSH bundle: %v", err)
	}
	build := exec.Command("go", "build", "-trimpath", "-o", filepath.Join(binDir, "intent-sh"), "./cmd/intent-sh")
	build.Dir = root
	build.Env = replaceEnvironment(os.Environ(), map[string]string{"CGO_ENABLED": "0", "GOOS": goos, "GOARCH": goarch})
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build remote intent-sh test binary: %v: %s", err, output)
	}
	for name, script := range map[string]string{"codex": fakeCodexScript, "claude": fakeClaudeScript} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o700); err != nil {
			t.Fatalf("write remote fake provider %s: %v", name, err)
		}
	}
	cfg := config.Defaults()
	cfg.Provider = config.ProviderAuto
	cfg.Priority = []string{provider.NameClaude, provider.NameCodex}
	cfg.TimeoutSeconds = 5
	if err := config.WriteAt(filepath.Join(bundle, "home", "xdg", "intent-sh", "config.toml"), cfg); err != nil {
		t.Fatalf("write remote smoke config: %v", err)
	}

	archive, err := exec.Command("tar", "-C", bundle, "-cf", "-", ".").Output()
	if err != nil {
		t.Fatalf("archive SSH smoke bundle: %v", err)
	}
	upload := harness.command(false, "tar -xf - -C "+shellQuote(harness.remoteRoot))
	upload.Stdin = bytes.NewReader(archive)
	var stderr bytes.Buffer
	upload.Stderr = &stderr
	if err := upload.Run(); err != nil {
		t.Fatalf("stage ephemeral SSH smoke bundle: %v: %s", err, harness.boundedError(stderr.String()))
	}
}

func (harness *sshSmokeHarness) shellPath(t *testing.T, name string) string {
	t.Helper()
	path := strings.TrimSpace(harness.output(t, "command -v "+name+" 2>/dev/null || true"))
	if path == "" {
		t.Skipf("remote target does not provide %s", name)
	}
	if !sshPathPattern.MatchString(path) {
		t.Fatalf("remote %s path was unsafe", name)
	}
	if name == "bash" {
		version := strings.TrimSpace(harness.output(t, shellQuote(path)+` --noprofile --norc -c 'printf "%s" "${BASH_VERSINFO[0]}"'`))
		major, err := strconv.Atoi(version)
		if err != nil {
			t.Fatalf("remote Bash reported an invalid major version")
		}
		if major < 4 {
			t.Skipf("remote Bash %s is below the native Readline boundary", version)
		}
	}
	return path
}

func TestSSHHarnessRejectsUnsafeCleanupPathsAndDisablesForwarding(t *testing.T) {
	for _, valid := range []string{"host", "user@example.invalid", "alias-name", "[2001:db8::1]"} {
		if !safeSSHTarget(valid) {
			t.Fatalf("safeSSHTarget(%q) = false", valid)
		}
	}
	for _, invalid := range []string{"", "-oProxyCommand=bad", "host command", "host\ncommand", "user@host;command", "user@host/path"} {
		if safeSSHTarget(invalid) {
			t.Fatalf("safeSSHTarget(%q) = true", invalid)
		}
	}
	for _, valid := range []string{"/tmp/intent-sh-ssh.abc123", "/var/folders/a/T/intent-sh-ssh.A1b2C3"} {
		if !safeRemoteTempPath(valid) {
			t.Fatalf("safeRemoteTempPath(%q) = false", valid)
		}
	}
	for _, invalid := range []string{
		"", "/", "relative/intent-sh-ssh.abc123", "/tmp/not-intent.abc123",
		"/tmp/intent-sh-ssh.x/../../home", "/tmp/./intent-sh-ssh.abc123", "/tmp/intent-sh-ssh.x",
		"/tmp/intent-sh-ssh.abc123;rm",
	} {
		if safeRemoteTempPath(invalid) {
			t.Fatalf("safeRemoteTempPath(%q) = true", invalid)
		}
	}
	target := strings.TrimSpace(os.Getenv("INTENT_SH_TEST_SSH_TARGET"))
	if target == "" {
		target = "example.invalid"
	}
	harness := &sshSmokeHarness{target: target, options: sshSafetyOptions()}
	joined := strings.Join(harness.options, " ")
	for _, required := range []string{"BatchMode=yes", "ClearAllForwardings=yes", "ForwardAgent=no", "ForwardX11=no", "PermitLocalCommand=no"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("SSH harness options omitted %s", required)
		}
	}
}

func (harness *sshSmokeHarness) startShell(t *testing.T, name, shellPath string) *runningShell {
	t.Helper()
	remoteHome := harness.remoteRoot + "/home"
	environment := map[string]string{
		"DATABASE_URL":            sshCredentialMarker,
		"HOME":                    remoteHome,
		"INTENT_PRIVATE_SENTINEL": sshCredentialMarker,
		"LANG":                    "C.UTF-8",
		"PATH":                    harness.remoteRoot + "/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin",
		"PROMPT":                  promptMarker,
		"PS1":                     promptMarker,
		"SHELL":                   shellPath,
		"SSH_CONNECTION":          sshRemoteMarker,
		"TERM":                    "xterm-256color",
		"TERM_PROGRAM":            sshClientTermMarker,
		"XDG_CONFIG_HOME":         remoteHome + "/xdg",
	}
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sortStrings(keys)
	parts := []string{"cd", shellQuote(harness.remoteRoot), "&&", "exec", "env", "-i"}
	for _, key := range keys {
		parts = append(parts, key+"="+shellQuote(environment[key]))
	}
	parts = append(parts, shellQuote(shellPath))
	if name == "bash" {
		parts = append(parts, "--noprofile", "--norc", "-i")
	} else {
		parts = append(parts, "-f", "-i")
	}
	remoteCommand := strings.Join(parts, " ")
	args := append([]string(nil), harness.options...)
	args = append(args, "-tt", harness.target, remoteCommand)
	clientCase := shellCase{name: name, executable: harness.path, args: args}
	client := startShellWithPTYOptions(t, clientCase, nil, `eval "$(intent-sh init `+name+`)"`, terminalPTYOptions{
		term: "xterm-256color", rows: 36, cols: 120, respondTerminalQueries: true,
	})
	configureStateDump(t, client)
	return client
}

func sortStrings(values []string) {
	for index := 1; index < len(values); index++ {
		for current := index; current > 0 && values[current] < values[current-1]; current-- {
			values[current], values[current-1] = values[current-1], values[current]
		}
	}
}

func (harness *sshSmokeHarness) assertPromptPrivacy(t *testing.T, promptPath string) {
	t.Helper()
	if !harness.succeeds(t, "grep -F -q "+shellQuote(`"remote":true`)+" "+shellQuote(promptPath)) {
		t.Fatal("remote provider input omitted the boolean remote context")
	}
	for _, forbidden := range []string{sshRemoteMarker, sshClientTermMarker, sshCredentialMarker, sshTerminalOnlyValue} {
		if harness.succeeds(t, "grep -F -q "+shellQuote(forbidden)+" "+shellQuote(promptPath)) {
			t.Fatalf("remote provider input included prohibited marker %s", forbidden)
		}
	}
	if harness.pathExists(t, harness.remoteRoot+"/home/claude-env-leaked") {
		t.Fatal("SSH or credential marker reached the remote provider process environment")
	}
}

func (harness *sshSmokeHarness) assertProviderTreeStopped(t *testing.T) {
	t.Helper()
	pidPath := harness.remoteRoot + "/home/claude-provider-pid"
	if !harness.pathExists(t, pidPath) {
		t.Fatal("remote cancellation did not record the fake provider process tree")
	}
	command := "for pid in $(cat " + shellQuote(pidPath) + "); do kill -0 \"$pid\" 2>/dev/null && exit 1; done; exit 0"
	if !harness.succeeds(t, command) {
		t.Fatal("remote provider process tree survived Ctrl+C cancellation")
	}
}

func TestSSHRemoteBashAndZshConformance(t *testing.T) {
	root := repositoryRoot(t)
	harness := newSSHSmokeHarness(t)
	harness.stage(t, root)

	for _, name := range []string{"zsh", "bash"} {
		t.Run(name, func(t *testing.T) {
			shellPath := harness.shellPath(t, name)
			client := harness.startShell(t, name, shellPath)
			defer client.close(t)
			matrix := newTerminalConformanceCase(t, shellCase{name: name}, "alt+g", "alt+u", "xterm-256color", []byte{'\r'}, 36, 120)
			remoteHome := harness.remoteRoot + "/home"

			client.write(t, `rm -f "$HOME"/claude-invoked "$HOME"/codex-invoked "$HOME"/claude-last-prompt "$HOME"/codex-last-prompt "$HOME"/claude-env-leaked "$HOME"/claude-provider-pid "$HOME"/review-ran "$HOME"/danger-ran`)
			client.writeBytes(t, matrix.enter)
			client.readUntilTimeout(t, promptMarker, 10*time.Second)
			client.write(t, "intent-sh config set provider auto >/dev/null")
			client.writeBytes(t, matrix.enter)
			client.readUntilTimeout(t, promptMarker, 10*time.Second)
			client.write(t, "printf '\\n"+sshTerminalOnlyValue+"\\n'")
			client.writeBytes(t, matrix.enter)
			client.readUntilTimeout(t, sshTerminalOnlyValue, 10*time.Second)
			client.readUntilTimeout(t, promptMarker, 10*time.Second)

			original := "REMOTE-" + name + "-INTENT_CASE_CLAUDE_SAFE_7Q"
			client.write(t, original)
			client.writeBytes(t, matrix.rewriteBytes)
			client.readUntilTimeout(t, "Claude generated one", 30*time.Second)
			assertShellState(t, client, "printf CLAUDE_ONE", len("printf CLAUDE_ONE"), original, 0, "safe")
			client.writeBytes(t, matrix.rewriteBytes)
			client.readUntilTimeout(t, "Claude generated two", 30*time.Second)
			assertShellState(t, client, "printf CLAUDE_TWO", len("printf CLAUDE_TWO"), original, 1, "safe")
			client.writeBytes(t, matrix.undoBytes)
			client.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
			assertShellState(t, client, original, len(original), "", 0, "")
			clearEditableLine(t, client)

			cancelled := "REMOTE-" + name + "-INTENT_CASE_CLAUDE_SLOW_7Q"
			client.write(t, cancelled)
			client.writeBytes(t, matrix.rewriteBytes)
			client.readUntilTimeout(t, "Ctrl+C to cancel", 10*time.Second)
			harness.waitForPath(t, remoteHome+"/claude-provider-pid", 10*time.Second)
			client.writeBytes(t, []byte{0x03})
			client.readUntilTimeout(t, "cancelled", 10*time.Second)
			assertShellState(t, client, cancelled, len(cancelled), "", 0, "")
			harness.assertProviderTreeStopped(t)
			if harness.pathExists(t, remoteHome+"/codex-invoked") {
				t.Fatal("remote cancellation invoked fallback or a client-side provider path")
			}
			harness.assertPromptPrivacy(t, remoteHome+"/claude-last-prompt")
			clearEditableLine(t, client)

			client.write(t, "intent-sh config set provider codex >/dev/null")
			client.writeBytes(t, matrix.enter)
			client.readUntilTimeout(t, promptMarker, 10*time.Second)

			reviewMarker := remoteHome + "/review-ran"
			client.write(t, "INTENT_CASE_REVIEW_7Q")
			client.writeBytes(t, matrix.rewriteBytes)
			client.readUntilTimeout(t, "REVIEW:", 30*time.Second)
			if harness.pathExists(t, reviewMarker) {
				t.Fatal("remote review command ran automatically")
			}
			client.writeBytes(t, matrix.undoBytes)
			client.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
			clearEditableLine(t, client)

			dangerMarker := remoteHome + "/danger-ran"
			client.write(t, "INTENT_CASE_DANGER_7Q")
			client.writeBytes(t, matrix.rewriteBytes)
			client.readUntilTimeout(t, "DANGEROUS:", 30*time.Second)
			if harness.pathExists(t, dangerMarker) {
				t.Fatal("remote dangerous command ran during generation")
			}
			client.writeBytes(t, matrix.enter)
			client.readUntilTimeout(t, "Press Enter again to execute.", 10*time.Second)
			if harness.pathExists(t, dangerMarker) {
				t.Fatal("remote dangerous command ran on the first Enter")
			}
			client.writeBytes(t, matrix.enter)
			harness.waitForPath(t, dangerMarker, 10*time.Second)
			if !harness.pathExists(t, remoteHome+"/codex-invoked") {
				t.Fatal("fake provider did not execute on the remote host")
			}
		})
	}
}

func TestSSHSmokeHarnessSkipsWithoutExplicitTarget(t *testing.T) {
	if os.Getenv("INTENT_SH_TEST_SSH_TARGET") != "" {
		t.Skip("explicit target is configured")
	}
	// The actual integration test above calls t.Skip before looking up ssh,
	// creating a remote directory, building a binary, or starting a provider.
	t.Log("ordinary tests require no SSH daemon, target, credential, or remote mutation")
}

func TestSSHMarkerValuesRemainOutsideLocalProviderBoundaries(t *testing.T) {
	for _, value := range []string{sshRemoteMarker, sshClientTermMarker, sshCredentialMarker, sshTerminalOnlyValue} {
		if strings.Contains(fakeCodexScript, value) || strings.Contains(fakeClaudeScript, value) {
			t.Fatalf("fixed SSH privacy marker %q was embedded in a provider fixture", value)
		}
	}
	if strings.Contains(fmt.Sprint(config.Defaults()), sshRemoteMarker) {
		t.Fatal("SSH marker entered default configuration")
	}
}
