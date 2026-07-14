package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/config"
	contextinfo "github.com/taiwong148960/intent-sh/internal/context"
	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/internal/provider"
	"github.com/taiwong148960/intent-sh/internal/safety"
)

type stubContextBuilder struct {
	gotShell, gotVersion, gotCWD string
	environment                  contextinfo.Environment
}

func (b *stubContextBuilder) Build(shell, version, cwd string) contextinfo.Environment {
	b.gotShell, b.gotVersion, b.gotCWD = shell, version, cwd
	return b.environment
}

type stubRouter struct {
	result       provider.Result
	err          error
	calls        int
	mode         string
	priority     []string
	request      provider.Request
	beforeReturn func()
}

func (r *stubRouter) Route(_ context.Context, mode string, priority []string, request provider.Request) (provider.Result, error) {
	r.calls++
	r.mode = mode
	r.priority = append([]string(nil), priority...)
	r.request = request
	if r.beforeReturn != nil {
		r.beforeReturn()
	}
	return r.result, r.err
}

type stubSafety struct {
	decision             safety.Decision
	err                  error
	calls                int
	command, shell, hint string
	beforeReturn         func()
}

func (s *stubSafety) Evaluate(_ context.Context, command, shell, hint string) (safety.Decision, error) {
	s.calls++
	s.command, s.shell, s.hint = command, shell, hint
	if s.beforeReturn != nil {
		s.beforeReturn()
	}
	return s.decision, s.err
}

func TestRewriteOrchestratesSuccessfulCommand(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	cfg.Provider = config.ProviderAuto
	cfg.Priority = []string{config.ProviderCodex, config.ProviderClaude}
	cfg.TimeoutSeconds = 17
	cfg.Model = "configured-model"
	cfg.RewriteKey = "alt+~"
	cfg.UndoKey = "ctrl+x"
	contextBuilder := &stubContextBuilder{environment: contextinfo.Environment{
		OS: "linux", Arch: "arm64", Shell: "zsh", ShellVersion: "5.9", CWD: "/work", AvailableTools: []string{"rg"},
	}}
	router := &stubRouter{result: provider.Result{Provider: provider.NameCodex, Value: protocol.ProviderResult{
		Status: protocol.ProviderStatusOK, Command: "rg TODO .", Explanation: "searches for TODO", RiskHint: "safe",
	}}}
	evaluator := &stubSafety{decision: safety.Decision{
		Command: "rg TODO .", Level: safety.LevelSafe, ReasonCode: safety.ReasonKnownReadOnly, Reason: "no known risky pattern matched",
	}}
	service := Service{
		LoadConfig: func() (config.Config, string, error) { return cfg, "/config", nil },
		Context:    contextBuilder,
		Router:     router,
		Safety:     evaluator,
		Getwd:      func() (string, error) { return "/work", nil },
	}
	request := validRequest()
	response, err := service.Rewrite(context.Background(), request)
	if err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}
	if response.Status != protocol.StatusOK || response.Replacement != "rg TODO ." || response.Provider != provider.NameCodex || response.Risk != "safe" {
		t.Fatalf("response = %#v", response)
	}
	if response.RequestID != request.RequestID || response.Message != "searches for TODO" || response.RiskReason == "" {
		t.Fatalf("response metadata = %#v", response)
	}
	if router.calls != 1 || router.mode != cfg.Provider || strings.Join(router.priority, ",") != "codex,claude" {
		t.Fatalf("router contract: calls=%d mode=%q priority=%#v", router.calls, router.mode, router.priority)
	}
	if router.request.Model != cfg.Model || router.request.Timeout != 17*time.Second || !strings.Contains(router.request.Prompt, `"buffer":"find TODOs"`) {
		t.Fatalf("provider request = %#v", router.request)
	}
	for _, prohibited := range []string{"editorBackend", protocol.BleshVersion, cfg.RewriteKey, cfg.UndoKey, "rewriteKey", "undoKey", "TERM_PROGRAM"} {
		if strings.Contains(router.request.Prompt, prohibited) {
			t.Fatalf("local binding or terminal metadata %q reached the model prompt: %q", prohibited, router.request.Prompt)
		}
	}
	if !strings.Contains(router.request.Prompt, `"availableTools":["rg"]`) || contextBuilder.gotCWD != "/work" {
		t.Fatalf("context was not included correctly: prompt=%q context=%#v", router.request.Prompt, contextBuilder)
	}
	if evaluator.calls != 1 || evaluator.command != "rg TODO ." || evaluator.shell != "zsh" || evaluator.hint != "safe" {
		t.Fatalf("safety contract = %#v", evaluator)
	}
}

