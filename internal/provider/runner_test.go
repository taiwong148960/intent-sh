package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/taiwong148960/intent-sh/internal/apperr"
)

type helperCapture struct {
	Args    []string          `json:"args"`
	Stdin   string            `json:"stdin"`
	CWD     string            `json:"cwd"`
	Entries []string          `json:"entries"`
	Env     map[string]string `json:"env"`
}

func TestProviderHelperProcess(t *testing.T) {
	separator := -1
	for index, arg := range os.Args {
		if arg == "--intent-sh-provider-helper" {
			separator = index
			break
		}
	}
	if separator < 0 {
		return
	}
	args := os.Args[separator+1:]
	if len(args) == 0 {
		os.Exit(90)
	}
	switch args[0] {
	case "capture":
		stdin, _ := io.ReadAll(os.Stdin)
		cwd, _ := os.Getwd()
		entries, _ := os.ReadDir(".")
		capture := helperCapture{Args: append([]string(nil), args[1:]...), Stdin: string(stdin), CWD: cwd, Env: map[string]string{}}
		for _, entry := range entries {
			capture.Entries = append(capture.Entries, entry.Name())
		}
		for _, entry := range os.Environ() {
			key, value, ok := strings.Cut(entry, "=")
			if ok {
				capture.Env[key] = value
			}
		}
		_ = json.NewEncoder(os.Stdout).Encode(capture)
	case "sleep":
		time.Sleep(10 * time.Second)
	case "stdout-flood":
		_, _ = os.Stdout.Write([]byte(strings.Repeat("o", 4096)))
	case "stderr-flood":
		_, _ = os.Stderr.Write([]byte(strings.Repeat("e", 4096)))
	case "result":
		_ = os.WriteFile(args[1], []byte(args[2]), 0o600)
	case "descendant":
		child := exec.Command(os.Args[0], "-test.run=TestProviderHelperProcess", "--", "--intent-sh-provider-helper", "child")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(91)
		}
		_, _ = fmt.Fprintf(os.Stdout, "%d\n", child.Process.Pid)
		time.Sleep(10 * time.Second)
	case "signal-aware":
		marker := args[1]
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		child := exec.Command(os.Args[0], "-test.run=TestProviderHelperProcess", "--", "--intent-sh-provider-helper", "child")
		if err := child.Start(); err != nil {
			os.Exit(93)
		}
		// The started marker is a readiness contract: cancellation may be sent
		// immediately after it appears, so install the handlers first.
		_ = os.WriteFile(marker, []byte(fmt.Sprintf("phase=started\nparent=%d\nchild=%d\n", os.Getpid(), child.Process.Pid)), 0o600)
		received := <-signals
		signal.Stop(signals)
		file, _ := os.OpenFile(marker, os.O_APPEND|os.O_WRONLY, 0)
		if file != nil {
			_, _ = fmt.Fprintf(file, "signal=%s\n", received)
			_ = file.Close()
		}
		_ = child.Wait()
		file, _ = os.OpenFile(marker, os.O_APPEND|os.O_WRONLY, 0)
		if file != nil {
			_, _ = fmt.Fprintln(file, "phase=exited")
			_ = file.Close()
		}
	case "child":
		time.Sleep(10 * time.Second)
	default:
		os.Exit(92)
	}
	os.Exit(0)
}

