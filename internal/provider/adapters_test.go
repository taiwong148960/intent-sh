package provider

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/schemas"
)

const (
	validOK      = `{"status":"ok","command":"pwd","explanation":"prints the directory","assumptions":[],"riskHint":"safe"}`
	validClarify = `{"status":"clarify","question":"Which directory?"}`
)

type runnerResponse struct {
	result RunResult
	err    error
}

type recordedInvocation struct {
	Invocation
	args    []string
	workDir string
}

type scriptedRunner struct {
	responses []runnerResponse
	calls     []recordedInvocation
}

func (r *scriptedRunner) Run(_ context.Context, invocation Invocation) (RunResult, error) {
	workDir := "/tmp/intent-sh-adapter-test"
	args := []string(nil)
	if invocation.Args != nil {
		args = invocation.Args(workDir)
	}
	copyInvocation := invocation
	copyInvocation.Stdin = append([]byte(nil), invocation.Stdin...)
	copyInvocation.Files = cloneFiles(invocation.Files)
	r.calls = append(r.calls, recordedInvocation{Invocation: copyInvocation, args: args, workDir: workDir})
	index := len(r.calls) - 1
	if index >= len(r.responses) {
		return RunResult{}, errors.New("unexpected runner call")
	}
	return r.responses[index].result, r.responses[index].err
}

func cloneFiles(source map[string][]byte) map[string][]byte {
	if source == nil {
		return nil
	}
	result := make(map[string][]byte, len(source))
	for name, data := range source {
		result[name] = append([]byte(nil), data...)
	}
	return result
}

func TestClaudeGenerateContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		response   runnerResponse
		wantStatus string
		wantKind   apperr.Kind
	}{
		{
			name: "success envelope",
			response: runnerResponse{result: RunResult{Stdout: []byte(
				`{"type":"result","is_error":false,"structured_output":` + validOK + `,"session_id":"discarded"}`,
			)}},
			wantStatus: protocol.ProviderStatusOK,
		},
		{
			name:       "clarify direct",
			response:   runnerResponse{result: RunResult{Stdout: []byte(validClarify)}},
			wantStatus: protocol.ProviderStatusClarify,
		},
		{
			name:     "auth failure",
			response: runnerResponse{result: RunResult{Stderr: []byte("Authentication required")}, err: &ExitError{Code: 1}},
			wantKind: apperr.KindProviderUnavailable,
		},
		{
			name:     "malformed output",
			response: runnerResponse{result: RunResult{Stdout: []byte(`{"result":"not json"}`)}},
			wantKind: apperr.KindProviderOutput,
		},
		{
			name:     "duplicate envelope field",
			response: runnerResponse{result: RunResult{Stdout: []byte(`{"is_error":true,"is_error":false,"structured_output":` + validOK + `}`)}},
			wantKind: apperr.KindProviderOutput,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runner := &scriptedRunner{responses: []runnerResponse{test.response}}
			adapter := Claude{Runner: runner, Program: "/opt/bin/claude"}
			request := Request{Prompt: "PROMPT_SENTINEL", Model: "test-model", Timeout: 7 * time.Second}
			value, err := adapter.Generate(context.Background(), request)
			if test.wantKind != "" {
				if got := apperr.KindOf(err); got != test.wantKind {
					t.Fatalf("kind = %q, want %q; err=%v", got, test.wantKind, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Generate() error = %v", err)
				}
				if value.Status != test.wantStatus {
					t.Fatalf("status = %q, want %q", value.Status, test.wantStatus)
				}
			}
			call := runner.calls[0]
			if call.Program != "/opt/bin/claude" || string(call.Stdin) != request.Prompt || call.Timeout != request.Timeout {
				t.Fatalf("invocation did not preserve program/stdin/timeout: %#v", call)
			}
			for _, pair := range [][2]string{{"--tools", ""}, {"--output-format", "json"}, {"--model", "test-model"}} {
				if !hasArgPair(call.args, pair[0], pair[1]) {
					t.Fatalf("args %#v omitted pair %#v", call.args, pair)
				}
			}
			for _, flag := range []string{"-p", "--bare", "--strict-mcp-config", "--no-session-persistence", "--json-schema"} {
				if !hasArg(call.args, flag) {
					t.Fatalf("args %#v omitted %s", call.args, flag)
				}
			}
			if hasArg(call.args, request.Prompt) {
				t.Fatal("prompt was exposed as a process argument")
			}
		})
	}
}

