// Package app orchestrates one complete rewrite without shell-side effects.
package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/config"
	contextinfo "github.com/taiwong148960/intent-sh/internal/context"
	"github.com/taiwong148960/intent-sh/internal/prompt"
	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/internal/provider"
	"github.com/taiwong148960/intent-sh/internal/safety"
	"github.com/taiwong148960/intent-sh/internal/textsafe"
)

type ConfigLoader func() (config.Config, string, error)

type ContextBuilder interface {
	Build(shell, shellVersion, cwd string) contextinfo.Environment
}

type ProviderRouter interface {
	Route(context.Context, string, []string, provider.Request) (provider.Result, error)
}

type SafetyEvaluator interface {
	Evaluate(context.Context, string, string, string) (safety.Decision, error)
}

// Service owns the dependency seams required for deterministic orchestration tests.
type Service struct {
	LoadConfig ConfigLoader
	Context    ContextBuilder
	Router     ProviderRouter
	Safety     SafetyEvaluator
	Getwd      func() (string, error)
}

func DefaultService() Service {
	return Service{
		LoadConfig: config.Load,
		Context:    contextinfo.NewBuilder(),
		Router:     provider.NewRouter(provider.Claude{}, provider.Codex{}),
		Safety:     safety.Engine{},
		Getwd:      os.Getwd,
	}
}

