package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taiwong148960/intent-sh/internal/app"
	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/config"
	contextinfo "github.com/taiwong148960/intent-sh/internal/context"
	"github.com/taiwong148960/intent-sh/internal/doctor"
	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/internal/provider"
	"github.com/taiwong148960/intent-sh/internal/safety"
)

type cliRouter struct {
	result provider.Result
	err    error
}

func (r cliRouter) Route(context.Context, string, []string, provider.Request) (provider.Result, error) {
	return r.result, r.err
}

type cliSafety struct {
	decision safety.Decision
	err      error
}

type cliDoctor struct {
	report doctor.Report
}

func (item cliDoctor) Run(context.Context) doctor.Report { return item.report }

func (s cliSafety) Evaluate(context.Context, string, string, string) (safety.Decision, error) {
	return s.decision, s.err
}

func TestVersionHelpAndInvalidDispatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		args       []string
		wantExit   int
		wantOutput string
		inStderr   bool
	}{
		{"version", []string{"version"}, apperr.ExitOK, "adapter protocol " + protocol.AdapterVersion, false},
		{"help", []string{"--help"}, apperr.ExitOK, "usage: intent-sh", false},
		{"empty", nil, apperr.ExitInvalidInput, "usage: intent-sh", true},
		{"unknown", []string{"unknown"}, apperr.ExitInvalidInput, "usage: intent-sh", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			exit := Run(test.args, strings.NewReader(""), &stdout, &stderr)
			if exit != test.wantExit {
				t.Fatalf("exit = %d, want %d", exit, test.wantExit)
			}
			output := stdout.String()
			if test.inStderr {
				output = stderr.String()
			}
			if !strings.Contains(output, test.wantOutput) {
				t.Fatalf("output = %q, want %q", output, test.wantOutput)
			}
		})
	}
}

func TestInitPrintsEmbeddedVersionMatchedAdapters(t *testing.T) {
	t.Parallel()
	for _, shell := range []string{"bash", "zsh"} {
		shell := shell
		t.Run(shell, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			exit := Run([]string{"init", shell}, strings.NewReader(""), &stdout, &stderr)
			if exit != apperr.ExitOK || stderr.Len() != 0 {
				t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
			}
			if !strings.Contains(stdout.String(), "__intent_sh_protocol_version="+protocol.AdapterVersion) {
				t.Fatalf("adapter output omitted version marker: %q", stdout.String())
			}
		})
	}
}

func TestAdapterRewriteCommandReturnsFramedSuccess(t *testing.T) {
	t.Parallel()
	service := cliService(
		cliRouter{result: provider.Result{Provider: provider.NameCodex, Value: protocol.ProviderResult{
			Status: protocol.ProviderStatusOK, Command: "pwd", Explanation: "directory", RiskHint: "safe",
		}}},
		cliSafety{decision: safety.Decision{Command: "pwd", Level: safety.LevelSafe, Reason: "known read-only"}},
	)
	request := protocol.AdapterRequest{
		Version: protocol.AdapterVersion, Action: protocol.ActionRewrite, Shell: "zsh", ShellVersion: "5.9",
		Buffer: "where am I", Cursor: len("where am I"), RequestID: "cli-request",
	}
	var stdin, stdout, stderr bytes.Buffer
	if err := protocol.EncodeRequest(&stdin, request); err != nil {
		t.Fatal(err)
	}
	exit := (Command{Service: &service}).Run(context.Background(), []string{"adapter", "rewrite", "--protocol", protocol.AdapterVersion}, &stdin, &stdout, &stderr)
	if exit != apperr.ExitOK || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	response, err := protocol.DecodeResponse(&stdout)
	if err != nil {
		t.Fatalf("DecodeResponse() error = %v", err)
	}
	if response.Status != protocol.StatusOK || response.Replacement != "pwd" || response.RequestID != request.RequestID {
		t.Fatalf("response = %#v", response)
	}
}

func TestAdapterRewriteCommandFramesFailures(t *testing.T) {
	t.Parallel()
	t.Run("protocol mismatch", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		exit := (Command{}).Run(context.Background(), []string{"adapter", "rewrite", "--protocol", "999"}, strings.NewReader(""), &stdout, &stderr)
		if exit != apperr.ExitProtocol {
			t.Fatalf("exit = %d, want protocol", exit)
		}
		response, err := protocol.DecodeResponse(&stdout)
		if err != nil || response.Status != protocol.StatusError || response.Replacement != "" {
			t.Fatalf("response=%#v err=%v", response, err)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		service := cliService(cliRouter{err: apperr.New(apperr.KindCancelled, "fake", "cancelled")}, cliSafety{})
		request := protocol.AdapterRequest{
			Version: protocol.AdapterVersion, Action: protocol.ActionRewrite, Shell: "zsh", ShellVersion: "5.9",
			Buffer: "intent", Cursor: 6, RequestID: "cancel-request",
		}
		var stdin, stdout, stderr bytes.Buffer
		if err := protocol.EncodeRequest(&stdin, request); err != nil {
			t.Fatal(err)
		}
		exit := (Command{Service: &service}).Run(context.Background(), []string{"adapter", "rewrite", "--protocol", protocol.AdapterVersion}, &stdin, &stdout, &stderr)
		if exit != apperr.ExitCancelled {
			t.Fatalf("exit = %d, want cancelled", exit)
		}
		response, err := protocol.DecodeResponse(&stdout)
		if err != nil || response.Status != protocol.StatusCancel || response.Replacement != "" {
			t.Fatalf("response=%#v err=%v", response, err)
		}
	})
}

