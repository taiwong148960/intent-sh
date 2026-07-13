package safety

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/taiwong148960/intent-sh/internal/apperr"
)

const (
	syntaxTimeout     = 2 * time.Second
	syntaxOutputLimit = 4 * 1024
)

// ExecSyntaxChecker invokes bash or zsh in parse-only mode from an isolated HOME.
type ExecSyntaxChecker struct {
	LookPath func(string) (string, error)
	TempRoot string
}

func (c ExecSyntaxChecker) Check(ctx context.Context, shell, command string) error {
	program, args, err := syntaxInvocation(shell, command)
	if err != nil {
		return err
	}
	lookPath := c.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	program, err = lookPath(program)
	if err != nil {
		return apperr.Wrap(apperr.KindSafety, "check shell syntax", "target shell executable was not found", err)
	}
	workDir, err := os.MkdirTemp(c.TempRoot, "intent-sh-syntax-*")
	if err != nil {
		return apperr.Wrap(apperr.KindInternal, "check shell syntax", "could not create the syntax-check workspace", err)
	}
	defer os.RemoveAll(workDir)

	runCtx, cancel := context.WithTimeout(ctx, syntaxTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, program, args...)
	cmd.Dir = workDir
	cmd.Env = syntaxEnvironment(workDir)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	stderr := &limitedWriter{remaining: syntaxOutputLimit}
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(runCtx.Err(), context.Canceled) {
			return apperr.Wrap(apperr.KindCancelled, "check shell syntax", "syntax check was cancelled", runCtx.Err())
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return apperr.Wrap(apperr.KindTimeout, "check shell syntax", "syntax check timed out", runCtx.Err())
		}
		return apperr.Wrap(apperr.KindSafety, "check shell syntax", "generated command is not valid "+shell+" syntax", err)
	}
	return nil
}

func syntaxInvocation(shell, command string) (string, []string, error) {
	switch shell {
	case ShellBash:
		return "bash", []string{"--noprofile", "--norc", "-n", "-c", command}, nil
	case ShellZsh:
		return "zsh", []string{"-f", "-n", "-c", command}, nil
	default:
		return "", nil, safetyError("check shell syntax", "supported target shells are bash and zsh")
	}
}

func syntaxEnvironment(home string) []string {
	result := []string{"HOME=" + home}
	for _, key := range []string{"PATH", "LANG", "LC_ALL", "LC_CTYPE", "TZ"} {
		if value, ok := os.LookupEnv(key); ok {
			result = append(result, key+"="+value)
		}
	}
	return result
}

type limitedWriter struct {
	remaining int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	original := len(p)
	if len(p) > w.remaining {
		p = p[:max(w.remaining, 0)]
	}
	w.remaining -= len(p)
	return original, nil
}
