package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/taiwong148960/intent-sh/internal/apperr"
	"github.com/taiwong148960/intent-sh/internal/protocol"
	"github.com/taiwong148960/intent-sh/schemas"
)

const ClaudeProgram = "claude"

var claudeRequiredFlags = []string{
	"--print",
	"--bare",
	"--tools",
	"--strict-mcp-config",
	"--no-session-persistence",
	"--output-format",
	"--json-schema",
}

// Claude invokes the official Claude Code CLI.
type Claude struct {
	Runner  CommandRunner
	Program string
}

func (Claude) Name() string { return NameClaude }

func (c Claude) Generate(ctx context.Context, request Request) (protocol.ProviderResult, error) {
	program := c.Program
	if program == "" {
		program = ClaudeProgram
	}
	runner := runnerOrDefault(c.Runner)
	result, err := runner.Run(ctx, Invocation{
		Program: program,
		Args:    claudeArgs(request.Model),
		Stdin:   []byte(request.Prompt),
		Timeout: request.Timeout,
	})
	if err != nil {
		return protocol.ProviderResult{}, processFailure("Claude Code", "Claude Code is not logged in; run claude and use /login", result, err)
	}
	value, err := decodeClaudeOutput(result.Stdout)
	if err != nil {
		return protocol.ProviderResult{}, err
	}
	return value, nil
}

func (c Claude) Probe(ctx context.Context) (ProbeResult, error) {
	result := ProbeResult{Provider: NameClaude}
	program := c.Program
	if program == "" {
		program = ClaudeProgram
	}
	runner := runnerOrDefault(c.Runner)
	version, err := probeVersion(ctx, runner, program, "Claude Code")
	if err != nil {
		return result, probeError(ProbeStageVersion, err)
	}
	result.Version = version
	help, err := runner.Run(ctx, Invocation{Program: program, Args: fixedArgs("--help"), Timeout: probeTimeout})
	if err != nil {
		return result, probeError(ProbeStageFeatures, processFailure("Claude Code", "Claude Code is not logged in; run claude and use /login", help, err))
	}
	if err := checkHelp("Claude Code", append(help.Stdout, help.Stderr...), claudeRequiredFlags); err != nil {
		return result, probeError(ProbeStageFeatures, err)
	}
	auth, err := runner.Run(ctx, Invocation{Program: program, Args: fixedArgs("auth", "status"), Timeout: probeTimeout})
	if err != nil {
		return result, probeError(ProbeStageLogin, processFailure("Claude Code", "Claude Code is not logged in; run claude and use /login", auth, err))
	}
	if err := ensureLoginReady("Claude Code", "Claude Code is not logged in; run claude and use /login", auth); err != nil {
		return result, probeError(ProbeStageLogin, err)
	}
	return result, nil
}

func claudeArgs(model string) func(string) []string {
	return func(string) []string {
		args := []string{
			"-p",
			"--bare",
			"--tools", "",
			"--strict-mcp-config",
			"--no-session-persistence",
			"--max-turns", "1",
			"--output-format", "json",
			"--json-schema", string(schemas.ProviderResult),
		}
		if model != "" {
			args = append(args, "--model", model)
		}
		return args
	}
}

type claudeEnvelope struct {
	IsError          bool            `json:"is_error"`
	Result           json.RawMessage `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
}

func decodeClaudeOutput(data []byte) (protocol.ProviderResult, error) {
	if value, err := DecodeResult(data); err == nil {
		return value, nil
	}
	if len(data) == 0 || len(data) > protocol.MaxProviderOutputBytes {
		return protocol.ProviderResult{}, outputError("Claude Code returned an invalid structured result")
	}
	if !utf8.Valid(data) {
		return protocol.ProviderResult{}, outputError("Claude Code returned invalid UTF-8")
	}
	if err := rejectDuplicateJSONFields(data); err != nil {
		return protocol.ProviderResult{}, outputError("Claude Code returned an invalid response envelope")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var envelope claudeEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return protocol.ProviderResult{}, outputWrap("Claude Code returned an invalid response envelope", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return protocol.ProviderResult{}, outputError("Claude Code returned trailing output")
	}
	if envelope.IsError {
		return protocol.ProviderResult{}, apperr.New(apperr.KindProviderUnavailable, "run Claude Code", "Claude Code provider failed")
	}
	if raw := nonNullRaw(envelope.StructuredOutput); raw != nil {
		return DecodeResult(raw)
	}
	if raw := nonNullRaw(envelope.Result); raw != nil {
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			return DecodeResult([]byte(text))
		}
		return DecodeResult(raw)
	}
	return protocol.ProviderResult{}, outputError("Claude Code response omitted structured output")
}

func nonNullRaw(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	return trimmed
}
