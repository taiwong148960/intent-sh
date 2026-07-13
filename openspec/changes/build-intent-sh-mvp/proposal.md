## Why

Shell users often know the operation they want but lose time recalling exact command syntax, while chat-based assistants interrupt the terminal workflow and may blur the boundary between suggesting and executing commands. `intent-sh` should validate a smaller, safer interaction: rewrite the editable shell buffer in place using an already-authenticated Claude Code or Codex CLI, then leave review and execution entirely to the user.

## What Changes

- Add a single Go binary named `intent-sh` with rewrite, setup, configuration, and diagnostic entry points plus a versioned request/result contract for shell adapters.
- Add Zsh ZLE and Bash Readline adapters for macOS and Linux that rewrite the full current buffer, regenerate from the preserved original intent, undo to that original, and cancel an in-flight provider process.
- Add Claude Code and Codex CLI providers that reuse official CLI authentication, run without model tools or session persistence, request structured output, and fall back according to local configuration.
- Build requests from the current buffer and a deliberately small environment snapshot; support both Chinese and English intent without reading history, terminal output, environment values, project files, diffs, or credentials.
- Validate every provider response locally as one bounded shell command, assign a local safe/review/dangerous classification, warn for risky commands, and require a second Enter before a dangerous generated command can run.
- Add deterministic unit, integration, adapter, and safety tests together with install/uninstall documentation for a source-built MVP.
- Keep Fish, PowerShell, selection rewriting, arbitrary terminal-screen selection, command explanations, desktop UI, daemons, cloud accounts, API keys, multi-step agents, and automatic execution outside this MVP.

## Capabilities

### New Capabilities

- `shell-rewrite-workflow`: Full-buffer rewrite, regenerate, undo, cancellation, buffer ownership, and keybinding behavior in Zsh and Bash.
- `provider-routing`: Minimal request context, structured model results, constrained Claude Code and Codex CLI invocation, timeout handling, and configured fallback.
- `command-safety`: Local output validation, shell syntax checks, risk classification, warnings, and dangerous-command confirmation before execution.
- `installation-diagnostics`: Source installation, adapter activation, local configuration, provider discovery, and actionable diagnostics on supported systems.

### Modified Capabilities

None.

## Impact

This creates the initial Go module, internal packages, JSON schemas, Zsh and Bash adapter scripts, installer assets, tests, and user documentation in the `intent-sh` repository. Runtime dependencies are a supported shell plus at least one installed and authenticated official provider CLI (`claude` or `codex`); there is no hosted service, credential store, database, or direct model API dependency. Provider CLI flags and shell editing APIs form the main external compatibility boundaries.
