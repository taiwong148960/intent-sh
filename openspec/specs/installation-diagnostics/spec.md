# Installation Diagnostics Specification

## Purpose

Define the supported binary distribution, reversible shell activation, validated configuration, readiness diagnostics, compatibility checks, and removal guidance.

## Requirements

### Requirement: Build as one supported Go binary
The project SHALL build one `intent-sh` executable for macOS on amd64 and arm64. The executable SHALL include rewrite, adapter initialization, setup guidance, configuration, doctor, and version commands and SHALL embed protocol-compatible Zsh and Bash adapter assets. Supported source and release targets MUST use Go's `darwin` target and Mach-O executable format.

#### Scenario: Build from source
- **WHEN** a developer on macOS with the supported Go toolchain runs the documented source build
- **THEN** one native executable is produced without requiring a desktop app, database, daemon, or hosted service

#### Scenario: Inspect supported release artifacts
- **WHEN** the release build produces `darwin/arm64` and `darwin/amd64` executables
- **THEN** each is verified as Mach-O with the declared CPU architecture, reproducible Go metadata, and embedded adapter markers

#### Scenario: Inspect the version
- **WHEN** the user runs `intent-sh version`
- **THEN** the output identifies the binary version and adapter protocol version

### Requirement: Activate adapters explicitly and reversibly
`intent-sh init zsh|bash` SHALL load and validate the effective rewrite and undo chords before emitting the embedded adapter. `intent-sh setup zsh|bash` SHALL report the macOS startup file, idempotent activation line, effective bindings, detected static conflicts, and removal guidance, but MUST NOT modify startup files, keymaps, terminal preferences, tmux, or configuration. Zsh SHALL honor `ZDOTDIR` and otherwise select `.zshrc`. Bash SHALL prefer an existing `.bash_profile`, `.bash_login`, `.profile`, or `.bashrc` in that order and default to `.bash_profile`. Guidance SHALL require Bash 4.0+ native Readline. Neither command SHALL install or configure a third-party line editor.

#### Scenario: Request Zsh setup guidance
- **WHEN** the user runs `intent-sh setup zsh` with default configuration on macOS
- **THEN** the command prints the exact activation line, selected `.zshrc`, defaults, Enter guard, cancellation key, and removal instruction without editing the file

#### Scenario: Request setup with custom chords
- **WHEN** valid custom rewrite and undo chords are present in configuration
- **THEN** setup prints those effective chords and checks the corresponding shell binding forms for static conflicts

#### Scenario: Request Bash setup guidance
- **WHEN** the user requests Bash setup on macOS
- **THEN** the command selects the first documented startup candidate and states the Bash 4.0/native Readline requirement without offering another backend

#### Scenario: Load an adapter
- **WHEN** a supported interactive shell evaluates `intent-sh init` for its own shell with valid binding configuration
- **THEN** the embedded adapter loads only if its protocol version matches the binary and installs only the effective rewrite and undo chords

#### Scenario: Invalid binding blocks initialization
- **WHEN** the effective binding configuration is invalid
- **THEN** `init` exits nonzero before emitting a partial script and no binding is installed by its output

#### Scenario: Unsupported Bash evaluates the adapter
- **WHEN** Bash older than 4.0 evaluates the emitted adapter
- **THEN** initialization exits nonzero with actionable Bash 4.0-or-Zsh guidance and leaves existing keybindings unchanged

#### Scenario: Request an unsupported shell
- **WHEN** the user requests setup or initialization for a shell outside the documented Zsh and Bash backends
- **THEN** the command exits nonzero with the supported Zsh/Bash choices and makes no system change

### Requirement: Provide validated secret-free configuration
Configuration SHALL use `${XDG_CONFIG_HOME:-$HOME/.config}/intent-sh/config.toml`, with defaults of auto routing, priority `claude` then `codex`, a 30-second timeout, no forced model, rewrite key `alt+g`, and undo key `alt+u`. The parser SHALL reject unknown keys, unknown providers, duplicate priority entries, timeouts outside 1–120 seconds, malformed or unsupported chords, reserved control chords, and equal rewrite/undo chords. The file MUST NOT contain provider credentials, raw terminal bytes, or terminal application settings.

#### Scenario: Run without a configuration file
- **WHEN** no config file exists
- **THEN** the binary uses all documented provider and binding defaults without creating a file

#### Scenario: Set a supported value
- **WHEN** the user uses `intent-sh config set` with a valid provider, priority, timeout, model, rewrite key, or undo key value
- **THEN** the configuration is updated atomically and `config show` reports the effective non-secret settings in canonical form

#### Scenario: Set a reserved chord
- **WHEN** a user attempts to assign Enter, `Ctrl+C`, terminal flow-control or signal keys, EOF, or an adapter-private continuation key to rewrite or undo
- **THEN** the update is rejected atomically with the exact field and reason and the previous configuration remains effective

#### Scenario: Configure the same chord twice
- **WHEN** rewrite and undo normalize to the same canonical chord
- **THEN** configuration is rejected before adapter initialization or provider invocation

#### Scenario: Load invalid configuration
- **WHEN** the file contains an unknown key or invalid provider, timeout, model, or chord value
- **THEN** rewrite and adapter initialization fail before invoking a provider and report the exact configuration field to correct

