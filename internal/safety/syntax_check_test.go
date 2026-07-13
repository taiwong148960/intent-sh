package safety

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"mvdan.cc/sh/v3/syntax"
)

func TestSyntaxInvocationDisablesStartupFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		shell   string
		program string
		flags   []string
	}{
		{ShellBash, "bash", []string{"--noprofile", "--norc", "-n", "-c"}},
		{ShellZsh, "zsh", []string{"-f", "-n", "-c"}},
	}
	for _, test := range tests {
		t.Run(test.shell, func(t *testing.T) {
			t.Parallel()
			program, args, err := syntaxInvocation(test.shell, "pwd")
			if err != nil || program != test.program {
				t.Fatalf("syntaxInvocation() = %q, %#v, %v", program, args, err)
			}
			for index, flag := range test.flags {
				if args[index] != flag {
					t.Fatalf("args = %#v, want prefix %#v", args, test.flags)
				}
			}
			if args[len(args)-1] != "pwd" {
				t.Fatalf("command arg = %q", args[len(args)-1])
			}
		})
	}
}

func TestExecSyntaxCheckerNeverExecutesCommandParts(t *testing.T) {
	t.Parallel()
	for _, shell := range []string{ShellBash, ShellZsh} {
		shell := shell
		t.Run(shell, func(t *testing.T) {
			t.Parallel()
			if _, err := exec.LookPath(shell); err != nil {
				t.Skipf("%s is not installed", shell)
			}
			dir := t.TempDir()
			substitutionMarker := filepath.Join(dir, "substitution-ran")
			redirectionMarker := filepath.Join(dir, "redirection-ran")
			quotedSubstitution, _ := syntax.Quote(substitutionMarker, syntax.LangBash)
			quotedRedirection, _ := syntax.Quote(redirectionMarker, syntax.LangBash)
			command := `printf '%s' "$(touch ` + quotedSubstitution + `)" > ` + quotedRedirection
			if err := (ExecSyntaxChecker{}).Check(context.Background(), shell, command); err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			for _, marker := range []string{substitutionMarker, redirectionMarker} {
				if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("syntax check executed command component and created %q", marker)
				}
			}
		})
	}
}

func TestExecSyntaxCheckerDropsStartupEnvironment(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "bash-env-ran")
	bashEnv := filepath.Join(dir, "BASH_ENV")
	if err := os.WriteFile(bashEnv, []byte("touch "+marker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BASH_ENV", bashEnv)
	if err := (ExecSyntaxChecker{}).Check(context.Background(), ShellBash, "pwd"); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("BASH_ENV was loaded during syntax check: %v", err)
	}
}

func TestExecSyntaxCheckerRejectsInvalidOrUnknownShellSyntax(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("bash"); err == nil {
		err := (ExecSyntaxChecker{}).Check(context.Background(), ShellBash, `echo "$(("`)
		if apperr.KindOf(err) != apperr.KindSafety {
			t.Fatalf("invalid Bash kind = %q, err=%v", apperr.KindOf(err), err)
		}
	}
	err := (ExecSyntaxChecker{}).Check(context.Background(), "fish", "pwd")
	if apperr.KindOf(err) != apperr.KindSafety {
		t.Fatalf("unknown shell kind = %q", apperr.KindOf(err))
	}
}

func TestExecSyntaxCheckerHonorsCancellation(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (ExecSyntaxChecker{}).Check(ctx, ShellBash, "pwd")
	if apperr.KindOf(err) != apperr.KindCancelled {
		t.Fatalf("kind = %q, want cancelled; err=%v", apperr.KindOf(err), err)
	}
}
