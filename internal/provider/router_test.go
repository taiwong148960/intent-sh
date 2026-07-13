package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/config"
	"github.com/taiwong148960/intent-sh/internal/protocol"
)

type providerAttempt struct {
	result protocol.ProviderResult
	err    error
}

type fakeProvider struct {
	name     string
	attempts []providerAttempt
	calls    int
}

func (p *fakeProvider) Name() string { return p.name }
func (p *fakeProvider) Generate(context.Context, Request) (protocol.ProviderResult, error) {
	index := p.calls
	p.calls++
	if index >= len(p.attempts) {
		return protocol.ProviderResult{}, apperr.New(apperr.KindInternal, "fake", "unexpected fake provider call")
	}
	return p.attempts[index].result, p.attempts[index].err
}
func (p *fakeProvider) Probe(context.Context) (ProbeResult, error) {
	return ProbeResult{Provider: p.name, Version: "fake"}, nil
}

func TestRouterStopsOnValidResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		result protocol.ProviderResult
	}{
		{"ok", protocol.ProviderResult{Status: protocol.ProviderStatusOK, Command: "pwd"}},
		{"clarify", protocol.ProviderResult{Status: protocol.ProviderStatusClarify, Question: "where?"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			first := &fakeProvider{name: NameClaude, attempts: []providerAttempt{{result: test.result}}}
			second := &fakeProvider{name: NameCodex, attempts: []providerAttempt{{result: protocol.ProviderResult{Status: protocol.ProviderStatusOK}}}}
			got, err := NewRouter(first, second).Route(context.Background(), config.ProviderAuto, []string{NameClaude, NameCodex}, Request{})
			if err != nil {
				t.Fatalf("Route() error = %v", err)
			}
			if got.Provider != NameClaude || first.calls != 1 || second.calls != 0 {
				t.Fatalf("result=%#v calls=(%d,%d)", got, first.calls, second.calls)
			}
		})
	}
}

func TestRouterAutoFallbackEligibility(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		kind apperr.Kind
	}{
		{"unavailable", apperr.KindProviderUnavailable},
		{"timeout", apperr.KindTimeout},
		{"invalid output", apperr.KindProviderOutput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			first := &fakeProvider{name: NameClaude, attempts: []providerAttempt{{err: apperr.New(test.kind, "fake", "first failed")}}}
			second := &fakeProvider{name: NameCodex, attempts: []providerAttempt{{result: protocol.ProviderResult{Status: protocol.ProviderStatusOK, Command: "pwd"}}}}
			got, err := NewRouter(first, second).Route(context.Background(), config.ProviderAuto, []string{NameClaude, NameCodex}, Request{})
			if err != nil {
				t.Fatalf("Route() error = %v", err)
			}
			if got.Provider != NameCodex || first.calls != 1 || second.calls != 1 {
				t.Fatalf("result=%#v calls=(%d,%d)", got, first.calls, second.calls)
			}
		})
	}
}

func TestRouterFallsBackOnUnsupportedSuccessStatus(t *testing.T) {
	t.Parallel()
	first := &fakeProvider{name: NameClaude, attempts: []providerAttempt{{result: protocol.ProviderResult{Status: "unexpected"}}}}
	second := &fakeProvider{name: NameCodex, attempts: []providerAttempt{{result: protocol.ProviderResult{Status: protocol.ProviderStatusClarify, Question: "where?"}}}}
	got, err := NewRouter(first, second).Route(context.Background(), config.ProviderAuto, []string{NameClaude, NameCodex}, Request{})
	if err != nil || got.Provider != NameCodex {
		t.Fatalf("Route() = %#v, %v", got, err)
	}
}