### Requirement: Diagnose local readiness without leaking secrets
Ordinary `intent-sh doctor` SHALL check that the host is macOS on a supported architecture, Bash 4.0+/Zsh compatibility, native ZLE/Readline compatibility, config/chord validity, protocol-2 compatibility, startup-file key conflicts, provider executable/version, and official login readiness. `intent-sh doctor --keys` SHALL additionally perform the bounded controlling-terminal delivery probe. Both modes SHALL emit stable identifiers and actionable guidance without tokens, credential files, prompts, buffers, history, screen contents, or unbounded bytes.

#### Scenario: Inspect a supported macOS host
- **WHEN** doctor runs on a supported macOS architecture
- **THEN** platform checks pass and readiness continues to shell, adapter, configuration, key, and provider checks

#### Scenario: At least one provider is ready
- **WHEN** configuration is valid, the shell and native adapter are compatible, effective keys have no detected static conflict, and a configured provider is installed and logged in
- **THEN** ordinary doctor identifies the usable provider and effective bindings and exits successfully

#### Scenario: No provider is installed
- **WHEN** neither configured official provider CLI is available
- **THEN** doctor exits nonzero and lists official provider installation and login as the required next action without asking for an API key

#### Scenario: Provider login is missing
- **WHEN** a provider executable is present but its official login status is not ready
- **THEN** doctor reports that provider as unavailable and points to its official login flow without reading or printing its credentials

#### Scenario: Effective keybinding conflict is detected
- **WHEN** a configured rewrite chord, undo chord, or Enter guard has a visible unsupported custom binding in the selected startup file
- **THEN** doctor reports the shell, action, and canonical chord rather than silently claiming the adapter is ready

#### Scenario: Interactive key delivery succeeds
- **WHEN** the user runs `doctor --keys` on a controlling terminal and supplies all requested matching sequences
- **THEN** doctor reports stable passing key-delivery checks, invokes no provider, and restores the original terminal mode

#### Scenario: Interactive key delivery fails
- **WHEN** a configured chord is intercepted, transformed, times out, or exceeds the bounded input limit
- **THEN** doctor reports a failing delivery check and manual remapping guidance without modifying terminal or configuration state

#### Scenario: Interactive probe is not attached to a terminal
- **WHEN** `doctor --keys` cannot open a controlling terminal
- **THEN** it exits nonzero with an actionable check while ordinary non-interactive doctor remains available

#### Scenario: Bash version is unsupported
- **WHEN** doctor inspects Bash older than 4.0
- **THEN** it reports the shell as unsupported and recommends Bash 4.0+ or Zsh without modifying the system shell

#### Scenario: Editor backend is unsupported
- **WHEN** doctor receives an adapter status that is not native ZLE for Zsh or native Readline for Bash
- **THEN** it reports the adapter as incompatible and instructs the user to reinitialize in a supported native editor session

### Requirement: Fail closed on compatibility problems
The adapter and binary SHALL negotiate their protocol version, supported shell version, and native editor backend. Provider compatibility checks SHALL identify unsupported CLI versions or missing required isolation and structured-output features. An incompatible shell, editor, adapter, or provider MUST NOT perform a rewrite.

#### Scenario: Provider lacks a required capability
- **WHEN** the installed provider version cannot disable tools or produce the required structured result
- **THEN** doctor marks it incompatible and routing skips it in auto mode or fails clearly in explicit mode

#### Scenario: Adapter is newer than the binary
- **WHEN** the loaded adapter requests an unsupported protocol version
- **THEN** the binary returns a compatibility error without parsing remaining fields or producing a replacement

#### Scenario: Reported Bash version is unsupported
- **WHEN** an adapter request reports Bash older than 4.0
- **THEN** the binary rejects the request before provider invocation and emits no replacement

#### Scenario: Reported backend is invalid for the shell
- **WHEN** an adapter request reports a backend other than native ZLE for Zsh or native Readline for Bash
- **THEN** the binary rejects the request as incompatible and emits no replacement

### Requirement: Document privacy, source installation, removal, and terminal qualification
The repository SHALL document macOS and its supported architectures, Bash 4.0+ native Readline, the behavioral macOS PTY contract, qualified terminal records, source installation, activation, binding reset, key probing, tmux, protected macOS SSH, remote provider locality, rewrite/regenerate/undo/cancel behavior, risk, provider login, exact provider context, non-collected data, and complete removal. Documentation MUST distinguish contract compatibility from named qualification and native execution from artifact inspection, explain heuristic risk, and state that generated commands are never automatically executed.

#### Scenario: New user follows the default guide
- **WHEN** a macOS user has one supported provider CLI installed and logged in and a terminal that delivers the default chords
- **THEN** the guide takes them from source build through native adapter activation, ordinary doctor, optional key probe, a harmless first rewrite, undo, and removal without requesting a new credential

#### Scenario: User remaps an intercepted chord
- **WHEN** the key probe shows that a terminal or tmux layer does not deliver a default chord
- **THEN** the guide shows how to choose an allowed alternative, validate it, reinitialize in a new shell, and restore the defaults without editing terminal settings automatically

#### Scenario: User qualifies a macOS SSH environment
- **WHEN** the user follows the protected guide against a prepared macOS remote host
- **THEN** the guide requires the remote binary/provider, verifies the remote platform, and records no credential material

#### Scenario: User removes the integration
- **WHEN** the user follows the removal instructions
- **THEN** removing one startup-file activation line and the binary disables the product, with the optional secret-free config safe to delete independently and no terminal or tmux configuration to undo

#### Scenario: User downgrades after configuring custom keys
- **WHEN** the user rolls back to a binary whose strict configuration schema predates binding keys
- **THEN** the documentation instructs them to remove `rewrite_key` and `undo_key` before downgrade so the older binary can load its defaults