func TestProcessRunnerBoundariesAndCleanup(t *testing.T) {
	t.Parallel()
	runner := ProcessRunner{
		TempRoot: t.TempDir(),
		Env: []string{
			"PATH=/usr/bin:/bin",
			"HOME=/safe/home",
			"LANG=en_US.UTF-8",
			"LC_ALL=en_US.UTF-8",
			"HTTPS_PROXY=http://proxy.invalid",
			"CODEX_HOME=/safe/codex",
			"ANTHROPIC_API_KEY=secret-anthropic",
			"OPENAI_API_KEY=secret-openai",
			"INTENT_SECRET=secret-other",
			"INTENT_SH_ADAPTER_PROTOCOL=secret-protocol",
			"INTENT_SH_ADAPTER_BACKEND=secret-backend",
			"INTENT_SH_ADAPTER_EDITOR_VERSION=secret-editor-version",
			"INTENT_SH_ADAPTER_READY=secret-ready",
			"INTENT_SH_ADAPTER_FAILURE=secret-failure",
			"INTENT_SH_ADAPTER_CONFLICTS=secret-binding",
			"INTENT_SH_REWRITE_KEY=secret-rewrite-key",
			"INTENT_SH_UNDO_KEY=secret-undo-key",
			"BLE_VERSION=secret-blesh-version",
			"BLE_ATTACHED=secret-blesh-attachment",
			"TERM=secret-term",
			"TERM_PROGRAM=secret-term-program",
			"WT_SESSION=secret-terminal-session",
			"TMUX=secret-tmux-socket",
			"SSH_CONNECTION=secret-ssh-connection",
			"SSH_CLIENT=secret-ssh-client",
			"SSH_TTY=secret-ssh-tty",
		},
	}
	wantArg := `literal spaces ; $(touch should-not-run) "quotes"`
	result, err := runner.Run(context.Background(), helperInvocation("capture", wantArg))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	var capture helperCapture
	if err := json.Unmarshal(result.Stdout, &capture); err != nil {
		t.Fatalf("decode capture: %v; output %q", err, result.Stdout)
	}
	if len(capture.Args) != 1 || capture.Args[0] != wantArg {
		t.Fatalf("args = %#v, want one literal argument", capture.Args)
	}
	if capture.Stdin != "private prompt on stdin" {
		t.Fatalf("stdin = %q", capture.Stdin)
	}
	resolvedWorkDir, resolveErr := filepath.EvalSymlinks(result.WorkDir)
	if resolveErr != nil && !errors.Is(resolveErr, os.ErrNotExist) {
		t.Fatalf("resolve workdir: %v", resolveErr)
	}
	// The directory is removed before Run returns, so resolve its existing parent
	// to account for macOS's /var -> /private/var alias.
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(result.WorkDir))
	if err != nil {
		t.Fatalf("resolve workdir parent: %v", err)
	}
	resolvedWorkDir = filepath.Join(resolvedParent, filepath.Base(result.WorkDir))
	if capture.CWD != resolvedWorkDir {
		t.Fatalf("cwd = %q, workdir = %q", capture.CWD, resolvedWorkDir)
	}
	if len(capture.Entries) != 0 {
		t.Fatalf("initial workdir entries = %#v, want empty", capture.Entries)
	}
	for _, key := range []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "INTENT_SECRET",
		"INTENT_SH_ADAPTER_PROTOCOL", "INTENT_SH_ADAPTER_BACKEND", "INTENT_SH_ADAPTER_EDITOR_VERSION",
		"INTENT_SH_ADAPTER_READY", "INTENT_SH_ADAPTER_FAILURE", "INTENT_SH_ADAPTER_CONFLICTS",
		"INTENT_SH_REWRITE_KEY", "INTENT_SH_UNDO_KEY", "BLE_VERSION", "BLE_ATTACHED",
		"TERM", "TERM_PROGRAM", "WT_SESSION", "TMUX", "SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY",
	} {
		if _, ok := capture.Env[key]; ok {
			t.Fatalf("prohibited environment key %s reached child", key)
		}
	}
	for _, key := range []string{"PATH", "HOME", "LANG", "LC_ALL", "HTTPS_PROXY", "CODEX_HOME"} {
		if _, ok := capture.Env[key]; !ok {
			t.Fatalf("allowed environment key %s was removed", key)
		}
	}
	assertRemoved(t, result.WorkDir)
}

func TestProcessRunnerTimeoutAndCancellation(t *testing.T) {
	t.Parallel()
	t.Run("timeout", func(t *testing.T) {
		runner := ProcessRunner{TempRoot: t.TempDir()}
		invocation := helperInvocation("sleep")
		invocation.Timeout = 100 * time.Millisecond
		started := time.Now()
		result, err := runner.Run(context.Background(), invocation)
		if apperr.KindOf(err) != apperr.KindTimeout {
			t.Fatalf("kind = %q, want timeout; err=%v", apperr.KindOf(err), err)
		}
		if time.Since(started) > 3*time.Second {
			t.Fatal("timed-out provider was not terminated promptly")
		}
		assertRemoved(t, result.WorkDir)
	})
	t.Run("cancellation", func(t *testing.T) {
		runner := ProcessRunner{TempRoot: t.TempDir()}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result, err := runner.Run(ctx, helperInvocation("sleep"))
		if apperr.KindOf(err) != apperr.KindCancelled {
			t.Fatalf("kind = %q, want cancelled; err=%v", apperr.KindOf(err), err)
		}
		assertRemoved(t, result.WorkDir)
	})
}

func TestProcessRunnerGracefullySignalsThenReapsProviderTree(t *testing.T) {
	tests := []struct {
		name       string
		cancel     bool
		wantSignal string
	}{
		{name: "cancellation", cancel: true, wantSignal: "interrupt"},
		{name: "timeout", wantSignal: "terminated"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "provider-phases")
			runner := ProcessRunner{TempRoot: t.TempDir()}
			invocation := helperInvocation("signal-aware", marker)
			invocation.Timeout = 250 * time.Millisecond
			ctx := context.Background()
			var cancel context.CancelFunc
			if test.cancel {
				ctx, cancel = context.WithCancel(ctx)
				invocation.Timeout = 5 * time.Second
			}
			resultChannel := make(chan error, 1)
			go func() {
				_, err := runner.Run(ctx, invocation)
				resultChannel <- err
			}()
			waitForProviderMarker(t, marker, "phase=started")
			if cancel != nil {
				cancel()
			}
			err := <-resultChannel
			wantKind := apperr.KindTimeout
			if test.cancel {
				wantKind = apperr.KindCancelled
			}
			if apperr.KindOf(err) != wantKind {
				t.Fatalf("kind = %q, want %q; err=%v", apperr.KindOf(err), wantKind, err)
			}
			phases, readErr := os.ReadFile(marker)
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, want := range []string{"phase=started", "signal=" + test.wantSignal, "phase=exited"} {
				if !strings.Contains(string(phases), want) {
					t.Fatalf("provider phases omitted %q: %s", want, phases)
				}
			}
			assertRecordedPIDsExited(t, string(phases))
		})
	}
}