func TestClaudeProbeChecksCapabilitiesAndLogin(t *testing.T) {
	t.Parallel()
	help := strings.Join(claudeRequiredFlags, "\n")
	runner := &scriptedRunner{responses: []runnerResponse{
		{result: RunResult{Stdout: []byte("2.3.4\n")}},
		{result: RunResult{Stdout: []byte(help)}},
		{result: RunResult{Stdout: []byte("logged in")}},
	}}
	got, err := (Claude{Runner: runner}).Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if got.Provider != NameClaude || got.Version != "2.3.4" {
		t.Fatalf("probe = %#v", got)
	}
	wantArgs := [][]string{{"--version"}, {"--help"}, {"auth", "status"}}
	for index, want := range wantArgs {
		if !reflect.DeepEqual(runner.calls[index].args, want) {
			t.Fatalf("call %d args = %#v, want %#v", index, runner.calls[index].args, want)
		}
	}
}

func TestClaudeProbeRejectsMissingIsolationFlag(t *testing.T) {
	t.Parallel()
	runner := &scriptedRunner{responses: []runnerResponse{
		{result: RunResult{Stdout: []byte("old-version")}},
		{result: RunResult{Stdout: []byte("--print --output-format")}},
	}}
	_, err := (Claude{Runner: runner}).Probe(context.Background())
	if apperr.KindOf(err) != apperr.KindProviderUnavailable {
		t.Fatalf("kind = %q, want unavailable", apperr.KindOf(err))
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %d, incompatible CLI should fail before auth", len(runner.calls))
	}
}

func TestCodexGenerateContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		response   runnerResponse
		wantStatus string
		wantKind   apperr.Kind
	}{
		{"success", runnerResponse{result: RunResult{ResultFile: []byte(validOK)}}, protocol.ProviderStatusOK, ""},
		{"clarify", runnerResponse{result: RunResult{ResultFile: []byte(validClarify)}}, protocol.ProviderStatusClarify, ""},
		{"auth failure", runnerResponse{result: RunResult{Stderr: []byte("not logged in")}, err: &ExitError{Code: 1}}, "", apperr.KindProviderUnavailable},
		{"malformed output", runnerResponse{result: RunResult{ResultFile: []byte("chatter")}}, "", apperr.KindProviderOutput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runner := &scriptedRunner{responses: []runnerResponse{test.response}}
			adapter := Codex{Runner: runner, Program: "/opt/bin/codex"}
			request := Request{Prompt: "PROMPT_SENTINEL", Model: "gpt-test", Timeout: 9 * time.Second}
			value, err := adapter.Generate(context.Background(), request)
			if test.wantKind != "" {
				if got := apperr.KindOf(err); got != test.wantKind {
					t.Fatalf("kind = %q, want %q; err=%v", got, test.wantKind, err)
				}
			} else if err != nil || value.Status != test.wantStatus {
				t.Fatalf("Generate() = %#v, %v; want status %q", value, err, test.wantStatus)
			}
			call := runner.calls[0]
			if call.Program != "/opt/bin/codex" || string(call.Stdin) != request.Prompt || call.ResultFile != codexResult {
				t.Fatalf("invalid invocation: %#v", call)
			}
			if !bytes.Equal(call.Files[codexSchema], schemas.ProviderResult) {
				t.Fatal("Codex invocation omitted the embedded result schema")
			}
			for _, pair := range [][2]string{
				{"--sandbox", "read-only"},
				{"--output-schema", call.workDir + "/" + codexSchema},
				{"--output-last-message", call.workDir + "/" + codexResult},
				{"--model", "gpt-test"},
			} {
				if !hasArgPair(call.args, pair[0], pair[1]) {
					t.Fatalf("args %#v omitted pair %#v", call.args, pair)
				}
			}
			for _, flag := range []string{"exec", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--skip-git-repo-check"} {
				if !hasArg(call.args, flag) {
					t.Fatalf("args %#v omitted %s", call.args, flag)
				}
			}
			if call.args[len(call.args)-1] != "-" || hasArg(call.args, request.Prompt) {
				t.Fatal("Codex must receive only the prompt on stdin")
			}
		})
	}
}

