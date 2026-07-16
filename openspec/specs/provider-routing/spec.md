# Provider Routing Specification

## Purpose

Define privacy-preserving request construction, constrained official CLI invocation, deterministic provider selection, and bounded provider execution.

## Requirements

### Requirement: Build a minimal allowlisted request context
The system SHALL send only the current input, cursor, original and previous command when applicable, generation index, operating system, architecture, shell and version, current directory path, SSH boolean, locale, and presence of commands from a fixed allowlist. It MUST NOT read or send shell history, terminal output, arbitrary environment values, project files, `.env` files, Git diffs, clipboard data, SSH configuration, or credential material.

#### Scenario: Inspect a rewrite payload
- **WHEN** a fake provider captures a normal rewrite request
- **THEN** the payload contains the documented allowlisted fields and none of the prohibited context sources

#### Scenario: Detect a remote shell
- **WHEN** the shell environment contains an SSH marker
- **THEN** the model context contains only `remote: true` and not the marker's value

#### Scenario: Report available commands
- **WHEN** the core builds the environment snapshot
- **THEN** it tests only the fixed command allowlist and sends command names without paths, versions, or filesystem enumeration

### Requirement: Request one strict structured result
Every provider SHALL be instructed to return exactly one JSON value matching the versioned result schema. A result SHALL be either `ok` with a command, explanation, assumptions, and risk hint, or `clarify` with one question and no command.

#### Scenario: Receive a valid command result
- **WHEN** a provider returns one schema-valid `ok` object
- **THEN** the router forwards it to local command validation without treating its risk hint as authoritative

#### Scenario: Receive a valid clarification
- **WHEN** a provider returns one schema-valid `clarify` object
- **THEN** the router returns the question without invoking a fallback provider

#### Scenario: Receive malformed or extra output
- **WHEN** provider output has unknown fields, model chatter, multiple JSON values, or does not satisfy either schema branch
- **THEN** the attempt fails closed and no text from that attempt is inserted into the shell buffer

### Requirement: Apply safety-biased model instructions
The provider prompt SHALL require conversion of intent into one command, prohibit execution and file inspection, prohibit adding `sudo` unless explicitly requested, prefer tools available on macOS and in the supplied command allowlist, preserve existing shell fragments when possible, and request a non-destructive preview for ambiguous destructive intent.

#### Scenario: Ambiguous deletion intent
- **WHEN** the input asks to delete old log files without an explicit scope and immediate-execution instruction
- **THEN** the model is instructed to return a preview command that lists matching files instead of deleting them

#### Scenario: Explicit macOS platform context
- **WHEN** a rewrite is requested from a supported macOS environment
- **THEN** the prompt identifies Darwin, architecture, and shell so the provider can avoid incompatible syntax

### Requirement: Invoke official provider CLIs in constrained subprocesses
The system SHALL invoke the official Claude Code or Codex CLI directly as a subprocess in a newly created empty temporary working directory. It SHALL disable model tools and session persistence, request structured output, apply the strongest supported user-config isolation, and MUST NOT invoke the provider through a shell command string.

#### Scenario: Invoke Claude Code
- **WHEN** Claude is selected
- **THEN** the process uses its supported non-interactive bare structured-output mode with tools and session persistence disabled

#### Scenario: Invoke Codex CLI
- **WHEN** Codex is selected
- **THEN** the process uses ephemeral read-only non-interactive mode, ignores project configuration, receives the prompt on stdin, and writes temporary schema/result files only below the empty working directory

#### Scenario: Provider process exits
- **WHEN** a provider succeeds, fails, times out, or is cancelled
- **THEN** its temporary directory is removed and no rewrite session is persisted by `intent-sh`

### Requirement: Reuse official login without handling credentials
The system SHALL rely on authentication already managed by each official CLI. It MUST NOT ask for an API key, implement OAuth, locate or parse provider tokens, copy credentials, or store authentication data.

#### Scenario: Provider is not logged in
- **WHEN** the selected official CLI reports that authentication is required
- **THEN** the system returns an actionable instruction to use that CLI's official login command without displaying or storing credential data

### Requirement: Route providers deterministically
Configuration SHALL support `auto`, `claude`, and `codex` provider modes, an ordered priority list, a timeout, and an optional model. In `auto` mode the router SHALL try providers sequentially in priority order; an explicit provider mode SHALL disable fallback.

#### Scenario: First provider succeeds
- **WHEN** the first auto-priority provider returns a valid `ok` result
- **THEN** no later provider is invoked

#### Scenario: First provider is unavailable
- **WHEN** the first auto-priority provider is absent, unauthenticated, transiently unavailable, timed out, or returns invalid structured output
- **THEN** the router records a bounded diagnostic and tries the next configured provider

#### Scenario: Explicit provider fails
- **WHEN** `provider` is explicitly `claude` or `codex` and that provider fails
- **THEN** the failure is returned without invoking the other provider

#### Scenario: User cancellation occurs
- **WHEN** the active provider attempt is cancelled by the user
- **THEN** routing stops immediately and no fallback is attempted

### Requirement: Bound provider processes and output
Each provider attempt SHALL have the configured deadline, bounded stdout and stderr capture, and process-group cleanup. Error messages returned to the adapter MUST be concise and MUST NOT expose the full prompt, raw model output, or unbounded provider diagnostics.

#### Scenario: Provider exceeds its deadline
- **WHEN** a provider remains active past the configured timeout
- **THEN** the full provider process group is terminated and the router either follows auto fallback policy or returns a timeout

#### Scenario: Provider emits excessive output
- **WHEN** stdout or stderr exceeds its configured bound
- **THEN** capture is stopped or truncated safely and the attempt fails without applying partial output

### Requirement: Support Chinese and English intent and alternatives
The same rewrite contract SHALL accept Chinese and English input. Regeneration SHALL include the preserved original, previous command, and generation index and SHALL request a materially different alternative.

#### Scenario: Rewrite Chinese intent
- **WHEN** a user submits a Chinese natural-language shell intent
- **THEN** the provider is asked for one target-shell command and may return a concise clarification in Chinese

#### Scenario: Regenerate a previous result
- **WHEN** generation index is greater than zero
- **THEN** the provider prompt includes the original intent and previous command rather than treating the generated command as new intent
