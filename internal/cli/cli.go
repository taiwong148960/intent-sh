// Package cli owns top-level command dispatch.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/taiwong148960/intent-sh/internal/app"
	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/config"
	"github.com/taiwong148960/intent-sh/internal/doctor"
	"github.com/taiwong148960/intent-sh/internal/protocol"
	setupguide "github.com/taiwong148960/intent-sh/internal/setup"
	"github.com/taiwong148960/intent-sh/internal/textsafe"
	shellassets "github.com/taiwong148960/intent-sh/shell"
)

var Version = "dev"

type Command struct {
	Service *app.Service
	Doctor  DoctorRunner
}

// DoctorRunner is the read-only diagnostics seam used by command tests.
type DoctorRunner interface {
	Run(context.Context) doctor.Report
}

type DoctorKeysRunner interface {
	RunKeys(context.Context) doctor.Report
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return RunContext(context.Background(), args, stdin, stdout, stderr)
}

func RunContext(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return (Command{}).Run(ctx, args, stdin, stdout, stderr)
}

func (command Command) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return apperr.ExitInvalidInput
	}
	switch args[0] {
	case "version":
		if len(args) != 1 {
			return invalidUsage(stderr)
		}
		fmt.Fprintf(stdout, "intent-sh %s (adapter protocol %s)\n", Version, protocol.AdapterVersion)
		return apperr.ExitOK
	case "help", "--help", "-h":
		if len(args) != 1 {
			return invalidUsage(stderr)
		}
		usage(stdout)
		return apperr.ExitOK
	case "adapter":
		return command.runAdapter(ctx, args[1:], stdin, stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "setup":
		return runSetup(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "doctor":
		return command.runDoctor(ctx, args[1:], stdout, stderr)
	default:
		return invalidUsage(stderr)
	}
}

func (command Command) runAdapter(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 3 || args[0] != "rewrite" || args[1] != "--protocol" {
		return invalidUsage(stderr)
	}
	if args[2] != protocol.AdapterVersion {
		err := apperr.New(apperr.KindProtocol, "start adapter rewrite", "adapter protocol is incompatible with binary protocol "+protocol.AdapterVersion)
		if encodeErr := writeCLIErrorFrame(stdout, err); encodeErr != nil {
			writeError(stderr, encodeErr)
			return apperr.ExitCode(encodeErr)
		}
		return apperr.ExitCode(err)
	}
	service := app.DefaultService()
	if command.Service != nil {
		service = *command.Service
	}
	err := service.HandleRewrite(ctx, stdin, stdout)
	if err != nil {
		return apperr.ExitCode(err)
	}
	return apperr.ExitOK
}

func runInit(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		return invalidUsage(stderr)
	}
	cfg, _, err := config.Load()
	if err != nil {
		writeError(stderr, err)
		return apperr.ExitCode(err)
	}
	script, err := shellassets.ScriptWithBindings(args[0], protocol.AdapterVersion, cfg.RewriteKey, cfg.UndoKey)
	if err != nil {
		writeError(stderr, err)
		return apperr.ExitCode(err)
	}
	if _, err := io.WriteString(stdout, script); err != nil {
		wrapped := apperr.Wrap(apperr.KindInternal, "print adapter", "could not print the shell adapter", err)
		writeError(stderr, wrapped)
		return apperr.ExitCode(wrapped)
	}
	return apperr.ExitOK
}

func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return invalidUsage(stderr)
	}
	switch args[0] {
	case "path":
		if len(args) != 1 {
			return invalidUsage(stderr)
		}
		path, err := config.Path()
		if err != nil {
			writeError(stderr, err)
			return apperr.ExitCode(err)
		}
		fmt.Fprintln(stdout, path)
		return apperr.ExitOK
	case "show":
		if len(args) != 1 {
			return invalidUsage(stderr)
		}
		cfg, _, err := config.Load()
		if err != nil {
			writeError(stderr, err)
			return apperr.ExitCode(err)
		}
		data, err := config.Marshal(cfg)
		if err != nil {
			writeError(stderr, err)
			return apperr.ExitCode(err)
		}
		_, _ = stdout.Write(data)
		return apperr.ExitOK
	case "set":
		if len(args) != 3 {
			return invalidUsage(stderr)
		}
		path, err := config.Path()
		if err != nil {
			writeError(stderr, err)
			return apperr.ExitCode(err)
		}
		cfg, err := config.SetAt(path, args[1], args[2])
		if err != nil {
			writeError(stderr, err)
			return apperr.ExitCode(err)
		}
		data, err := config.Marshal(cfg)
		if err != nil {
			writeError(stderr, err)
			return apperr.ExitCode(err)
		}
		_, _ = stdout.Write(data)
		return apperr.ExitOK
	default:
		return invalidUsage(stderr)
	}
}