// Rewrite returns a replacement only after every provider and local check succeeds.
func (s Service) Rewrite(ctx context.Context, request protocol.AdapterRequest) (protocol.AdapterResponse, error) {
	base := protocol.AdapterResponse{Version: protocol.AdapterVersion, RequestID: request.RequestID}
	if err := ValidateRequest(request); err != nil {
		return base, err
	}
	loadConfig := s.LoadConfig
	if loadConfig == nil {
		loadConfig = config.Load
	}
	cfg, _, err := loadConfig()
	if err != nil {
		return base, err
	}
	getwd := s.Getwd
	if getwd == nil {
		getwd = os.Getwd
	}
	cwd, err := getwd()
	if err != nil {
		return base, apperr.Wrap(apperr.KindInvalidInput, "build rewrite context", "could not determine the current directory", err)
	}
	builder := s.Context
	if builder == nil {
		defaultBuilder := contextinfo.NewBuilder()
		builder = defaultBuilder
	}
	environment := builder.Build(request.Shell, request.ShellVersion, cwd)
	promptText, err := prompt.Build(prompt.Input{
		Buffer:          request.Buffer,
		Cursor:          request.Cursor,
		Original:        request.Original,
		Previous:        request.Previous,
		GenerationIndex: request.GenerationIndex,
		Environment:     environment,
	})
	if err != nil {
		return base, apperr.Wrap(apperr.KindInternal, "build provider prompt", "could not build the provider request", err)
	}
	router := s.Router
	if router == nil {
		return base, apperr.New(apperr.KindInternal, "route provider", "provider router was not initialized")
	}
	providerResult, err := router.Route(ctx, cfg.Provider, cfg.Priority, provider.Request{
		Prompt:  promptText,
		Model:   cfg.Model,
		Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		return base, err
	}
	switch providerResult.Value.Status {
	case protocol.ProviderStatusClarify:
		base.Status = protocol.StatusClarify
		base.Message = boundMessage(providerResult.Value.Question, 1024)
		base.Provider = providerResult.Provider
		return base, nil
	case protocol.ProviderStatusOK:
	default:
		return base, apperr.New(apperr.KindProviderOutput, "apply provider result", "provider returned an unsupported result status")
	}
	evaluator := s.Safety
	if evaluator == nil {
		defaultEvaluator := safety.Engine{}
		evaluator = defaultEvaluator
	}
	decision, err := evaluator.Evaluate(ctx, providerResult.Value.Command, request.Shell, providerResult.Value.RiskHint)
	if err != nil {
		return base, err
	}
	base.Status = protocol.StatusOK
	base.Replacement = decision.Command
	base.Message = boundMessage(providerResult.Value.Explanation, 1024)
	base.Provider = providerResult.Provider
	base.Risk = string(decision.Level)
	base.RiskReason = boundMessage(decision.Reason, 512)
	return base, nil
}

// ValidateRequest checks semantic fields not covered by NUL framing.
func ValidateRequest(request protocol.AdapterRequest) error {
	if request.Version != protocol.AdapterVersion {
		return apperr.New(apperr.KindProtocol, "validate adapter request", "adapter protocol is incompatible with binary protocol "+protocol.AdapterVersion)
	}
	if request.Action != protocol.ActionRewrite {
		return apperr.New(apperr.KindInvalidInput, "validate adapter request", "adapter action must be rewrite")
	}
	if request.Shell != safety.ShellBash && request.Shell != safety.ShellZsh {
		return apperr.New(apperr.KindInvalidInput, "validate adapter request", "supported shells are bash and zsh")
	}
	if request.ShellVersion == "" || len(request.ShellVersion) > 64 || strings.ContainsAny(request.ShellVersion, "\x00\r\n") {
		return apperr.New(apperr.KindInvalidInput, "validate adapter request", "shell version must be a short single-line value")
	}
	if request.Shell == safety.ShellBash && bashMajor(request.ShellVersion) < 4 {
		return apperr.New(apperr.KindProtocol, "validate adapter request", "Bash 4.0 or newer is required; use Zsh or install a modern Bash")
	}
	if strings.TrimSpace(request.Buffer) == "" {
		return apperr.New(apperr.KindInvalidInput, "validate adapter request", "enter a command or intent before requesting a rewrite")
	}
	if request.Cursor < 0 || request.Cursor > len(request.Buffer) {
		return apperr.New(apperr.KindInvalidInput, "validate adapter request", "cursor is outside the editable buffer")
	}
	if request.RequestID == "" || len(request.RequestID) > 128 || strings.ContainsAny(request.RequestID, "\x00\r\n") {
		return apperr.New(apperr.KindInvalidInput, "validate adapter request", "request ID must be a short single-line value")
	}
	if request.GenerationIndex < 0 {
		return apperr.New(apperr.KindInvalidInput, "validate adapter request", "generation index must be non-negative")
	}
	if request.GenerationIndex > 0 && (strings.TrimSpace(request.Original) == "" || strings.TrimSpace(request.Previous) == "") {
		return apperr.New(apperr.KindInvalidInput, "validate adapter request", "regeneration requires the original intent and previous command")
	}
	return nil
}

func bashMajor(version string) int {
	prefix := version
	if index := strings.IndexByte(prefix, '.'); index >= 0 {
		prefix = prefix[:index]
	}
	major, _ := strconv.Atoi(prefix)
	return major
}

// HandleRewrite decodes and fully buffers one adapter response before writing.
func (s Service) HandleRewrite(ctx context.Context, input io.Reader, output io.Writer) error {
	request, err := protocol.DecodeRequest(input)
	if err != nil {
		response := ErrorResponse(protocol.AdapterRequest{}, err)
		if writeErr := writeBufferedResponse(output, response); writeErr != nil {
			return writeErr
		}
		return err
	}
	response, rewriteErr := s.Rewrite(ctx, request)
	if rewriteErr != nil {
		response = ErrorResponse(request, rewriteErr)
	}
	if err := writeBufferedResponse(output, response); err != nil {
		return err
	}
	return rewriteErr
}

func ErrorResponse(request protocol.AdapterRequest, err error) protocol.AdapterResponse {
	status := protocol.StatusError
	if apperr.KindOf(err) == apperr.KindCancelled {
		status = protocol.StatusCancel
	}
	return protocol.AdapterResponse{
		Version:   protocol.AdapterVersion,
		Status:    status,
		Message:   boundMessage(apperr.Message(err), 1024),
		RequestID: request.RequestID,
	}
}

func writeBufferedResponse(output io.Writer, response protocol.AdapterResponse) error {
	var buffer bytes.Buffer
	if err := protocol.EncodeResponse(&buffer, response); err != nil {
		return err
	}
	if _, err := io.Copy(output, &buffer); err != nil {
		return apperr.Wrap(apperr.KindInternal, "write adapter response", "could not write the adapter response", err)
	}
	return nil
}

func boundMessage(message string, limit int) string {
	return textsafe.Terminal(message, limit)
}
