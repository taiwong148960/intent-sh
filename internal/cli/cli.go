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
	script, err := shellassets.Script(args[0], protocol.AdapterVersion)
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
	plan, err := setupguide.InspectDefault(args[0])
	if err != nil {
		writeError(stderr, err)
		return apperr.ExitCode(err)
	}
	fmt.Fprintf(stdout, "Shell: %s\nStartup file: %s\n\nAdd this idempotent activation line:\n%s\n\nDefault bindings:\n", plan.Shell, textsafe.Terminal(plan.StartupFile, 4096), plan.Activation)
	for _, binding := range plan.Bindings {
		fmt.Fprintf(stdout, "- %s\n", binding)
	}
	if plan.Shell == setupguide.ShellBash {
		fmt.Fprintln(stdout, "\nOptional Bash 3.2 compatibility (user managed):")
		fmt.Fprintf(stdout, "- Bash 3.2 requires ble.sh %s from commit %s.\n", plan.BleshVersion, plan.BleshCommit)
		fmt.Fprintf(stdout, "- Install and manage ble.sh separately using the official project: %s\n", plan.BleshInstallURL)
		fmt.Fprintln(stdout, "- Load and attach ble.sh before this activation line; setup never downloads or sources it.")
		fmt.Fprintln(stdout, "- Bash 4.0+ can use native Readline when ble.sh is not attached; stock Zsh is another native alternative.")
		fmt.Fprintln(stdout, "- Review existing ble-bind M-g/M-u bindings and accept-line advice before activation.")
	}
	if len(plan.Conflicts) > 0 || plan.BleshLoadOrderConflict {
		fmt.Fprintln(stdout, "\nWarnings:")
		for _, conflict := range plan.Conflicts {
			backend := conflict.Backend
			if backend == "" {
				backend = setupguide.ConflictBackendNative
			}
			fmt.Fprintf(stdout, "- %s already has a custom %s %s binding; review it before activation.\n", plan.Shell, backend, conflict.Key)
		}
		if plan.BleshLoadOrderConflict {
			fmt.Fprintln(stdout, "- intent-sh appears before ble.sh in the startup file; move the intent-sh activation after ble.sh is attached.")
		}
	}
	fmt.Fprintf(stdout, "\nRemoval: delete this exact line from %s:\n%s\n", textsafe.Terminal(plan.StartupFile, 4096), plan.Activation)
	if plan.Shell == setupguide.ShellBash {
		fmt.Fprintln(stdout, "This removes only intent-sh integration; it does not remove the independently managed ble.sh installation.")
	}
	fmt.Fprintln(stdout, "\nNo startup file was modified.")
	return apperr.ExitOK
}

func (command Command) runDoctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		return invalidUsage(stderr)
	}
	runner := command.Doctor
	if runner == nil {
		defaultRunner := doctor.NewDefault()
		runner = defaultRunner
	}
	report := runner.Run(ctx)
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
	fmt.Fprintln(w, "usage: intent-sh <adapter|init|setup|config|doctor|version>")
}