func TestRewriteReturnsClarificationWithoutSafetyCall(t *testing.T) {
	t.Parallel()
	router := &stubRouter{result: provider.Result{Provider: provider.NameClaude, Value: protocol.ProviderResult{
		Status: protocol.ProviderStatusClarify, Question: "Which directory?",
	}}}
	evaluator := &stubSafety{}
	service := testService(router, evaluator)
	response, err := service.Rewrite(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}
	if response.Status != protocol.StatusClarify || response.Message != "Which directory?" || response.Replacement != "" {
		t.Fatalf("response = %#v", response)
	}
	if evaluator.calls != 0 {
		t.Fatalf("safety was called %d times for clarification", evaluator.calls)
	}
}

func TestValidateRequestFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*protocol.AdapterRequest)
		kind   apperr.Kind
	}{
		{"version", func(r *protocol.AdapterRequest) { r.Version = "9" }, apperr.KindProtocol},
		{"action", func(r *protocol.AdapterRequest) { r.Action = "execute" }, apperr.KindInvalidInput},
		{"shell", func(r *protocol.AdapterRequest) { r.Shell = "fish" }, apperr.KindInvalidInput},
		{"shell version", func(r *protocol.AdapterRequest) { r.ShellVersion = "" }, apperr.KindInvalidInput},
		{"missing backend", func(r *protocol.AdapterRequest) { r.EditorBackend = "" }, apperr.KindInvalidInput},
		{"missing editor version", func(r *protocol.AdapterRequest) { r.EditorVersion = "" }, apperr.KindInvalidInput},
		{"zsh wrong backend", func(r *protocol.AdapterRequest) { r.EditorBackend = protocol.EditorBackendReadline }, apperr.KindProtocol},
		{"zsh mismatched editor version", func(r *protocol.AdapterRequest) { r.EditorVersion = "5.8" }, apperr.KindProtocol},
		{"old native bash", func(r *protocol.AdapterRequest) {
			r.Shell, r.ShellVersion = "bash", "3.2.57(1)-release"
			r.EditorBackend, r.EditorVersion = protocol.EditorBackendReadline, "3.2.57(1)-release"
		}, apperr.KindProtocol},
		{"blesh on bash 3.1", func(r *protocol.AdapterRequest) {
			r.Shell, r.ShellVersion = "bash", "3.1.23(1)-release"
			r.EditorBackend, r.EditorVersion = protocol.EditorBackendBlesh, protocol.BleshVersion
		}, apperr.KindProtocol},
		{"unverified blesh", func(r *protocol.AdapterRequest) {
			r.Shell, r.ShellVersion = "bash", "3.2.57(1)-release"
			r.EditorBackend, r.EditorVersion = protocol.EditorBackendBlesh, "0.4.0-devel3"
		}, apperr.KindProtocol},
		{"unknown bash backend", func(r *protocol.AdapterRequest) {
			r.Shell, r.ShellVersion = "bash", "5.2.37(1)-release"
			r.EditorBackend, r.EditorVersion = "other", "1"
		}, apperr.KindProtocol},
		{"empty input", func(r *protocol.AdapterRequest) { r.Buffer, r.Cursor = "  ", 0 }, apperr.KindInvalidInput},
		{"cursor", func(r *protocol.AdapterRequest) { r.Cursor = len(r.Buffer) + 1 }, apperr.KindInvalidInput},
		{"cursor UTF-8 boundary", func(r *protocol.AdapterRequest) { r.Buffer, r.Cursor = "a中b", 2 }, apperr.KindInvalidInput},
		{"invalid UTF-8", func(r *protocol.AdapterRequest) { r.Buffer, r.Cursor = string([]byte{'a', 0xff}), 1 }, apperr.KindInvalidInput},
		{"request ID", func(r *protocol.AdapterRequest) { r.RequestID = "" }, apperr.KindInvalidInput},
		{"regeneration original", func(r *protocol.AdapterRequest) { r.GenerationIndex, r.Original, r.Previous = 1, "", "pwd" }, apperr.KindInvalidInput},
		{"regeneration previous", func(r *protocol.AdapterRequest) { r.GenerationIndex, r.Original, r.Previous = 1, "intent", "" }, apperr.KindInvalidInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validRequest()
			test.mutate(&request)
			err := ValidateRequest(request)
			if got := apperr.KindOf(err); got != test.kind {
				t.Fatalf("kind = %q, want %q; err=%v", got, test.kind, err)
			}
		})
	}
}