func waitForProviderMarker(t *testing.T, path, value string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), value) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("provider marker did not reach %s", value)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertRecordedPIDsExited(t *testing.T, phases string) {
	t.Helper()
	for _, line := range strings.Split(phases, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || (key != "parent" && key != "child") {
			continue
		}
		pid, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("invalid recorded pid: %q", line)
		}
		if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
			t.Fatalf("recorded %s process %d survived teardown: %v", key, pid, err)
		}
	}
}

func TestProcessRunnerBoundsOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		scenario  string
		configure func(*Invocation)
		length    func(RunResult) int
	}{
		{"stdout", "stdout-flood", func(inv *Invocation) { inv.StdoutMax = 64 }, func(result RunResult) int { return len(result.Stdout) }},
		{"stderr", "stderr-flood", func(inv *Invocation) { inv.StderrMax = 48 }, func(result RunResult) int { return len(result.Stderr) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := ProcessRunner{TempRoot: t.TempDir()}
			invocation := helperInvocation(test.scenario)
			test.configure(&invocation)
			result, err := runner.Run(context.Background(), invocation)
			if apperr.KindOf(err) != apperr.KindProviderOutput {
				t.Fatalf("kind = %q, want provider output; err=%v", apperr.KindOf(err), err)
			}
			if got := test.length(result); got > 64 {
				t.Fatalf("captured %d bytes, bound not enforced", got)
			}
			assertRemoved(t, result.WorkDir)
		})
	}
}

func TestProcessRunnerReadsBoundedResultAndRemovesWorkspace(t *testing.T) {
	t.Parallel()
	runner := ProcessRunner{TempRoot: t.TempDir()}
	invocation := helperInvocation("result", "provider-result.json", `{"status":"clarify","question":"where?"}`)
	invocation.Files = map[string][]byte{"schema.json": []byte(`{"type":"object"}`)}
	invocation.ResultFile = "provider-result.json"
	result, err := runner.Run(context.Background(), invocation)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(string(result.ResultFile), `"clarify"`) {
		t.Fatalf("result file = %q", result.ResultFile)
	}
	assertRemoved(t, result.WorkDir)
}

func TestReadBoundedFileRejectsLinksAndSpecialFiles(t *testing.T) {
	dir := t.TempDir()
	secret := "SECRET_PROVIDER_RESULT_TARGET"
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "result-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "result-fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{link, fifo} {
		_, err := readBoundedFile(path, 1024)
		if apperr.KindOf(err) != apperr.KindProviderOutput {
			t.Fatalf("readBoundedFile(%q) kind = %q, want provider output; err=%v", path, apperr.KindOf(err), err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("readBoundedFile(%q) exposed linked content: %v", path, err)
		}
	}
}

func TestProcessRunnerKillsDescendantProcessGroup(t *testing.T) {
	runner := ProcessRunner{TempRoot: t.TempDir()}
	invocation := helperInvocation("descendant")
	invocation.Timeout = 200 * time.Millisecond
	result, err := runner.Run(context.Background(), invocation)
	if apperr.KindOf(err) != apperr.KindTimeout {
		t.Fatalf("kind = %q, want timeout; err=%v", apperr.KindOf(err), err)
	}
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(result.Stdout)))
	if parseErr != nil {
		t.Fatalf("parse child pid from %q: %v", result.Stdout, parseErr)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		killErr := syscall.Kill(pid, 0)
		if errors.Is(killErr, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant process %d remained alive after process-group cleanup", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
	assertRemoved(t, result.WorkDir)
}

func TestProcessRunnerRejectsEscapingFilePaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	invocation := helperInvocation("capture")
	invocation.Files = map[string][]byte{"../escape": []byte("no")}
	_, err := (ProcessRunner{TempRoot: root}).Run(context.Background(), invocation)
	if err == nil {
		t.Fatal("Run() accepted an escaping path")
	}
	if _, statErr := os.Stat(filepath.Join(root, "escape")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("unexpected escaped file: %v", statErr)
	}
}

func helperInvocation(scenario string, args ...string) Invocation {
	allArgs := []string{"-test.run=TestProviderHelperProcess", "--", "--intent-sh-provider-helper", scenario}
	allArgs = append(allArgs, args...)
	return Invocation{
		Program: os.Args[0],
		Args:    fixedArgs(allArgs...),
		Stdin:   []byte("private prompt on stdin"),
		Timeout: 2 * time.Second,
	}
}

func assertRemoved(t *testing.T, path string) {
	t.Helper()
	if path == "" {
		t.Fatal("runner did not report its work directory")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("work directory %q was not removed: %v", path, err)
	}
}
