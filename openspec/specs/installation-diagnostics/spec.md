# Installation Diagnostics Specification

## Purpose

Define the supported binary distribution, reversible shell activation, validated configuration, readiness diagnostics, compatibility checks, and removal guidance.

## Requirements

### Requirement: Build as one supported Go binary
The project SHALL build one `intent-sh` executable for macOS and Linux on amd64 and arm64. The executable SHALL include rewrite, adapter initialization, setup guidance, configuration, doctor, and version commands and SHALL embed protocol-compatible Zsh and Bash adapter assets.

#### Scenario: Build from source
- **WHEN** a developer with the supported Go toolchain runs the documented source build
- **THEN** one executable is produced without requiring a desktop app, database, daemon, or hosted service

#### Scenario: Inspect the version
- **WHEN** the user runs `intent-sh version`
- **THEN** the output identifies the binary version and adapter protocol version

### Requirement: Activate adapters explicitly and reversibly
`intent-sh init zsh|bash` SHALL emit the embedded adapter for the requested shell. `intent-sh setup zsh|bash` SHALL report the appropriate startup file and an idempotent activation line, but MUST NOT modify a startup file by default. Bash setup guidance SHALL explain that Bash 3.2 requires the exact tested ble.sh commit loaded before `intent-sh`, while Bash 4.0 or newer can use native Readline. Neither setup nor initialization SHALL download, install, update, or remove ble.sh automatically.

#### Scenario: Request Zsh setup guidance
- **WHEN** the user runs `intent-sh setup zsh`
- **THEN** the command prints the exact activation line, target file, default bindings, and removal instruction without editing the file

#### Scenario: Request Bash 3 setup guidance
- **WHEN** the user requests Bash setup for a stock macOS Bash 3.2 workflow
- **THEN** the guidance identifies ble.sh as an optional user-managed prerequisite, shows that ble.sh must load before the `intent-sh` activation line, and provides modern Bash and Zsh alternatives

#### Scenario: Load an adapter
- **WHEN** a supported interactive shell evaluates `intent-sh init` for its own shell with a compatible editor backend
- **THEN** the embedded adapter loads only if its protocol and backend capabilities match the binary

#### Scenario: Bash 3 dependency is missing
- **WHEN** Bash 3.2 evaluates the emitted adapter without compatible ble.sh attached
- **THEN** initialization exits nonzero with actionable guidance and leaves existing keybindings unchanged

#### Scenario: Request an unsupported shell
- **WHEN** the user requests setup or initialization for Fish, PowerShell, or an unknown shell
- **THEN** the command exits nonzero with the supported Zsh/Bash choices and makes no system change

### Requirement: Provide validated secret-free configuration
Configuration SHALL use `${XDG_CONFIG_HOME:-$HOME/.config}/intent-sh/config.toml`, with defaults of auto routing, priority `claude` then `codex`, a 30-second timeout, and no forced model. The parser SHALL reject unknown keys, unknown providers, duplicate priority entries, and timeouts outside 1–120 seconds. The file MUST NOT contain provider credentials.

#### Scenario: Run without a configuration file
- **WHEN** no config file exists
- **THEN** the binary uses the documented defaults without creating a file

#### Scenario: Set a supported value
- **WHEN** the user uses `intent-sh config set` with a valid provider, priority, timeout, or model value
- **THEN** the configuration is updated atomically and `config show` reports the effective non-secret settings

#### Scenario: Load invalid configuration
- **WHEN** the file contains an unknown key or invalid value
- **THEN** rewrite fails before invoking a provider and reports the exact configuration field to correct

### Requirement: Diagnose local readiness without leaking secrets
`intent-sh doctor` SHALL check supported platform and architecture, shell version, active editor backend, ble.sh version and required widget capabilities when selected, configuration validity, adapter/binary protocol compatibility, native and ble.sh keybinding conflicts, configured provider executable and compatible version, and official CLI login readiness. It SHALL emit stable check identifiers, actionable guidance, and no tokens or credential-file contents.

#### Scenario: At least one provider is ready
- **WHEN** configuration is valid, the selected editor backend and adapter are compatible, and a configured provider is installed and logged in
- **THEN** doctor identifies the usable provider and editor backend and exits successfully

#### Scenario: No provider is installed
- **WHEN** neither configured official provider CLI is available
- **THEN** doctor exits nonzero and lists official provider installation and login as the required next action without asking for an API key