func TestValidateRequestAcceptsCoherentEditorBackends(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*protocol.AdapterRequest)
	}{
		{name: "zle", mutate: func(*protocol.AdapterRequest) {}},
		{name: "native readline", mutate: func(r *protocol.AdapterRequest) {
			r.Shell, r.ShellVersion = "bash", "4.0.44(1)-release"
			r.EditorBackend, r.EditorVersion = protocol.EditorBackendReadline, r.ShellVersion
		}},
		{name: "Bash 3.2 ble.sh", mutate: func(r *protocol.AdapterRequest) {
			r.Shell, r.ShellVersion = "bash", "3.2.57(1)-release"
			r.EditorBackend, r.EditorVersion = protocol.EditorBackendBlesh, protocol.BleshVersion
		}},
		{name: "modern Bash ble.sh", mutate: func(r *protocol.AdapterRequest) {
			r.Shell, r.ShellVersion = "bash", "5.2.37(1)-release"
			r.EditorBackend, r.EditorVersion = protocol.EditorBackendBlesh, protocol.BleshVersion
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validRequest()
			test.mutate(&request)
			if err := ValidateRequest(request); err != nil {
				t.Fatalf("ValidateRequest() error = %v", err)
			}
		})
	}
}

func TestRewriteRejectsIncoherentBackendBeforeProvider(t *testing.T) {
	t.Parallel()
	router := &stubRouter{}
	request := validRequest()
	request.EditorBackend = protocol.EditorBackendReadline
	_, err := testService(router, &stubSafety{}).Rewrite(context.Background(), request)
	if apperr.KindOf(err) != apperr.KindProtocol {
		t.Fatalf("kind = %q; err=%v", apperr.KindOf(err), err)
	}
	if router.calls != 0 {
		t.Fatalf("provider was called %d times", router.calls)
	}
}

func TestHandleRewriteFailureFlowsNeverEmitReplacement(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		routerErr  error
		safetyErr  error
		wantKind   apperr.Kind
		wantStatus string
	}{
		{"provider unavailable", apperr.New(apperr.KindProviderUnavailable, "fake", "provider unavailable"), nil, apperr.KindProviderUnavailable, protocol.StatusError},
		{"timeout", apperr.New(apperr.KindTimeout, "fake", "provider timed out"), nil, apperr.KindTimeout, protocol.StatusError},
		{"cancel", apperr.New(apperr.KindCancelled, "fake", "cancelled"), nil, apperr.KindCancelled, protocol.StatusCancel},
		{"invalid provider output", apperr.New(apperr.KindProviderOutput, "fake", "invalid output"), nil, apperr.KindProviderOutput, protocol.StatusError},
		{"safety rejection", nil, apperr.New(apperr.KindSafety, "fake", "unsafe shape"), apperr.KindSafety, protocol.StatusError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			router := &stubRouter{result: provider.Result{Provider: provider.NameCodex, Value: protocol.ProviderResult{
				Status: protocol.ProviderStatusOK, Command: "replacement-sentinel", RiskHint: "safe",
			}}, err: test.routerErr}
			evaluator := &stubSafety{decision: safety.Decision{Command: "replacement-sentinel", Level: safety.LevelSafe}, err: test.safetyErr}
			service := testService(router, evaluator)
			var input, output bytes.Buffer
			if err := protocol.EncodeRequest(&input, validRequest()); err != nil {
				t.Fatal(err)
			}
			err := service.HandleRewrite(context.Background(), &input, &output)
			if got := apperr.KindOf(err); got != test.wantKind {
				t.Fatalf("kind = %q, want %q; err=%v", got, test.wantKind, err)
			}
			response, decodeErr := protocol.DecodeResponse(&output)
			if decodeErr != nil {
				t.Fatalf("DecodeResponse() error = %v", decodeErr)
			}
			if response.Status != test.wantStatus || response.Replacement != "" || strings.Contains(response.Message, "replacement-sentinel") {
				t.Fatalf("failure response leaked a replacement: %#v", response)
			}
		})
	}
}

