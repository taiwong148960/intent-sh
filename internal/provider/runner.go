package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/taiwong148960/intent-sh/internal/apperr"
)

const (
	defaultStdoutLimit = 64 * 1024
	defaultStderrLimit = 16 * 1024
	defaultResultLimit = 64 * 1024
	maxControlFileSize = 128 * 1024
)

// Invocation describes one direct process call. Args receives the freshly
// created working directory so adapters can refer only to controlled files.
type Invocation struct {
	Program    string
	Args       func(workDir string) []string
	Stdin      []byte
	Files      map[string][]byte
	ResultFile string
	Timeout    time.Duration
	StdoutMax  int
	StderrMax  int
	ResultMax  int
}

// RunResult contains bounded process output. WorkDir is returned for cleanup tests;
// it no longer exists by the time Run returns.
type RunResult struct {
	Stdout          []byte
	Stderr          []byte
	ResultFile      []byte
	WorkDir         string
	StdoutTruncated bool
	StderrTruncated bool
}

// CommandRunner is the process seam used by provider adapter contract tests.
type CommandRunner interface {
	Run(context.Context, Invocation) (RunResult, error)
}

// ProcessRunner executes official provider binaries without a shell.
type ProcessRunner struct {
	TempRoot string
	Env      []string
	LookPath func(string) (string, error)
}

// ExitError retains bounded diagnostics for provider-specific classification.
type ExitError struct {
	Code   int
	Stderr string
	err    error
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("provider process exited with status %d", e.Code)
}
func (e *ExitError) Unwrap() error { return e.err }

func (r ProcessRunner) Run(ctx context.Context, invocation Invocation) (result RunResult, retErr error) {
	if strings.TrimSpace(invocation.Program) == "" {
		return result, apperr.New(apperr.KindProviderUnavailable, "start provider", "provider executable was not configured")
	}
	if invocation.Timeout <= 0 {
		return result, apperr.New(apperr.KindConfiguration, "start provider", "provider timeout must be positive")
	}
	if err := validateControlledPaths(invocation); err != nil {
		return result, err
	}

	workDir, err := os.MkdirTemp(r.TempRoot, "intent-sh-provider-*")
	if err != nil {
		return result, apperr.Wrap(apperr.KindInternal, "prepare provider", "could not create the provider workspace", err)
	}
	result.WorkDir = workDir
	defer func() {
		if cleanupErr := os.RemoveAll(workDir); cleanupErr != nil && retErr == nil {
			retErr = apperr.Wrap(apperr.KindInternal, "clean provider", "could not remove the provider workspace", cleanupErr)
		}
	}()

	for name, data := range invocation.Files {
		if len(data) > maxControlFileSize {
			return result, apperr.New(apperr.KindProviderOutput, "prepare provider", "provider control file exceeded the size limit")
		}
		path := filepath.Join(workDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return result, apperr.Wrap(apperr.KindInternal, "prepare provider", "could not create a provider control directory", err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return result, apperr.Wrap(apperr.KindInternal, "prepare provider", "could not write a provider control file", err)
		}
	}

	lookPath := r.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	program, err := lookPath(invocation.Program)
	if err != nil {
		return result, apperr.Wrap(apperr.KindProviderUnavailable, "start provider", "provider executable was not found", err)
	}
	args := []string(nil)
	if invocation.Args != nil {
		args = invocation.Args(workDir)
	}

	runCtx, cancel := context.WithTimeout(ctx, invocation.Timeout)
	defer cancel()
	cmd := exec.Command(program, args...)
	cmd.Dir = workDir
	cmd.Env = allowEnvironment(r.environment())
	cmd.Stdin = bytes.NewReader(invocation.Stdin)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout := newCappedBuffer(limitOrDefault(invocation.StdoutMax, defaultStdoutLimit))
	stderr := newCappedBuffer(limitOrDefault(invocation.StderrMax, defaultStderrLimit))
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return result, apperr.Wrap(apperr.KindProviderUnavailable, "start provider", "provider process could not be started", err)
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	var waitErr error
	select {
	case waitErr = <-waitCh:
		killProcessGroup(cmd.Process.Pid)
	case <-runCtx.Done():
		killProcessGroup(cmd.Process.Pid)
		waitErr = <-waitCh
		result = collectRunResult(result, stdout, stderr)
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return result, apperr.Wrap(apperr.KindTimeout, "run provider", "provider timed out", runCtx.Err())
		}
		return result, apperr.Wrap(apperr.KindCancelled, "run provider", "provider request was cancelled", runCtx.Err())
	}
	result = collectRunResult(result, stdout, stderr)

	if result.StdoutTruncated || result.StderrTruncated {
		return result, apperr.New(apperr.KindProviderOutput, "run provider", "provider output exceeded the capture limit")
	}
	if waitErr != nil {
		code := -1
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			code = exitErr.ExitCode()
		}
		return result, &ExitError{Code: code, Stderr: string(result.Stderr), err: waitErr}
	}
	if invocation.ResultFile != "" {
		data, readErr := readBoundedFile(filepath.Join(workDir, filepath.FromSlash(invocation.ResultFile)), limitOrDefault(invocation.ResultMax, defaultResultLimit))
		if readErr != nil {
			return result, readErr
		}
		result.ResultFile = data
	}
	return result, nil
}