#### Scenario: Provider login is missing
- **WHEN** a provider executable is present but its official login status is not ready
- **THEN** doctor reports that provider as unavailable and points to its official login flow without reading or printing its credentials

#### Scenario: Native keybinding conflict is detected
- **WHEN** a native default `Alt+G`, `Alt+U`, or Enter guard binding conflicts with an existing unsupported custom binding
- **THEN** doctor reports the shell and key conflict rather than silently claiming the adapter is ready

#### Scenario: ble.sh keybinding conflict is detected
- **WHEN** a ble.sh keymap already has an unsupported binding on an `intent-sh` rewrite, undo, or guarded-accept key
- **THEN** doctor reports the backend, keymap, and conflicting key without replacing it silently

#### Scenario: Bash 3 with compatible ble.sh is ready
- **WHEN** doctor runs from an initialized Bash 3.2 session whose adapter reports the exact tested ble.sh backend and matching protocol
- **THEN** it reports conditional Bash support as ready instead of failing only because the Bash major version is below 4

#### Scenario: Bash 3 is missing a usable editor backend
- **WHEN** doctor inspects Bash 3.2 without compatible attached ble.sh capability
- **THEN** it exits nonzero and recommends loading a supported ble.sh version first, using stock Zsh, or installing modern Bash without modifying the system shell

#### Scenario: ble.sh is incompatible
- **WHEN** ble.sh is present but its version, attachment state, or required widget API does not satisfy the tested compatibility contract
- **THEN** doctor reports the failed capability and does not claim the Bash adapter is ready

### Requirement: Fail closed on compatibility problems
The adapter and binary SHALL negotiate their protocol version and active editor backend. Provider compatibility checks SHALL identify unsupported CLI versions or missing required isolation and structured-output features, and ble.sh compatibility checks SHALL identify an unsupported version, detached editor, missing widget API, or invalid load order. An incompatible component MUST NOT perform a rewrite.

#### Scenario: Provider lacks a required capability
- **WHEN** the installed provider version cannot disable tools or produce the required structured result
- **THEN** doctor marks it incompatible and routing skips it in auto mode or fails clearly in explicit mode

#### Scenario: Adapter is newer than the binary
- **WHEN** the loaded adapter requests an unsupported protocol version
- **THEN** the binary returns a compatibility error without parsing remaining fields or producing a replacement

#### Scenario: Reported backend is invalid for the shell
- **WHEN** an adapter request reports native Readline for Bash 3.x or an unknown editor backend
- **THEN** the binary rejects the request as incompatible and emits no replacement

#### Scenario: ble.sh loses required capability
- **WHEN** the ble.sh backend is detached or replaced after adapter initialization
- **THEN** the next interactive action fails closed, preserves the current buffer, and asks the user to reinitialize in the supported order

### Requirement: Document privacy, source installation, and removal
The repository SHALL document supported systems, source installation, adapter activation, Bash editor-backend selection, the optional pinned ble.sh prerequisite and load order, first rewrite, regenerate, undo, cancellation, risk behavior, provider login prerequisites, the exact context sent to providers, explicit non-collected data, and complete removal. Documentation MUST explain that ble.sh is separately maintained code running in the interactive shell, that Bash 3.2 can have performance limitations, that risk detection is heuristic, and that no generated command is automatically executed.

#### Scenario: New user follows the native-shell guide
- **WHEN** a user has a natively supported Zsh or Bash 4.0+ session and one provider CLI installed and logged in
- **THEN** the guide takes them from source build through adapter activation, doctor success, a harmless first rewrite, undo, and removal without requesting a new credential or installing ble.sh

#### Scenario: Bash 3 user follows the ble.sh guide
- **WHEN** a Bash 3.2 user chooses the optional compatibility path
- **THEN** the guide explains the external dependency and trust boundary, points to official ble.sh installation guidance, loads it before `intent-sh`, verifies the selected backend with doctor, and exercises a harmless rewrite without automatic execution

#### Scenario: User removes intent-sh but keeps ble.sh
- **WHEN** a ble.sh user follows the `intent-sh` removal instructions
- **THEN** removing the `intent-sh` activation line and binary disables the integration without modifying the independently managed ble.sh installation

#### Scenario: User removes all optional components
- **WHEN** a user installed ble.sh only for Bash 3 compatibility and chooses complete removal
- **THEN** documentation distinguishes removal of `intent-sh`, its optional secret-free config, and the separately managed ble.sh files without deleting any component automatically
