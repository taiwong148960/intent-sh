## MODIFIED Requirements

### Requirement: Activate adapters explicitly and reversibly
`intent-sh init zsh|bash` SHALL load and validate the effective rewrite and undo chords before emitting the embedded adapter for the requested shell. `intent-sh setup zsh|bash` SHALL report the appropriate startup file, an idempotent activation line, the effective default or configured bindings, detected static conflicts, and removal guidance, but MUST NOT modify a startup file, shell keymap, terminal preference, tmux configuration, or user configuration by default. Bash setup guidance SHALL state that Bash 4.0 or newer with native Readline is required. Neither setup nor initialization SHALL download, install, or configure a third-party line editor.

#### Scenario: Request Zsh setup guidance
- **WHEN** the user runs `intent-sh setup zsh` with default configuration
- **THEN** the command prints the exact activation line, target file, `Alt+G` and `Alt+U` defaults, Enter guard, cancellation key, and removal instruction without editing the file

#### Scenario: Request setup with custom chords
- **WHEN** valid custom rewrite and undo chords are present in configuration
- **THEN** setup prints those effective chords and checks the corresponding shell binding forms for static conflicts

#### Scenario: Request Bash setup guidance
- **WHEN** the user requests Bash setup
- **THEN** the guidance states the Bash 4.0 minimum and native Readline requirement without offering or managing an alternate editor backend

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
- **WHEN** the user requests setup or initialization for Fish, Nushell, PowerShell, or an unknown shell
- **THEN** the command exits nonzero with the supported Zsh/Bash choices and makes no system change

### Requirement: Diagnose local readiness without leaking secrets
Ordinary `intent-sh doctor` SHALL check supported platform and architecture, Bash 4.0+/Zsh compatibility, native ZLE/Readline editor compatibility, config and chord validity, adapter/binary protocol-2 compatibility, effective-key conflicts visible in the selected startup file, configured provider executable and compatible version, and official CLI login readiness. `intent-sh doctor --keys` SHALL additionally perform the bounded opt-in controlling-terminal delivery probe for rewrite, undo, Enter, and cancellation. Both modes SHALL emit stable check identifiers, actionable guidance, and no tokens, credential-file contents, prompts, shell buffers, history, screen contents, or unbounded received bytes.

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
The repository SHALL document supported systems, the Bash 4.0 minimum and native Readline requirement, the behavioral PTY contract, qualified terminal records, source installation, adapter activation, binding configuration and reset, interactive key probing, tmux and SSH expectations, remote provider locality, first rewrite, regenerate, undo, cancellation, risk behavior, provider login prerequisites, the exact context sent to providers, explicit non-collected data, and complete removal. Documentation MUST distinguish contract-compatible from recorded qualified environments, explain that risk detection is heuristic, and state that no generated command is automatically executed.

#### Scenario: New user follows the default guide
- **WHEN** a user has one supported provider CLI installed and logged in and a terminal that delivers the default chords
- **THEN** the guide takes them from source build through native adapter activation, ordinary doctor, optional key probe, a harmless first rewrite, undo, and removal without requesting a new credential

#### Scenario: User remaps an intercepted chord
- **WHEN** the key probe shows that a terminal or tmux layer does not deliver a default chord
- **THEN** the guide shows how to choose an allowed alternative, validate it, reinitialize in a new shell, and restore the defaults without editing terminal settings automatically

#### Scenario: User qualifies an SSH environment
- **WHEN** the user follows the SSH qualification guide
- **THEN** the guide makes clear that the remote host needs its own binary and authenticated provider and records no client or remote credential material

#### Scenario: User removes the integration
- **WHEN** the user follows the removal instructions
- **THEN** removing one startup-file activation line and the binary disables the product, with the optional secret-free config safe to delete independently and no terminal or tmux configuration to undo

#### Scenario: User downgrades after configuring custom keys
- **WHEN** the user rolls back to a binary whose strict configuration schema predates binding keys
- **THEN** the documentation instructs them to remove `rewrite_key` and `undo_key` before downgrade so the older binary can load its defaults
