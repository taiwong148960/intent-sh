package provider

import (
	"context"
	"path/filepath"

	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/schemas"
)

const (
	CodexProgram = "codex"
	codexSchema  = "provider-result.schema.json"
	codexResult  = "provider-result.json"
)

var codexRequiredFlags = []string{
	"--ephemeral",
	"--ignore-user-config",
	"--ignore-rules",
	"--sandbox",
	"--output-schema",
	"--output-last-message",
	"--skip-git-repo-check",
}

// Codex invokes the official Codex CLI.
type Codex struct {
	Runner  CommandRunner
	Program string
}

func (Codex) Name() string { return NameCodex }

func (c Codex) Generate(ctx context.Context, request Request) (protocol.ProviderResult, error) {
	program := c.Program
	if program == "" {
		program = CodexProgram
	}
	runner := runnerOrDefault(c.Runner)
	result, err := runner.Run(ctx, Invocation{
		Program:    program,
		Args:       codexArgs(request.Model),
		Stdin:      []byte(request.Prompt),
		Files:      map[string][]byte{codexSchema: schemas.ProviderResult},
		ResultFile: codexResult,
		Timeout:    request.Timeout,
	})
	if err != nil {
		return protocol.ProviderResult{}, processFailure("Codex CLI", "Codex CLI is not logged in; run codex login", result, err)
	}
	return DecodeResult(result.ResultFile)
}

func (c Codex) Probe(ctx context.Context) (ProbeResult, error) {
	result := ProbeResult{Provider: NameCodex}
	program := c.Program
	if program == "" {
		program = CodexProgram
	}
	runner := runnerOrDefault(c.Runner)
	version, err := probeVersion(ctx, runner, program, "Codex CLI")
	if err != nil {
		return result, probeError(ProbeStageVersion, err)
	}
	result.Version = version
	help, err := runner.Run(ctx, Invocation{Program: program, Args: fixedArgs("exec", "--help"), Timeout: probeTimeout})
	if err != nil {
		return result, probeError(ProbeStageFeatures, processFailure("Codex CLI", "Codex CLI is not logged in; run codex login", help, err))
	}
	if err := checkHelp("Codex CLI", append(help.Stdout, help.Stderr...), codexRequiredFlags); err != nil {
		return result, probeError(ProbeStageFeatures, err)
	}
	auth, err := runner.Run(ctx, Invocation{Program: program, Args: fixedArgs("login", "status"), Timeout: probeTimeout})
	if err != nil {
		return result, probeError(ProbeStageLogin, processFailure("Codex CLI", "Codex CLI is not logged in; run codex login", auth, err))
	}
	if err := ensureLoginReady("Codex CLI", "Codex CLI is not logged in; run codex login", auth); err != nil {
		return result, probeError(ProbeStageLogin, err)
	}
	return result, nil
}

func codexArgs(model string) func(string) []string {
	return func(workDir string) []string {
		args := []string{
			"exec",
			"--ephemeral",
			"--ignore-user-config",
			"--ignore-rules",
			"--sandbox", "read-only",
			"--skip-git-repo-check",
			"--output-schema", filepath.Join(workDir, codexSchema),
			"--output-last-message", filepath.Join(workDir, codexResult),
			"--color", "never",
		}
		if model != "" {
			args = append(args, "--model", model)
		}
		return append(args, "-")
	}
}