func (r ProcessRunner) environment() []string {
	if r.Env != nil {
		return r.Env
	}
	return os.Environ()
}

func validateControlledPaths(invocation Invocation) error {
	for name := range invocation.Files {
		if !validRelativePath(name) {
			return apperr.New(apperr.KindInternal, "prepare provider", "provider control file path was invalid")
		}
	}
	if invocation.ResultFile != "" && !validRelativePath(invocation.ResultFile) {
		return apperr.New(apperr.KindInternal, "prepare provider", "provider result file path was invalid")
	}
	return nil
}

func validRelativePath(name string) bool {
	return fs.ValidPath(name) && name != "." && !filepath.IsAbs(name)
}

func allowEnvironment(source []string) []string {
	allowed := make([]string, 0, len(source))
	for _, entry := range source {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || !allowedEnvironmentKey(key) {
			continue
		}
		allowed = append(allowed, entry)
	}
	return allowed
}

func allowedEnvironmentKey(key string) bool {
	switch key {
	case "PATH", "HOME", "USER", "LOGNAME", "TMPDIR", "LANG", "TZ",
		"SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS",
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY",
		"http_proxy", "https_proxy", "no_proxy", "all_proxy",
		"CODEX_HOME", "CLAUDE_CONFIG_DIR":
		return true
	default:
		return strings.HasPrefix(key, "LC_")
	}
}

func collectRunResult(result RunResult, stdout, stderr *cappedBuffer) RunResult {
	result.Stdout = stdout.Bytes()
	result.Stderr = stderr.Bytes()
	result.StdoutTruncated = stdout.Truncated()
	result.StderrTruncated = stderr.Truncated()
	return result
}

func readBoundedFile(path string, limit int) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, apperr.Wrap(apperr.KindProviderOutput, "read provider result", "provider did not write a result", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, apperr.Wrap(apperr.KindProviderOutput, "read provider result", "could not inspect the provider result", err)
	}
	if !info.Mode().IsRegular() {
		return nil, apperr.New(apperr.KindProviderOutput, "read provider result", "provider result was not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, apperr.Wrap(apperr.KindProviderOutput, "read provider result", "could not read the provider result", err)
	}
	if len(data) > limit {
		return nil, apperr.New(apperr.KindProviderOutput, "read provider result", "provider result exceeded the output limit")
	}
	return data, nil
}

func killProcessGroup(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

func limitOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

type cappedBuffer struct {
	limit     int
	data      []byte
	truncated bool
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit, data: make([]byte, 0, limit)}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		keep := len(p)
		if keep > remaining {
			keep = remaining
		}
		b.data = append(b.data, p[:keep]...)
	}
	if len(p) > remaining {
		b.truncated = true
	}
	return len(p), nil
}

func (b *cappedBuffer) Bytes() []byte   { return append([]byte(nil), b.data...) }
func (b *cappedBuffer) Truncated() bool { return b.truncated }