func TestRouterExplicitModeNeverFallsBack(t *testing.T) {
	t.Parallel()
	first := &fakeProvider{name: NameClaude, attempts: []providerAttempt{{err: apperr.New(apperr.KindTimeout, "fake", "timed out")}}}
	second := &fakeProvider{name: NameCodex, attempts: []providerAttempt{{result: protocol.ProviderResult{Status: protocol.ProviderStatusOK}}}}
	_, err := NewRouter(first, second).Route(context.Background(), NameClaude, []string{NameClaude, NameCodex}, Request{})
	if apperr.KindOf(err) != apperr.KindTimeout || first.calls != 1 || second.calls != 0 {
		t.Fatalf("kind=%q calls=(%d,%d)", apperr.KindOf(err), first.calls, second.calls)
	}
}

func TestRouterCancellationNeverFallsBack(t *testing.T) {
	t.Parallel()
	first := &fakeProvider{name: NameClaude, attempts: []providerAttempt{{err: apperr.New(apperr.KindCancelled, "fake", "cancelled")}}}
	second := &fakeProvider{name: NameCodex, attempts: []providerAttempt{{result: protocol.ProviderResult{Status: protocol.ProviderStatusOK}}}}
	_, err := NewRouter(first, second).Route(context.Background(), config.ProviderAuto, []string{NameClaude, NameCodex}, Request{})
	if apperr.KindOf(err) != apperr.KindCancelled || first.calls != 1 || second.calls != 0 {
		t.Fatalf("kind=%q calls=(%d,%d)", apperr.KindOf(err), first.calls, second.calls)
	}
}

func TestRouterStopsOnNonFallbackInternalError(t *testing.T) {
	t.Parallel()
	first := &fakeProvider{name: NameClaude, attempts: []providerAttempt{{err: errorsForTest("raw internal detail")}}}
	second := &fakeProvider{name: NameCodex, attempts: []providerAttempt{{result: protocol.ProviderResult{Status: protocol.ProviderStatusOK}}}}
	_, err := NewRouter(first, second).Route(context.Background(), config.ProviderAuto, []string{NameClaude, NameCodex}, Request{})
	if apperr.KindOf(err) != apperr.KindInternal || second.calls != 0 {
		t.Fatalf("kind=%q second calls=%d", apperr.KindOf(err), second.calls)
	}
}

func TestRouterAggregatesBoundedDiagnostics(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 500)
	first := &fakeProvider{name: NameClaude, attempts: []providerAttempt{{err: apperr.New(apperr.KindProviderUnavailable, "fake", long)}}}
	second := &fakeProvider{name: NameCodex, attempts: []providerAttempt{{err: apperr.New(apperr.KindProviderOutput, "fake", long)}}}
	_, err := NewRouter(first, second).Route(context.Background(), config.ProviderAuto, []string{NameClaude, NameCodex}, Request{})
	message := apperr.Message(err)
	if len(message) > maxRouterDiagnosticBytes+len("…") {
		t.Fatalf("diagnostic length = %d, want bounded: %q", len(message), message)
	}
	if !strings.Contains(message, "claude") || !strings.Contains(message, "codex") {
		t.Fatalf("diagnostic omitted attempted providers: %q", message)
	}
}

func TestRouterSanitizesProviderErrorsBeforeReturning(t *testing.T) {
	t.Parallel()
	secret := "SECRET_PROMPT_OR_STDERR_SENTINEL"
	for _, mode := range []string{config.ProviderCodex, config.ProviderAuto} {
		first := &fakeProvider{name: NameCodex, attempts: []providerAttempt{{err: apperr.Wrap(
			apperr.KindProviderOutput,
			"fake provider",
			secret,
			errorsForTest(secret),
		)}}}
		priority := []string{NameCodex}
		_, err := NewRouter(first).Route(context.Background(), mode, priority, Request{Prompt: secret})
		if err == nil {
			t.Fatal("provider failure unexpectedly succeeded")
		}
		if strings.Contains(err.Error(), secret) || strings.Contains(apperr.Message(err), secret) {
			t.Fatalf("%s route leaked sensitive provider data: %v", mode, err)
		}
		if !strings.Contains(apperr.Message(err), "invalid structured result") {
			t.Fatalf("%s route omitted curated diagnostic: %q", mode, apperr.Message(err))
		}
	}
}

type errorsForTest string

func (e errorsForTest) Error() string { return string(e) }
