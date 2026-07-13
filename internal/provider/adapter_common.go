package provider

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/taiwong148960/intent-sh/internal/apperr"
)

const probeTimeout = 5 * time.Second

func runnerOrDefault(runner CommandRunner) CommandRunner {
	if runner != nil {
		return runner
	}
	return ProcessRunner{}
}

func fixedArgs(args ...string) func(string) []string {
	return func(string) []string { return append([]string(nil), args...) }
}

func processFailure(providerName, loginInstruction string, result RunResult, err error) error {
	if err == nil {
		return nil
	}
	if kind := apperr.KindOf(err); kind != apperr.KindInternal {
		return err
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		return apperr.Wrap(apperr.KindProviderUnavailable, "run "+providerName, providerName+" provider failed", err)
	}
	diagnostic := strings.ToLower(string(result.Stderr) + "\n" + string(result.Stdout))
	if containsAny(diagnostic,
		"not logged in", "not authenticated", "authentication required", "unauthorized",
		"please log in", "please login", "login required", "auth required") {
		return apperr.Wrap(apperr.KindProviderUnavailable, "authenticate "+providerName, loginInstruction, err)
	}
	if containsAny(diagnostic,
		"rate limit", "rate_limit", "overloaded", "temporarily unavailable", "try again later",
		"connection refused", "connection reset", "network error", "service unavailable") {
		return apperr.Wrap(apperr.KindProviderUnavailable, "run "+providerName, providerName+" is temporarily unavailable", err)
	}
	return apperr.Wrap(apperr.KindProviderUnavailable, "run "+providerName, providerName+" provider failed", err)
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func checkHelp(providerName string, help []byte, flags []string) error {
	text := string(help)
	for _, flag := range flags {
		if !strings.Contains(text, flag) {
			return apperr.New(apperr.KindProviderUnavailable, "probe "+providerName, "installed "+providerName+" CLI is incompatible; update it")
		}
	}
	return nil
}

func ensureLoginReady(providerName, instruction string, result RunResult) error {
	status := strings.ToLower(string(result.Stdout) + "\n" + string(result.Stderr))
	compact := strings.ReplaceAll(strings.ReplaceAll(status, " ", ""), "_", "")
	if containsAny(status, "not logged in", "not authenticated", "login required", "authentication required") ||
		strings.Contains(compact, `"loggedin":false`) {
		return apperr.New(apperr.KindProviderUnavailable, "authenticate "+providerName, instruction)
	}
	return nil
}

func probeVersion(ctx context.Context, runner CommandRunner, program, providerName string) (string, error) {
	result, err := runner.Run(ctx, Invocation{
		Program: program,
		Args:    fixedArgs("--version"),
		Timeout: probeTimeout,
	})
	if err != nil {
		return "", processFailure(providerName, providerName+" CLI is not logged in", result, err)
	}
	version := strings.TrimSpace(string(result.Stdout))
	if version == "" {
		version = strings.TrimSpace(string(result.Stderr))
	}
	if version == "" {
		return "", apperr.New(apperr.KindProviderUnavailable, "probe "+providerName, providerName+" CLI did not report a version")
	}
	return boundedText(version, 120), nil
}
