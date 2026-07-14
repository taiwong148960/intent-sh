package shelltest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/taiwong148960/intent-sh/internal/config"
	"github.com/taiwong148960/intent-sh/internal/provider"
)

// tmuxTestServer always addresses tmux with an explicit private socket and an
// empty -f file. It never reaches the user's default server or configuration,
// and the tests never use capture-pane or any other scrollback-reading command.
type tmuxTestServer struct {
	path       string
	socketPath string
	configPath string
	processEnv []string
}

func newTmuxTestServer(t *testing.T) *tmuxTestServer {
	t.Helper()
	path := os.Getenv("INTENT_SH_TEST_TMUX")
	if path == "" {
		var err error
		path, err = exec.LookPath("tmux")
		if err != nil {
			t.Skip("tmux is not installed; install it or set INTENT_SH_TEST_TMUX")
		}
	}
	configPath := filepath.Join(t.TempDir(), "empty-tmux.conf")
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatalf("write empty tmux configuration: %v", err)
	}
	// tmux expands -L below TMUX_TMPDIR/tmux-<uid>. Go's macOS test temp
	// directories are long enough for that expansion to exceed sockaddr_un.
	// An explicit -S path under a private short directory avoids that platform
	// limit while retaining a unique server per test.
	socketDirectory, err := os.MkdirTemp("/tmp", "intent-sh-tmux-")
	if err != nil {
		t.Fatalf("create private tmux socket directory: %v", err)
	}
	server := &tmuxTestServer{
		path:       path,
		socketPath: filepath.Join(socketDirectory, "socket"),
		configPath: configPath,
		processEnv: tmuxProcessEnvironment(os.Environ(), socketDirectory),
	}
	t.Cleanup(func() {
		cmd := exec.Command(server.path, "-S", server.socketPath, "-f", server.configPath, "kill-server")
		cmd.Env = server.processEnv
		_ = cmd.Run()
		_ = os.RemoveAll(socketDirectory)
	})
	return server
}

func tmuxProcessEnvironment(source []string, privateSocketDirectory string) []string {
	return replaceEnvironment(source, map[string]string{
		"TMUX":        "",
		"TMUX_TMPDIR": privateSocketDirectory,
	})
}

func (server *tmuxTestServer) args(command ...string) []string {
	return append([]string{"-S", server.socketPath, "-f", server.configPath}, command...)
}

func (server *tmuxTestServer) run(t *testing.T, command ...string) string {
	t.Helper()
	cmd := exec.Command(server.path, server.args(command...)...)
	cmd.Env = server.processEnv
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run isolated tmux %s: %v: %s", strings.Join(command, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func tmuxShellCommand(shell shellCase, environment map[string]string) string {
	parts := []string{"env", "-i"}
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, key+"="+shellQuote(environment[key]))
	}
	parts = append(parts, shellQuote(shell.executable))
	for _, arg := range shell.args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func (server *tmuxTestServer) clientEnvironment(environment map[string]string) map[string]string {
	result := make(map[string]string, len(environment)+2)
	for key, value := range environment {
		result[key] = value
	}
	for _, entry := range server.processEnv {
		key, value, ok := strings.Cut(entry, "=")
		if ok && (key == "TMUX" || key == "TMUX_TMPDIR") {
			result[key] = value
		}
	}
	return result
}

func (server *tmuxTestServer) startSession(t *testing.T, matrix terminalConformanceCase, environment map[string]string, session string) *runningShell {
	t.Helper()
	clientEnvironment := server.clientEnvironment(environment)
	client := shellCase{
		name:       matrix.shell.name,
		executable: server.path,
		args: server.args(
			"new-session", "-s", session,
			"-x", strconv.Itoa(int(matrix.cols)), "-y", strconv.Itoa(int(matrix.rows)),
			tmuxShellCommand(matrix.shell, environment),
		),
	}
	return startShellWithPTYOptions(t, client, clientEnvironment, `eval "$(intent-sh init `+matrix.shell.name+`)"`, terminalPTYOptions{
		term: "xterm-256color", rows: matrix.rows, cols: matrix.cols, respondTerminalQueries: true,
	})
}