func runSetup(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "bash" && args[0] != "zsh" {
		return invalidUsage(stderr)
	}
	cfg, _, err := config.Load()
	if err != nil {
		writeError(stderr, err)
		return apperr.ExitCode(err)
	}
	plan, err := setupguide.InspectDefaultWithBindings(args[0], cfg.RewriteKey, cfg.UndoKey)
	if err != nil {
		writeError(stderr, err)
		return apperr.ExitCode(err)
	}
	fmt.Fprintf(stdout, "Shell: %s\nStartup file: %s\n\nAdd this idempotent activation line:\n%s\n\nEffective bindings:\n", plan.Shell, textsafe.Terminal(plan.StartupFile, 4096), plan.Activation)
	for _, binding := range plan.Bindings {
		fmt.Fprintf(stdout, "- %s\n", binding)
	}
	fmt.Fprintln(stdout, "\nBinding diagnostics:")
	fmt.Fprintln(stdout, "- Run `intent-sh doctor --keys` interactively to verify delivery from the controlling terminal.")
	fmt.Fprintln(stdout, "- Change native bindings with `intent-sh config set rewrite_key <chord>` and `undo_key <chord>`, then start a new shell.")
	fmt.Fprintln(stdout, "- Supported chords are one Alt+printable-ASCII key or one unreserved Ctrl+letter; defaults are Alt+G and Alt+U.")
	if plan.Shell == setupguide.ShellBash {
		fmt.Fprintln(stdout, "\nBash requirement:")
		fmt.Fprintln(stdout, "- Bash 4.0 or newer with native Readline is required.")
	}
	if len(plan.Conflicts) > 0 {
		fmt.Fprintln(stdout, "\nWarnings:")
		for _, conflict := range plan.Conflicts {
			backend := conflict.Backend
			if backend == "" {
				backend = setupguide.ConflictBackendNative
			}
			fmt.Fprintf(stdout, "- %s already has a custom %s %s binding; review it before activation.\n", plan.Shell, backend, conflict.Key)
		}
	}
	fmt.Fprintf(stdout, "\nRemoval: delete this exact line from %s:\n%s\n", textsafe.Terminal(plan.StartupFile, 4096), plan.Activation)
	fmt.Fprintln(stdout, "\nNo startup file was modified.")
	return apperr.ExitOK
}

func (command Command) runDoctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	withKeys := len(args) == 1 && args[0] == "--keys"
	if len(args) != 0 && !withKeys {
		return invalidUsage(stderr)
	}
	runner := command.Doctor
	if runner == nil {
		defaultRunner := doctor.NewDefault()
		runner = defaultRunner
	}
	var report doctor.Report
	if withKeys {
		keyRunner, ok := runner.(DoctorKeysRunner)
		if !ok {
			err := apperr.New(apperr.KindInternal, "run doctor key probe", "interactive key diagnostics were not initialized")
			writeError(stderr, err)
			return apperr.ExitCode(err)
		}
		report = keyRunner.RunKeys(ctx)
	} else {
		report = runner.Run(ctx)
	}
	doctor.Render(stdout, report)
	if report.Ready {
		return apperr.ExitOK
	}
	kind := report.FailureKind
	if kind == "" {
		kind = apperr.KindInternal
	}
	return apperr.ExitCode(apperr.New(kind, "run doctor", "local readiness checks failed"))
}

func writeCLIErrorFrame(stdout io.Writer, err error) error {
	response := app.ErrorResponse(protocol.AdapterRequest{}, err)
	var buffer bytes.Buffer
	if encodeErr := protocol.EncodeResponse(&buffer, response); encodeErr != nil {
		return encodeErr
	}
	if _, writeErr := io.Copy(stdout, &buffer); writeErr != nil {
		return apperr.Wrap(apperr.KindInternal, "write adapter response", "could not write the adapter response", writeErr)
	}
	return nil
}

func writeError(w io.Writer, err error) {
	message := textsafe.Terminal(strings.TrimSpace(apperr.Message(err)), 1024)
	if message == "" {
		message = "intent-sh encountered an internal error"
	}
	fmt.Fprintf(w, "intent-sh: %s\n", message)
}

func invalidUsage(stderr io.Writer) int {
	usage(stderr)
	return apperr.ExitInvalidInput
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: intent-sh <adapter|init|setup|config|doctor [--keys]|version>")
	fmt.Fprintln(w, "  doctor         run non-interactive local readiness checks")
	fmt.Fprintln(w, "  doctor --keys  temporarily read bounded keys from /dev/tty; no provider is invoked")
	fmt.Fprintln(w, "  setup bash|zsh print read-only activation, effective bindings, conflicts, and removal guidance")
}
