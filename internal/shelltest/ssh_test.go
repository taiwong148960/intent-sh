package shelltest

import (
	"bytes"
	"errors"
	"fmt"
	"net"
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
	sshUserPattern   = regexp.MustCompile(`^[A-Za-z0-9_.+-]{1,64}$`)
	sshHostPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,252}$`)
	sshZonePattern   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,32}$`)
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
	coverage   bool
}

func newSSHSmokeHarness(t *testing.T) *sshSmokeHarness {
	t.Helper()
	target := strings.TrimSpace(os.Getenv("INTENT_SH_TEST_SSH_TARGET"))
	if target == "" {
		qualificationSkipf(t, "INTENT_SH_TEST_SSH_TARGET is not set; opt-in SSH smoke test skipped")
	}
	if !safeSSHTarget(target) {
		t.Fatalf("INTENT_SH_TEST_SSH_TARGET must be one bounded host or user@host token")
	}
	path, err := exec.LookPath("ssh")
	if err != nil {
		t.Fatal("ssh is required when INTENT_SH_TEST_SSH_TARGET is set")
	}
	configPath := strings.TrimSpace(os.Getenv("INTENT_SH_TEST_SSH_CONFIG"))
	loopback := os.Getenv("INTENT_SH_TEST_SSH_LOOPBACK") == "1"
	if err := validateSSHConfigPath(configPath, os.Getenv("RUNNER_TEMP"), loopback); err != nil {
		t.Fatalf("validate INTENT_SH_TEST_SSH_CONFIG: %v", err)
	}
	options := sshSafetyOptions()
	if configPath != "" {
		options = append([]string{"-F", configPath}, options...)
	}
	harness := &sshSmokeHarness{
		path:    path,
		target:  target,
		options: options,
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

func validateSSHConfigPath(value, runnerTemp string, requireLoopback bool) error {
	if value == "" {
		if requireLoopback {
			return errors.New("the loopback fixture requires its generated client configuration")
		}
		return nil
	}
	if !sshPathPattern.MatchString(value) || filepath.Clean(value) != value {
		return errors.New("path must be bounded, absolute, and clean")
	}
	info, err := os.Lstat(value)
	if err != nil {
		return fmt.Errorf("inspect path: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() == 0 || info.Size() > 16<<10 {
		return errors.New("path must name a private, bounded regular file")
	}
	if requireLoopback {
		if !sshPathPattern.MatchString(runnerTemp) || filepath.Clean(runnerTemp) != runnerTemp || runnerTemp == "/" {
			return errors.New("RUNNER_TEMP is outside the loopback fixture boundary")
		}
		expected := filepath.Join(runnerTemp, "intent-sh-loopback-ssh", "client_config")
		if value != expected {
			return errors.New("loopback client configuration is outside the job-owned state directory")
		}
	}
	return nil
}

func safeSSHTarget(value string) bool {
	if !sshTargetPattern.MatchString(value) || strings.HasPrefix(value, "-") || strings.Count(value, "@") > 1 {
		return false
	}
	user, host, hasUser := strings.Cut(value, "@")
	if !hasUser {
		host = user
	} else if !sshUserPattern.MatchString(user) || host == "" {
		return false
	}
	if strings.Contains(host, ":") {
		if strings.HasPrefix(host, "[") != strings.HasSuffix(host, "]") {
			return false
		}
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
		address, zone, hasZone := strings.Cut(host, "%")
		if hasZone && !sshZonePattern.MatchString(zone) {
			return false
		}
		return net.ParseIP(address) != nil && strings.Contains(address, ":")
	}
	return sshHostPattern.MatchString(host)
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
	intentBinary := filepath.Join(binDir, "intent-sh")
	if prebuilt := os.Getenv("INTENT_SH_TEST_BINARY"); prebuilt != "" {
		copyPrebuiltBinary(t, prebuilt, intentBinary)
	} else {
		build := exec.Command("go", "build", "-trimpath", "-o", intentBinary, "./cmd/intent-sh")
		build.Dir = root
		build.Env = replaceEnvironment(os.Environ(), map[string]string{"CGO_ENABLED": "0", "GOOS": goos, "GOARCH": goarch})
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build remote intent-sh test binary: %v: %s", err, output)
		}
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
	if err := os.WriteFile(filepath.Join(bundle, "empty-tmux.conf"), nil, 0o600); err != nil {
		t.Fatalf("write remote empty tmux configuration: %v", err)
	}
	coverageDirectory := qualificationCoverageDirectory(t)
	if coverageDirectory != "" {
		if err := os.Mkdir(filepath.Join(bundle, "coverage"), 0o700); err != nil {
			t.Fatalf("create remote executable coverage directory: %v", err)
		}
		harness.coverage = true
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
	if coverageDirectory != "" {
		t.Cleanup(func() { harness.downloadCoverage(t, coverageDirectory) })
	}
}

func qualificationCoverageDirectory(t *testing.T) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv("INTENT_SH_COVERAGE_DIR"))
	if value == "" {
		return ""
	}
	if os.Getenv("INTENT_SH_TEST_SSH_LOOPBACK") != "1" {
		t.Fatal("remote executable coverage is restricted to the job-owned loopback SSH fixture")
	}
	if !sshPathPattern.MatchString(value) || filepath.Clean(value) != value {
		t.Fatal("INTENT_SH_COVERAGE_DIR must be one bounded absolute path")
	}
	info, err := os.Lstat(value)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("INTENT_SH_COVERAGE_DIR must be a real existing directory")
	}
	return value
}

func (harness *sshSmokeHarness) downloadCoverage(t *testing.T, localDirectory string) {
	t.Helper()
	remoteDirectory := harness.remoteRoot + "/coverage"
	unsafeEntry := strings.TrimSpace(harness.output(t, "find "+shellQuote(remoteDirectory)+" -mindepth 1 -maxdepth 1 ! -type f -print -quit"))
	if unsafeEntry != "" {
		t.Fatal("remote executable coverage contained a non-regular entry")
	}
	listing := strings.Fields(harness.output(t, "find "+shellQuote(remoteDirectory)+" -mindepth 1 -maxdepth 1 -type f -exec basename {} \\;"))
	if len(listing) == 0 || len(listing) > 1000 {
		t.Fatal("remote executable coverage file count is outside its bound")
	}
	namePattern := regexp.MustCompile(`^cov(?:meta|counters)\.[a-f0-9.]{16,200}$`)
	parts := []string{"cd", shellQuote(remoteDirectory), "&&", "tar", "-cf", "-", "--"}
	for _, name := range listing {
		if !namePattern.MatchString(name) {
			t.Fatal("remote executable coverage returned an unsafe filename")
		}
		parts = append(parts, shellQuote(name))
	}
	remote := harness.command(false, strings.Join(parts, " "))
	local := exec.Command("tar", "-xf", "-", "-C", localDirectory)
	pipe, err := remote.StdoutPipe()
	if err != nil {
		t.Fatal("prepare remote coverage transfer")
	}
	local.Stdin = pipe
	var remoteError, localError bytes.Buffer
	remote.Stderr = &remoteError
	local.Stderr = &localError
	if err := local.Start(); err != nil {
		t.Fatal("start local coverage extraction")
	}
	if err := remote.Start(); err != nil {
		_ = local.Process.Kill()
		_ = local.Wait()
		t.Fatal("start remote coverage transfer")
	}
	remoteRunErr := remote.Wait()
	localRunErr := local.Wait()
	if remoteRunErr != nil || localRunErr != nil {
		t.Fatalf("transfer bounded remote executable coverage: remote=%s local=%s", harness.boundedError(remoteError.String()), boundedTestOutput(localError.String()))
	}
}

func (harness *sshSmokeHarness) executablePath(t *testing.T, name string) string {
	t.Helper()
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`).MatchString(name) {
		t.Fatal("remote executable name was unsafe")
	}
	value := strings.TrimSpace(harness.output(t, "command -v "+name+" 2>/dev/null || true"))
	if value == "" {
		qualificationSkipf(t, "remote target does not provide %s", name)
	}
	if !sshPathPattern.MatchString(value) || path.Clean(value) != value {
		t.Fatalf("remote %s path was unsafe", name)
	}
	return value
}

func (harness *sshSmokeHarness) shellPath(t *testing.T, name string) string {
	t.Helper()
	path := harness.executablePath(t, name)
	if name == "bash" {
		version := strings.TrimSpace(harness.output(t, shellQuote(path)+` --noprofile --norc -c 'printf "%s" "${BASH_VERSINFO[0]}"'`))
		major, err := strconv.Atoi(version)
		if err != nil {
			t.Fatalf("remote Bash reported an invalid major version")
		}
		if major < 4 {
			qualificationSkipf(t, "remote Bash %s is below the native Readline boundary", version)
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
	for _, invalid := range []string{
		"", "-oProxyCommand=bad", "host command", "host\ncommand", "user@host;command", "user@host/path",
		"@host", "user@", "user@@host", "host:2222", "[2001:db8::1", "2001:db8::1]",
	} {
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
	runnerTemp := t.TempDir()
	configDirectory := filepath.Join(runnerTemp, "intent-sh-loopback-ssh")
	if err := os.Mkdir(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDirectory, "client_config")
	if err := os.WriteFile(configPath, []byte("Host intent-sh-loopback\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateSSHConfigPath(configPath, runnerTemp, true); err != nil {
		t.Fatalf("valid loopback config was rejected: %v", err)
	}
	for _, invalid := range []string{"relative", configPath + "/child", filepath.Join(runnerTemp, "client_config")} {
		if err := validateSSHConfigPath(invalid, runnerTemp, true); err == nil {
			t.Fatalf("unsafe loopback config %q was accepted", invalid)
		}
	}
	worldReadable := filepath.Join(configDirectory, "world-readable")
	if err := os.WriteFile(worldReadable, []byte("Host bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateSSHConfigPath(worldReadable, runnerTemp, false); err == nil {
		t.Fatal("world-readable SSH config was accepted")
	}
	symlinkPath := filepath.Join(configDirectory, "symlink")
	if err := os.Symlink(configPath, symlinkPath); err != nil {
		t.Fatal(err)
	}
	if err := validateSSHConfigPath(symlinkPath, runnerTemp, false); err == nil {
		t.Fatal("symlink SSH config was accepted")
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

func (harness *sshSmokeHarness) remoteShellEnvironment(shellPath string) map[string]string {
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
	if harness.coverage {
		environment["GOCOVERDIR"] = harness.remoteRoot + "/coverage"
	}
	return environment
}

func (harness *sshSmokeHarness) remoteCleanShellCommand(environment map[string]string, name, shellPath string) string {
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
	return strings.Join(parts, " ")
}

func (harness *sshSmokeHarness) startPTYCommand(t *testing.T, name, remoteCommand, initialize string) *runningShell {
	t.Helper()
	args := append([]string(nil), harness.options...)
	args = append(args, "-tt", harness.target, remoteCommand)
	clientCase := shellCase{name: name, executable: harness.path, args: args}
	client := startShellWithPTYOptions(t, clientCase, nil, initialize, terminalPTYOptions{
		term: "xterm-256color", rows: 36, cols: 120, respondTerminalQueries: true,
	})
	return client
}

func (harness *sshSmokeHarness) startShell(t *testing.T, name, shellPath string) *runningShell {
	t.Helper()
	environment := harness.remoteShellEnvironment(shellPath)
	remoteCommand := harness.remoteCleanShellCommand(environment, name, shellPath)
	client := harness.startPTYCommand(t, name, remoteCommand, `eval "$(intent-sh init `+name+`)"`)
	configureStateDump(t, client)
	return client
}

func (harness *sshSmokeHarness) tmuxCommand(t *testing.T, tmuxPath string, arguments ...string) string {
	t.Helper()
	socketPath := harness.remoteRoot + "/tmux.socket"
	configPath := harness.remoteRoot + "/empty-tmux.conf"
	for _, value := range []string{tmuxPath, socketPath, configPath} {
		if !sshPathPattern.MatchString(value) || path.Clean(value) != value {
			t.Fatal("remote tmux command contained an unsafe path")
		}
	}
	parts := []string{shellQuote(tmuxPath), "-S", shellQuote(socketPath), "-f", shellQuote(configPath)}
	for _, argument := range arguments {
		if len(argument) == 0 || len(argument) > 512 || strings.ContainsAny(argument, "\x00\r\n") {
			t.Fatal("remote tmux command contained an unsafe argument")
		}
		parts = append(parts, shellQuote(argument))
	}
	return strings.Join(parts, " ")
}

func (harness *sshSmokeHarness) runTmux(t *testing.T, tmuxPath string, arguments ...string) string {
	t.Helper()
	return strings.TrimSpace(harness.output(t, harness.tmuxCommand(t, tmuxPath, arguments...)))
}

func (harness *sshSmokeHarness) startTmuxSession(t *testing.T, name, shellPath, tmuxPath, session string) *runningShell {
	t.Helper()
	environment := harness.remoteShellEnvironment(shellPath)
	innerShell := harness.remoteCleanShellCommand(environment, name, shellPath)
	remoteCommand := "cd " + shellQuote(harness.remoteRoot) + " && exec " + harness.tmuxCommand(t, tmuxPath,
		"new-session", "-s", session, "-x", "120", "-y", "36", innerShell)
	return harness.startPTYCommand(t, name, remoteCommand, `eval "$(intent-sh init `+name+`)"`)
}

func (harness *sshSmokeHarness) attachTmuxSession(t *testing.T, name, tmuxPath, session string) *runningShell {
	t.Helper()
	remoteCommand := "cd " + shellQuote(harness.remoteRoot) + " && exec " + harness.tmuxCommand(t, tmuxPath, "attach-session", "-t", session)
	return harness.startPTYCommand(t, name, remoteCommand, "")
}

func waitForPTYClientExit(t *testing.T, client *runningShell, timeout time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- client.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(timeout):
		_ = client.cmd.Process.Kill()
		<-done
	}
	_ = client.file.Close()
}

func abruptlyDisconnectPTYClient(t *testing.T, client *runningShell) {
	t.Helper()
	_ = client.file.Close()
	waitForPTYClientExit(t, client, 5*time.Second)
}

func runSSHPhase(t *testing.T, name string, phase func(*testing.T)) {
	t.Helper()
	if !t.Run(name, phase) {
		t.FailNow()
	}
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
	deadline := time.Now().Add(10 * time.Second)
	for !harness.succeeds(t, command) {
		if time.Now().After(deadline) {
			t.Fatal("remote provider process tree survived cancellation or disconnect")
		}
		time.Sleep(50 * time.Millisecond)
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

func TestSSHDirectDisconnectReapsRemoteProvider(t *testing.T) {
	root := repositoryRoot(t)
	harness := newSSHSmokeHarness(t)
	harness.stage(t, root)
	shellPath := harness.shellPath(t, "bash")
	remoteHome := harness.remoteRoot + "/home"
	var client *runningShell
	runSSHPhase(t, "start-slow-provider", func(t *testing.T) {
		client = harness.startShell(t, "bash", shellPath)
		client.write(t, `rm -f "$HOME"/claude-invoked "$HOME"/codex-invoked "$HOME"/claude-provider-pid "$HOME"/claude-provider-phase "$HOME"/danger-ran`)
		client.writeBytes(t, []byte{'\r'})
		client.readUntilTimeout(t, promptMarker, 10*time.Second)
		client.write(t, "REMOTE-DISCONNECT-INTENT_CASE_CLAUDE_SLOW_7Q")
		client.writeBytes(t, []byte{0x1b, 'g'})
		client.readUntilTimeout(t, "Ctrl+C to cancel", 10*time.Second)
		harness.waitForPath(t, remoteHome+"/claude-provider-pid", 10*time.Second)
	})
	runSSHPhase(t, "disconnect-client", func(t *testing.T) {
		abruptlyDisconnectPTYClient(t, client)
	})
	runSSHPhase(t, "verify-remote-reap", func(t *testing.T) {
		harness.assertProviderTreeStopped(t)
		phasePath := remoteHome + "/claude-provider-phase"
		phases := harness.output(t, "cat "+shellQuote(phasePath))
		// A transport loss may terminate the remote session before the fake
		// provider can append its graceful-exit marker. The durable contract is
		// that work started, never completed, and no executable descendant lives.
		if strings.Count(phases, "phase=started") != 1 ||
			strings.Contains(phases, "phase=completed") ||
			strings.Count(phases, "phase=cancel-signal") > 1 {
			t.Fatal("remote disconnect recorded an invalid bounded cancellation lifecycle")
		}
		for _, forbidden := range []string{remoteHome + "/codex-invoked", remoteHome + "/danger-ran"} {
			if harness.pathExists(t, forbidden) {
				t.Fatalf("disconnect produced prohibited fallback or target side effect: %s", filepath.Base(forbidden))
			}
		}
	})
}

func TestSSHToTmuxReconnectStateAndPaneIsolation(t *testing.T) {
	root := repositoryRoot(t)
	harness := newSSHSmokeHarness(t)
	harness.stage(t, root)
	shellPath := harness.shellPath(t, "zsh")
	tmuxPath := harness.executablePath(t, "tmux")
	const session = "intent-qualification"
	cleanupCommand := harness.tmuxCommand(t, tmuxPath, "kill-server")
	t.Cleanup(func() {
		_ = harness.command(false, cleanupCommand).Run()
	})

	client := harness.startTmuxSession(t, "zsh", shellPath, tmuxPath, session)
	configureStateDump(t, client)
	matrix := newTerminalConformanceCase(t, shellCase{name: "zsh"}, "alt+g", "alt+u", "screen-256color", []byte{'\r'}, 36, 120)
	original := "SSH-TMUX-INTENT_CASE_SAFE_7Q"
	dangerMarker := harness.remoteRoot + "/home/danger-ran"
	runSSHPhase(t, "safe-rewrite", func(t *testing.T) {
		client.write(t, "intent-sh config set provider codex >/dev/null")
		client.writeBytes(t, matrix.enter)
		client.readUntilTimeout(t, promptMarker, 10*time.Second)
		client.write(t, original)
		client.writeBytes(t, matrix.rewriteBytes)
		client.readUntilTimeout(t, "generated one", 30*time.Second)
		assertShellState(t, client, "printf GEN_ONE", len("printf GEN_ONE"), original, 0, "safe")
	})
	runSSHPhase(t, "safe-detach-reattach", func(t *testing.T) {
		harness.runTmux(t, tmuxPath, "detach-client", "-s", session)
		waitForPTYClientExit(t, client, 10*time.Second)
		client = harness.attachTmuxSession(t, "zsh", tmuxPath, session)
		assertShellState(t, client, "printf GEN_ONE", len("printf GEN_ONE"), original, 0, "safe")
		client.writeBytes(t, matrix.undoBytes)
		client.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
		assertShellState(t, client, original, len(original), "", 0, "")
		clearEditableLine(t, client)
	})
	runSSHPhase(t, "dangerous-first-acceptance", func(t *testing.T) {
		client.write(t, "INTENT_CASE_DANGER_7Q")
		client.writeBytes(t, matrix.rewriteBytes)
		client.readUntilTimeout(t, "DANGEROUS:", 30*time.Second)
		client.writeBytes(t, matrix.enter)
		client.readUntilTimeout(t, "Press Enter again to execute.", 10*time.Second)
		if harness.pathExists(t, dangerMarker) {
			t.Fatal("SSH-to-tmux dangerous command ran on first acceptance")
		}
	})
	runSSHPhase(t, "dangerous-detach-reattach", func(t *testing.T) {
		harness.runTmux(t, tmuxPath, "detach-client", "-s", session)
		waitForPTYClientExit(t, client, 10*time.Second)
		client = harness.attachTmuxSession(t, "zsh", tmuxPath, session)
		assertShellState(t, client, "touch "+dangerMarker, len("touch "+dangerMarker), "INTENT_CASE_DANGER_7Q", 0, "dangerous")
		client.writeBytes(t, matrix.enter)
		harness.waitForPath(t, dangerMarker, 10*time.Second)
		client.readUntilTimeout(t, promptMarker, 10*time.Second)
	})
	runSSHPhase(t, "independent-pane", func(t *testing.T) {
		environment := harness.remoteShellEnvironment(shellPath)
		innerShell := harness.remoteCleanShellCommand(environment, "zsh", shellPath)
		paneID := harness.runTmux(t, tmuxPath, "split-window", "-d", "-P", "-F", "#{pane_id}", "-t", session+":0", innerShell)
		if !regexp.MustCompile(`^%[0-9]{1,6}$`).MatchString(paneID) {
			t.Fatal("remote tmux returned an unsafe pane identifier")
		}
		resetClientOutput(client)
		harness.runTmux(t, tmuxPath, "select-pane", "-t", paneID)
		client.readUntilTimeout(t, promptMarker, 10*time.Second)
		client.write(t, `eval "$(intent-sh init zsh)"`)
		client.writeBytes(t, matrix.enter)
		client.readUntilTimeout(t, promptMarker, 10*time.Second)
		configureStateDump(t, client)
		assertShellState(t, client, "", 0, "", 0, "")
		client.write(t, "NEW-PANE-INTENT_CASE_SAFE_7Q")
		client.writeBytes(t, matrix.rewriteBytes)
		client.readUntilTimeout(t, "generated one", 30*time.Second)
		assertShellState(t, client, "printf GEN_ONE", len("printf GEN_ONE"), "NEW-PANE-INTENT_CASE_SAFE_7Q", 0, "safe")
		client.close(t)
		harness.assertPromptPrivacy(t, harness.remoteRoot+"/home/codex-last-prompt")
	})
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