func (server *tmuxTestServer) attach(t *testing.T, matrix terminalConformanceCase, environment map[string]string, target string) *runningShell {
	t.Helper()
	clientEnvironment := server.clientEnvironment(environment)
	client := shellCase{
		name:       matrix.shell.name,
		executable: server.path,
		args:       server.args("attach-session", "-t", target),
	}
	return startShellWithPTYOptions(t, client, clientEnvironment, "", terminalPTYOptions{
		term: "xterm-256color", rows: matrix.rows, cols: matrix.cols, respondTerminalQueries: true,
	})
}

func stopDetachedTmuxClient(t *testing.T, server *tmuxTestServer, client *runningShell, target string) {
	t.Helper()
	// Detach through the private server rather than assuming the server's
	// current prefix table. Prefix delivery has its own intercepted-binding
	// coverage; this journey verifies that the live pane state survives a
	// client detach and a fresh attach.
	server.run(t, "detach-client", "-s", target)
	done := make(chan error, 1)
	go func() { done <- client.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("detach isolated tmux client: %v", err)
		}
	case <-time.After(5 * time.Second):
		_ = client.cmd.Process.Kill()
		<-done
		t.Fatal("timed out detaching isolated tmux client")
	}
	_ = client.file.Close()
}

func resetClientOutput(client *runningShell) {
	client.pending = ""
	for {
		select {
		case <-client.chunks:
		default:
			return
		}
	}
}

func tmuxConformanceEnvironment(t *testing.T, matrix terminalConformanceCase, binDir, providerMode string, priority []string) (map[string]string, string) {
	t.Helper()
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	cfg := config.Defaults()
	cfg.Provider = providerMode
	cfg.Priority = append([]string(nil), priority...)
	cfg.TimeoutSeconds = 5
	cfg.RewriteKey = matrix.rewriteValue
	cfg.UndoKey = matrix.undoValue
	if err := config.WriteAt(filepath.Join(xdg, "intent-sh", "config.toml"), cfg); err != nil {
		t.Fatalf("write tmux conformance config: %v", err)
	}
	return map[string]string{
		"HOME": home, "XDG_CONFIG_HOME": xdg, "SHELL": matrix.shell.executable,
		"PATH": binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"PS1":  promptMarker, "PROMPT": promptMarker, "TERM": matrix.term,
	}, home
}