func TestConfigCommands(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	var stdout, stderr bytes.Buffer
	if exit := Run([]string{"config", "path"}, strings.NewReader(""), &stdout, &stderr); exit != apperr.ExitOK {
		t.Fatalf("config path exit=%d stderr=%q", exit, stderr.String())
	}
	wantPath := filepath.Join(xdg, "intent-sh", "config.toml")
	if strings.TrimSpace(stdout.String()) != wantPath {
		t.Fatalf("path = %q, want %q", stdout.String(), wantPath)
	}

	stdout.Reset()
	stderr.Reset()
	if exit := Run([]string{"config", "show"}, strings.NewReader(""), &stdout, &stderr); exit != apperr.ExitOK {
		t.Fatalf("config show exit=%d stderr=%q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `provider = 'auto'`) {
		t.Fatalf("default config = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exit := Run([]string{"config", "set", "provider", "codex"}, strings.NewReader(""), &stdout, &stderr); exit != apperr.ExitOK {
		t.Fatalf("config set exit=%d stderr=%q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `provider = 'codex'`) {
		t.Fatalf("effective config = %q", stdout.String())
	}
	loaded, err := config.LoadAt(wantPath)
	if err != nil || loaded.Provider != config.ProviderCodex {
		t.Fatalf("persisted config=%#v err=%v", loaded, err)
	}
}

func TestSetupPrintsReversibleGuidanceWithoutWriting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	exit := Run([]string{"setup", "zsh"}, strings.NewReader(""), &stdout, &stderr)
	if exit != apperr.ExitOK || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `eval "$(intent-sh init zsh)"`) || !strings.Contains(stdout.String(), "No startup file was modified") {
		t.Fatalf("guidance = %q", stdout.String())
	}
	if _, err := config.LoadAt(filepath.Join(home, ".zshrc")); err != nil {
		t.Fatalf("setup unexpectedly affected startup path: %v", err)
	}
}

func TestDoctorCommandUsesReportOutcome(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		report   doctor.Report
		wantExit int
		want     string
	}{
		{"ready", doctor.Report{Ready: true, Checks: []doctor.Check{{Status: doctor.StatusPass, ID: "provider.ready", Detail: "ready"}}}, apperr.ExitOK, "READY intent-sh"},
		{"not ready", doctor.Report{FailureKind: apperr.KindProviderUnavailable, Checks: []doctor.Check{{Status: doctor.StatusFail, ID: "provider.ready", Detail: "not ready"}}}, apperr.ExitProviderUnavailable, "NOT_READY"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := (Command{Doctor: cliDoctor{report: test.report}}).Run(context.Background(), []string{"doctor"}, strings.NewReader(""), &stdout, &stderr)
			if exit != test.wantExit || stderr.Len() != 0 || !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCLIErrorOutputIsBoundedAndControlSafe(t *testing.T) {
	t.Parallel()
	secretCause := "SECRET_INTERNAL_CAUSE_SENTINEL"
	err := apperr.Wrap(apperr.KindConfiguration, "fake", "safe\x1b[31m\n"+strings.Repeat("x", 4000), errors.New(secretCause))
	var output bytes.Buffer
	writeError(&output, err)
	if strings.Contains(output.String(), secretCause) || strings.ContainsAny(output.String(), "\x1b") {
		t.Fatalf("unsafe CLI error = %q", output.String())
	}
	if output.Len() > 1100 {
		t.Fatalf("CLI error was not bounded: %d bytes", output.Len())
	}

	var framed, stderr bytes.Buffer
	protocolSecret := "SECRET_PROTOCOL_FIELD_SENTINEL"
	exit := (Command{}).Run(context.Background(), []string{"adapter", "rewrite", "--protocol", protocolSecret}, strings.NewReader(""), &framed, &stderr)
	if exit != apperr.ExitProtocol {
		t.Fatalf("exit = %d", exit)
	}
	response, decodeErr := protocol.DecodeResponse(&framed)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if strings.Contains(response.Message, protocolSecret) {
		t.Fatalf("protocol input leaked: %#v", response)
	}
}

func cliService(router cliRouter, evaluator cliSafety) app.Service {
	builder := contextinfo.NewBuilder()
	return app.Service{
		LoadConfig: func() (config.Config, string, error) { return config.Defaults(), "", nil },
		Context:    builder,
		Router:     router,
		Safety:     evaluator,
		Getwd:      func() (string, error) { return "/tmp", nil },
	}
}
