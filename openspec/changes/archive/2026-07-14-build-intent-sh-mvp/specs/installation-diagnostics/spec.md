## ADDED Requirements

### Requirement: Build as one supported Go binary
The project SHALL build one `intent-sh` executable for macOS and Linux on amd64 and arm64. The executable SHALL include rewrite, adapter initialization, setup guidance, configuration, doctor, and version commands and SHALL embed protocol-compatible Zsh and Bash adapter assets.

#### Scenario: Build from source
- **WHEN** a developer with the supported Go toolchain runs the documented source build
- **THEN** one executable is produced without requiring a desktop app, database, daemon, or hosted service

#### Scenario: Inspect the version
- **WHEN** the user runs `intent-sh version`
- **THEN** the output identifies the binary version and adapter protocol version

### Requirement: Activate adapters explicitly and reversibly
`intent-sh init zsh|bash` SHALL emit the embedded adapter for the requested shell. `intent-sh setup zsh|bash` SHALL report the appropriate startup file and an idempotent activation line, but MUST NOT modify a startup file by default.

#### Scenario: Request Zsh setup guidance
- **WHEN** the user runs `intent-sh setup zsh`
- **THEN** the command prints the exact activation line, target file, default bindings, and removal instruction without editing the file

#### Scenario: Load an adapter
- **WHEN** a supported interactive shell evaluates `intent-sh init` for its own shell
- **THEN** the embedded adapter loads only if its protocol version matches the binary

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
`intent-sh doctor` SHALL check supported platform and architecture, shell compatibility including the Bash 4.0 minimum, config validity, adapter/binary protocol compatibility, default-key conflicts, configured provider executable and compatible version, and official CLI login readiness. It SHALL emit stable check identifiers, actionable guidance, and no tokens or credential-file contents.

#### Scenario: At least one provider is ready
- **WHEN** the configuration is valid, the adapter is compatible, and a configured provider is installed and logged in
- **THEN** doctor identifies the usable provider and exits successfully

#### Scenario: No provider is installed
- **WHEN** neither configured official provider CLI is available
- **THEN** doctor exits nonzero and lists official provider installation and login as the required next action without asking for an API key

#### Scenario: Provider login is missing
- **WHEN** a provider executable is present but its official login status is not ready
- **THEN** doctor reports that provider as unavailable and points to its official login flow without reading or printing its credentials

#### Scenario: Keybinding conflict is detected
- **WHEN** a default `Alt+G`, `Alt+U`, or Enter guard binding conflicts with an existing unsupported custom binding
- **THEN** doctor reports the shell and key conflict rather than silently claiming the adapter is ready

#### Scenario: Bash is too old
- **WHEN** doctor inspects a Bash version older than 4.0
- **THEN** it reports the shell as incompatible and recommends stock Zsh or an installed modern Bash without modifying the system shell

### Requirement: Fail closed on compatibility problems
The adapter and binary SHALL negotiate their protocol version, and provider compatibility checks SHALL identify unsupported CLI versions or missing required isolation/structured-output features. An incompatible component MUST NOT perform a rewrite.

#### Scenario: Provider lacks a required capability
- **WHEN** the installed provider version cannot disable tools or produce the required structured result
- **THEN** doctor marks it incompatible and routing skips it in auto mode or fails clearly in explicit mode

#### Scenario: Adapter is newer than the binary
- **WHEN** the loaded adapter requests an unsupported protocol version
- **THEN** the binary returns a compatibility error without parsing remaining fields or producing a replacement

### Requirement: Document privacy, source installation, and removal
The repository SHALL document supported systems, source installation, adapter activation, first rewrite, regenerate, undo, cancellation, risk behavior, provider login prerequisites, the exact context sent to providers, explicit non-collected data, and complete removal. Documentation MUST explain that risk detection is heuristic and that no generated command is automatically executed.

#### Scenario: New user follows the MVP guide
- **WHEN** a user has one supported provider CLI installed and logged in
- **THEN** the guide takes them from source build through adapter activation, doctor success, a harmless first rewrite, undo, and removal without requesting a new credential

#### Scenario: User removes the MVP
- **WHEN** the user follows the removal instructions
- **THEN** removing one startup-file activation line and the binary disables the product, with the optional secret-free config safe to delete independently