func TestTmuxHarnessUsesPrivateSocketEnvironmentAndCleanInnerShell(t *testing.T) {
	privateDirectory := filepath.Join(t.TempDir(), "socket-root")
	environment := tmuxProcessEnvironment([]string{"TMUX=/user/socket,1,2", "TMUX_TMPDIR=/user/tmux", "KEEP=value"}, privateDirectory)
	values := make(map[string]string)
	for _, entry := range environment {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	if values["TMUX"] != "" || values["TMUX_TMPDIR"] != privateDirectory || values["KEEP"] != "value" {
		t.Fatalf("isolated tmux environment = %#v", values)
	}
	command := tmuxShellCommand(shellCase{executable: "/bin/zsh", args: []string{"-f", "-i"}}, map[string]string{"HOME": "/tmp/test-home"})
	if !strings.HasPrefix(command, "env -i ") || strings.Contains(command, "KEEP=value") {
		t.Fatalf("inner tmux shell did not start with a clean environment: %q", command)
	}
}

func TestTmuxLifecycleMatrix(t *testing.T) {
	root := repositoryRoot(t)
	binDir := buildMVPTools(t, root)
	for _, shell := range nativeConformanceShells(root) {
		for _, chordCase := range []struct {
			name, rewrite, undo string
			enter               []byte
		}{
			{name: "default-alt-cr", rewrite: "alt+g", undo: "alt+u", enter: []byte{'\r'}},
			{name: "custom-ctrl-lf", rewrite: "ctrl+x", undo: "ctrl+r", enter: []byte{'\n'}},
		} {
			t.Run(shell.name+"/"+chordCase.name, func(t *testing.T) {
				requireCompatibleShell(t, shell)
				matrix := newTerminalConformanceCase(t, shell, chordCase.rewrite, chordCase.undo, "screen-256color", chordCase.enter, 32, 110)
				server := newTmuxTestServer(t)
				environment, home := tmuxConformanceEnvironment(t, matrix, binDir, config.ProviderAuto, []string{provider.NameClaude, provider.NameCodex})
				client := server.startSession(t, matrix, environment, "lifecycle")
				defer client.close(t)
				configureStateDump(t, client)

				runConformanceLifecycleOnShell(t, client, home, matrix)
				client.readUntilTimeout(t, promptMarker, 10*time.Second)

				client.write(t, "INTENT_CASE_RESIZE_7Q")
				client.writeBytes(t, matrix.rewriteBytes)
				client.readUntilTimeout(t, "Ctrl+C to cancel", 10*time.Second)
				client.resize(t, 46, 148)
				client.readUntilTimeout(t, "resize result", 30*time.Second)
				assertShellState(t, client, "printf RESIZED", len("printf RESIZED"), "INTENT_CASE_RESIZE_7Q", 0, "safe")
				client.writeBytes(t, matrix.undoBytes)
				client.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
				clearEditableLine(t, client)
			})
		}
	}
}

func TestTmuxDetachReattachAndSessionStateIsolation(t *testing.T) {
	root := repositoryRoot(t)
	binDir := buildMVPTools(t, root)
	shellCase := shellCase{name: "zsh", executable: "zsh", args: []string{"-f", "-i"}, script: filepath.Join(root, "shell", "zsh", "intent-sh.zsh")}
	requireCompatibleShell(t, shellCase)
	matrix := newTerminalConformanceCase(t, shellCase, "alt+g", "alt+u", "screen-256color", []byte{'\r'}, 32, 110)
	server := newTmuxTestServer(t)
	environment, home := tmuxConformanceEnvironment(t, matrix, binDir, config.ProviderCodex, []string{provider.NameCodex})
	client := server.startSession(t, matrix, environment, "primary")
	configureStateDump(t, client)

	original := "PRIMARY-INTENT_CASE_SAFE_7Q"
	client.write(t, original)
	client.writeBytes(t, matrix.rewriteBytes)
	client.readUntilTimeout(t, "generated one", 30*time.Second)
	assertShellState(t, client, "printf GEN_ONE", len("printf GEN_ONE"), original, 0, "safe")
	stopDetachedTmuxClient(t, server, client, "primary")

	client = server.attach(t, matrix, environment, "primary")
	client.readUntilTimeout(t, "printf GEN_ONE", 10*time.Second)
	assertShellState(t, client, "printf GEN_ONE", len("printf GEN_ONE"), original, 0, "safe")
	client.writeBytes(t, matrix.undoBytes)
	client.readUntilTimeout(t, "restored the original buffer", 10*time.Second)
	assertShellState(t, client, original, len(original), "", 0, "")
	clearEditableLine(t, client)

	dangerMarker := filepath.Join(home, "danger-ran")
	client.write(t, "INTENT_CASE_DANGER_7Q")
	client.writeBytes(t, matrix.rewriteBytes)
	client.readUntilTimeout(t, "DANGEROUS:", 30*time.Second)
	client.writeBytes(t, matrix.enter)
	client.readUntilTimeout(t, "Press Enter again to execute.", 10*time.Second)
	stopDetachedTmuxClient(t, server, client, "primary")

	client = server.attach(t, matrix, environment, "primary")
	defer client.close(t)
	client.readUntilTimeout(t, "touch "+dangerMarker, 10*time.Second)
	assertShellState(t, client, "touch "+dangerMarker, len("touch "+dangerMarker), "INTENT_CASE_DANGER_7Q", 0, "dangerous")
	client.writeBytes(t, matrix.enter)
	waitForPathTimeout(t, dangerMarker, 10*time.Second, client)
	client.readUntilTimeout(t, promptMarker, 10*time.Second)

	// A new pane starts a distinct shell process. Switch to it through tmux's
	// control command and assert its state through the shell widget, never by
	// reading pane contents through tmux.
	paneID := server.run(t, "split-window", "-d", "-P", "-F", "#{pane_id}", "-t", "primary:0", tmuxShellCommand(shellCase, environment))
	resetClientOutput(client)
	server.run(t, "select-pane", "-t", paneID)
	client.readUntilTimeout(t, promptMarker, 10*time.Second)
	client.write(t, `eval "$(intent-sh init zsh)"`)
	client.writeBytes(t, matrix.enter)
	client.readUntilTimeout(t, promptMarker, 10*time.Second)
	configureStateDump(t, client)
	assertShellState(t, client, "", 0, "", 0, "")
	client.write(t, "PANE-INTENT_CASE_SAFE_7Q")
	client.writeBytes(t, matrix.rewriteBytes)
	client.readUntilTimeout(t, "generated one", 30*time.Second)
	assertShellState(t, client, "printf GEN_ONE", len("printf GEN_ONE"), "PANE-INTENT_CASE_SAFE_7Q", 0, "safe")

	// A different tmux session is also a distinct shell process and has no
	// rewrite or confirmation state from either pane in the primary session.
	server.run(t, "new-session", "-d", "-s", "secondary", "-x", "110", "-y", "32", tmuxShellCommand(shellCase, environment))
	secondary := server.attach(t, matrix, environment, "secondary")
	defer secondary.close(t)
	secondary.write(t, `eval "$(intent-sh init zsh)"`)
	secondary.writeBytes(t, matrix.enter)
	secondary.readUntilTimeout(t, promptMarker, 10*time.Second)
	configureStateDump(t, secondary)
	assertShellState(t, secondary, "", 0, "", 0, "")
	secondary.write(t, "SESSION-INTENT_CASE_SAFE_7Q")
	secondary.writeBytes(t, matrix.rewriteBytes)
	secondary.readUntilTimeout(t, "generated one", 30*time.Second)
	assertShellState(t, secondary, "printf GEN_ONE", len("printf GEN_ONE"), "SESSION-INTENT_CASE_SAFE_7Q", 0, "safe")

	assertShellState(t, client, "printf GEN_ONE", len("printf GEN_ONE"), "PANE-INTENT_CASE_SAFE_7Q", 0, "safe")
	resetClientOutput(client)
	server.run(t, "select-pane", "-t", "primary:0.0")
	// Some tmux builds repaint only the pane border when select-pane is
	// issued through the private control client. Query the selected shell's
	// bounded state widget directly rather than relying on prompt repaint.
	assertShellState(t, client, "", 0, "", 0, "")
	clearEditableLine(t, secondary)
}

func TestTmuxInterceptedRootBindingFailsKeyDeliveryDiagnostic(t *testing.T) {
	root := repositoryRoot(t)
	binDir := buildMVPTools(t, root)
	shellCase := shellCase{name: "zsh", executable: "zsh", args: []string{"-f", "-i"}, script: filepath.Join(root, "shell", "zsh", "intent-sh.zsh")}
	requireCompatibleShell(t, shellCase)
	matrix := newTerminalConformanceCase(t, shellCase, "alt+g", "alt+u", "screen-256color", []byte{'\r'}, 32, 110)
	server := newTmuxTestServer(t)
	environment, home := tmuxConformanceEnvironment(t, matrix, binDir, config.ProviderCodex, []string{provider.NameCodex})
	client := server.startSession(t, matrix, environment, "intercepted")
	defer client.close(t)

	server.run(t, "bind-key", "-n", "M-g", "send-keys", "-l", "X")
	client.write(t, "intent-sh doctor --keys")
	client.writeBytes(t, matrix.enter)
	client.readUntilTimeout(t, "press Alt+G now", 20*time.Second)
	client.writeBytes(t, matrix.rewriteBytes)
	client.readUntilTimeout(t, "press Alt+U now", 10*time.Second)
	client.writeBytes(t, matrix.undoBytes)
	client.readUntilTimeout(t, "press Enter now", 10*time.Second)
	client.writeBytes(t, matrix.enter)
	client.readUntilTimeout(t, "press Ctrl+C now", 10*time.Second)
	client.writeBytes(t, []byte{0x03})
	output := client.readUntilTimeout(t, "NOT_READY resolve the failed checks above", 20*time.Second)
	for _, want := range []string{"FAIL terminal.keys.rewrite", "intercepted or transformed", "0x58", "intent-sh config set rewrite_key"} {
		if !strings.Contains(output, want) {
			t.Fatalf("intercepted tmux diagnostic omitted %q: %q", want, output)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "codex-invoked")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("key-delivery diagnostic invoked a provider: %v", err)
	}
}