func TestCodexProbeChecksCapabilitiesAndLogin(t *testing.T) {
	t.Parallel()
	help := strings.Join(codexRequiredFlags, "\n")
	runner := &scriptedRunner{responses: []runnerResponse{
		{result: RunResult{Stdout: []byte("codex-cli 0.144.3\n")}},
		{result: RunResult{Stdout: []byte(help)}},
		{result: RunResult{Stdout: []byte("Logged in using ChatGPT")}},
	}}
	got, err := (Codex{Runner: runner}).Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if got.Provider != NameCodex || got.Version != "codex-cli 0.144.3" {
		t.Fatalf("probe = %#v", got)
	}
	wantArgs := [][]string{{"--version"}, {"exec", "--help"}, {"login", "status"}}
	for index, want := range wantArgs {
		if !reflect.DeepEqual(runner.calls[index].args, want) {
			t.Fatalf("call %d args = %#v, want %#v", index, runner.calls[index].args, want)
		}
	}
}

func TestProviderSubprocessArgumentsExcludeBindingsAndTerminalMetadata(t *testing.T) {
	t.Parallel()
	argumentSets := [][]string{
		claudeArgs("model-sentinel")("/tmp/provider-work"),
		codexArgs("model-sentinel")("/tmp/provider-work"),
	}
	for _, args := range argumentSets {
		joined := strings.Join(args, "\x00")
		for _, prohibited := range []string{
			"alt+g", "alt+u", "rewrite_key", "undo_key", "rewriteKey", "undoKey",
			"TERM", "TERM_PROGRAM", "WT_SESSION", "TMUX", "SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY",
		} {
			if strings.Contains(joined, prohibited) {
				t.Fatalf("provider arguments %#v contained prohibited metadata %q", args, prohibited)
			}
		}
	}
}

func TestAdapterFailuresDoNotExposePromptOutputOrStderr(t *testing.T) {
	t.Parallel()
	secret := "SECRET_PROVIDER_BOUNDARY_SENTINEL"
	tests := []struct {
		name      string
		run       func(*scriptedRunner) error
		responses []runnerResponse
	}{
		{
			name:      "Claude stderr",
			responses: []runnerResponse{{result: RunResult{Stderr: []byte(secret)}, err: &ExitError{Code: 9, Stderr: secret}}},
			run: func(runner *scriptedRunner) error {
				_, err := (Claude{Runner: runner}).Generate(context.Background(), Request{Prompt: secret, Timeout: time.Second})
				return err
			},
		},
		{
			name:      "Codex raw result",
			responses: []runnerResponse{{result: RunResult{ResultFile: []byte(secret)}}},
			run: func(runner *scriptedRunner) error {
				_, err := (Codex{Runner: runner}).Generate(context.Background(), Request{Prompt: secret, Timeout: time.Second})
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &scriptedRunner{responses: test.responses}
			err := test.run(runner)
			if err == nil {
				t.Fatal("failure fixture unexpectedly succeeded")
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(apperr.Message(err), secret) {
				t.Fatalf("provider boundary leaked sensitive data: %v", err)
			}
		})
	}
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func hasArgPair(args []string, key, value string) bool {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == key && args[index+1] == value {
			return true
		}
	}
	return false
}
