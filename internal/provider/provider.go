// Package provider invokes supported model CLIs behind a constrained process boundary.
package provider

import (
	"context"
	"errors"
	"time"

	"github.com/taiwong148960/intent-sh/internal/protocol"
)

const (
	NameClaude = "claude"
	NameCodex  = "codex"
)

// Request is the complete input supplied to one provider attempt.
type Request struct {
	Prompt  string
	Model   string
	Timeout time.Duration
}

// Result couples a strict provider value with the provider that produced it.
type Result struct {
	Provider string
	Value    protocol.ProviderResult
}

// ProbeResult contains safe compatibility information for doctor output.
type ProbeResult struct {
	Provider string
	Version  string
}

// ProbeStage identifies the compatibility step that failed without exposing
// provider process output.
type ProbeStage string

const (
	ProbeStageVersion  ProbeStage = "version"
	ProbeStageFeatures ProbeStage = "features"
	ProbeStageLogin    ProbeStage = "login"
)

// ProbeError preserves a safe provider error and its failed readiness stage.
type ProbeError struct {
	Stage ProbeStage
	Err   error
}

func (e *ProbeError) Error() string {
	if e == nil || e.Err == nil {
		return "provider probe failed"
	}
	return e.Err.Error()
}

func (e *ProbeError) Unwrap() error { return e.Err }

// ProbeStageOf returns the failed stage for diagnostic reporting.
func ProbeStageOf(err error) (ProbeStage, bool) {
	var probeErr *ProbeError
	if errors.As(err, &probeErr) {
		return probeErr.Stage, true
	}
	return "", false
}

func probeError(stage ProbeStage, err error) error {
	if err == nil {
		return nil
	}
	return &ProbeError{Stage: stage, Err: err}
}

// Provider is implemented only by official CLI adapters.
type Provider interface {
	Name() string
	Generate(context.Context, Request) (protocol.ProviderResult, error)
	Probe(context.Context) (ProbeResult, error)
}
