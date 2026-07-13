package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/config"
	"github.com/taiwong148960/intent-sh/internal/protocol"
)

const maxRouterDiagnosticBytes = 480

// Router tries providers in configured order and never in parallel.
type Router struct {
	Providers map[string]Provider
}

func NewRouter(providers ...Provider) Router {
	byName := make(map[string]Provider, len(providers))
	for _, item := range providers {
		if item != nil {
			byName[item.Name()] = item
		}
	}
	return Router{Providers: byName}
}

func (r Router) Route(ctx context.Context, mode string, priority []string, request Request) (Result, error) {
	names := priority
	if mode != config.ProviderAuto {
		names = []string{mode}
	}
	if len(names) == 0 {
		return Result{}, apperr.New(apperr.KindConfiguration, "route provider", "no providers are configured")
	}

	diagnostics := make([]string, 0, len(names))
	lastKind := apperr.KindProviderUnavailable
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return Result{}, apperr.Wrap(apperr.KindCancelled, "route provider", "provider request was cancelled", err)
		}
		item := r.Providers[name]
		if item == nil {
			err := apperr.New(apperr.KindProviderUnavailable, "route provider", fmt.Sprintf("configured provider %q is unavailable", boundedText(name, 40)))
			if mode != config.ProviderAuto {
				return Result{}, err
			}
			diagnostics = append(diagnostics, safeDiagnostic(name, err))
			continue
		}

		value, err := item.Generate(ctx, request)
		if err == nil && value.Status != protocol.ProviderStatusOK && value.Status != protocol.ProviderStatusClarify {
			err = apperr.New(apperr.KindProviderOutput, "route provider", "provider returned an unsupported result status")
		}
		if err == nil {
			return Result{Provider: name, Value: value}, nil
		}
		err = sanitizedProviderError(name, err)
		if apperr.KindOf(err) == apperr.KindCancelled {
			return Result{}, err
		}
		if mode != config.ProviderAuto || !fallbackEligible(err) {
			return Result{}, err
		}
		lastKind = apperr.KindOf(err)
		diagnostics = append(diagnostics, safeDiagnostic(name, err))
	}

	message := "all configured providers failed"
	if len(diagnostics) > 0 {
		message += ": " + strings.Join(diagnostics, "; ")
	}
	message = boundedText(message, maxRouterDiagnosticBytes)
	return Result{}, apperr.New(lastKind, "route provider", message)
}

func fallbackEligible(err error) bool {
	switch apperr.KindOf(err) {
	case apperr.KindProviderUnavailable, apperr.KindTimeout, apperr.KindProviderOutput:
		return true
	default:
		return false
	}
}

func safeDiagnostic(name string, err error) string {
	return boundedText(name+": "+apperr.Message(err), 150)
}

func sanitizedProviderError(name string, err error) error {
	kind := apperr.KindOf(err)
	label := "provider"
	login := "Use the provider's official login flow."
	switch name {
	case NameClaude:
		label = "Claude Code"
		login = "Claude Code is not logged in; run claude and use /login"
	case NameCodex:
		label = "Codex CLI"
		login = "Codex CLI is not logged in; run codex login"
	}
	message := "internal provider failure"
	switch kind {
	case apperr.KindProviderUnavailable:
		original := strings.ToLower(apperr.Message(err))
		switch {
		case containsAny(original, "not logged in", "not authenticated", "login required", "authentication required"):
			message = login
		case strings.Contains(original, "incompatible"):
			message = "installed " + label + " CLI is incompatible; update it"
		case strings.Contains(original, "temporarily unavailable"):
			message = label + " is temporarily unavailable"
		default:
			message = label + " is unavailable"
		}
	case apperr.KindTimeout:
		message = label + " timed out"
	case apperr.KindCancelled:
		message = "provider request was cancelled"
	case apperr.KindProviderOutput:
		message = label + " returned an invalid structured result"
	case apperr.KindConfiguration:
		message = "provider request configuration is invalid"
	}
	return apperr.Wrap(kind, "route provider", message, err)
}