func TestHandleRewriteBuffersUntilAllChecksFinish(t *testing.T) {
	t.Parallel()
	var input, output bytes.Buffer
	if err := protocol.EncodeRequest(&input, validRequest()); err != nil {
		t.Fatal(err)
	}
	router := &stubRouter{result: provider.Result{Provider: provider.NameCodex, Value: protocol.ProviderResult{
		Status: protocol.ProviderStatusOK, Command: "pwd", Explanation: "directory", RiskHint: "safe",
	}}}
	evaluator := &stubSafety{decision: safety.Decision{Command: "pwd", Level: safety.LevelSafe, Reason: "known read-only"}}
	router.beforeReturn = func() {
		if output.Len() != 0 {
			t.Errorf("response bytes were written before provider routing completed")
		}
	}
	evaluator.beforeReturn = func() {
		if output.Len() != 0 {
			t.Errorf("response bytes were written before safety completed")
		}
	}
	if err := testService(router, evaluator).HandleRewrite(context.Background(), &input, &output); err != nil {
		t.Fatalf("HandleRewrite() error = %v", err)
	}
	response, err := protocol.DecodeResponse(&output)
	if err != nil || response.Replacement != "pwd" {
		t.Fatalf("response = %#v, %v", response, err)
	}
}

func TestHandleRewriteMalformedFrameReturnsFramedError(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := (Service{}).HandleRewrite(context.Background(), strings.NewReader("not-a-frame"), &output)
	if apperr.KindOf(err) != apperr.KindProtocol {
		t.Fatalf("kind = %q, want protocol", apperr.KindOf(err))
	}
	response, decodeErr := protocol.DecodeResponse(&output)
	if decodeErr != nil {
		t.Fatalf("framed error did not decode: %v", decodeErr)
	}
	if response.Status != protocol.StatusError || response.Replacement != "" {
		t.Fatalf("response = %#v", response)
	}
}

func TestRewriteConfigAndWorkingDirectoryFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		service Service
		kind    apperr.Kind
	}{
		{
			name: "config",
			service: Service{LoadConfig: func() (config.Config, string, error) {
				return config.Config{}, "", apperr.New(apperr.KindConfiguration, "fake", "bad config")
			}},
			kind: apperr.KindConfiguration,
		},
		{
			name: "cwd",
			service: Service{
				LoadConfig: func() (config.Config, string, error) { return config.Defaults(), "", nil },
				Getwd:      func() (string, error) { return "", errors.New("cwd gone") },
			},
			kind: apperr.KindInvalidInput,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := test.service.Rewrite(context.Background(), validRequest())
			if apperr.KindOf(err) != test.kind {
				t.Fatalf("kind = %q, want %q; err=%v", apperr.KindOf(err), test.kind, err)
			}
		})
	}
}

func TestErrorResponseRedactsPromptAndInternalCauses(t *testing.T) {
	t.Parallel()
	secret := "SECRET_PROMPT_BODY_SENTINEL"
	router := &stubRouter{err: apperr.Wrap(apperr.KindProviderUnavailable, "fake", "provider unavailable", errors.New(secret))}
	service := testService(router, &stubSafety{})
	request := validRequest()
	request.Buffer = secret
	request.Cursor = len(secret)
	var input, output bytes.Buffer
	if err := protocol.EncodeRequest(&input, request); err != nil {
		t.Fatal(err)
	}
	err := service.HandleRewrite(context.Background(), &input, &output)
	if apperr.KindOf(err) != apperr.KindProviderUnavailable {
		t.Fatalf("kind = %q", apperr.KindOf(err))
	}
	response, decodeErr := protocol.DecodeResponse(&output)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if strings.Contains(response.Message, secret) || response.Replacement != "" {
		t.Fatalf("error response leaked private input: %#v", response)
	}
}

func validRequest() protocol.AdapterRequest {
	return protocol.AdapterRequest{
		Version: protocol.AdapterVersion, Action: protocol.ActionRewrite,
		Shell: "zsh", ShellVersion: "5.9", EditorBackend: protocol.EditorBackendZLE, EditorVersion: "5.9",
		Buffer: "find TODOs", Cursor: len("find TODOs"), RequestID: "request-1",
	}
}

func testService(router *stubRouter, evaluator *stubSafety) Service {
	return Service{
		LoadConfig: func() (config.Config, string, error) { return config.Defaults(), "/config", nil },
		Context:    &stubContextBuilder{environment: contextinfo.Environment{OS: "linux", Arch: "amd64", Shell: "zsh", ShellVersion: "5.9", CWD: "/work"}},
		Router:     router,
		Safety:     evaluator,
		Getwd:      func() (string, error) { return "/work", nil },
	}
}
