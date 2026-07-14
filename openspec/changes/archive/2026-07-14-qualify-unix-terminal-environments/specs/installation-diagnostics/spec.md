## MODIFIED Requirements

### Requirement: Activate adapters explicitly and reversibly
`intent-sh init zsh|bash` SHALL load and validate the effective rewrite and undo chords before emitting the embedded adapter for the requested shell. `intent-sh setup zsh|bash` SHALL report the appropriate startup file, an idempotent activation line, the effective default or configured bindings, detected static conflicts, and removal guidance, but MUST NOT modify a startup file, shell keymap, terminal preference, tmux configuration, or user configuration by default.

#### Scenario: Request Zsh setup guidance
- **WHEN** the user runs `intent-sh setup zsh` with default configuration
- **THEN** the command prints the exact activation line, target file, `Alt+G` and `Alt+U` defaults, Enter guard, cancellation key, and removal instruction without editing the file

#### Scenario: Request setup with custom chords
- **WHEN** valid custom rewrite and undo chords are present in configuration
- **THEN** setup prints those effective chords and checks the corresponding shell binding forms for static conflicts

#### Scenario: Load an adapter
- **WHEN** a supported interactive shell evaluates `intent-sh init` for its own shell with valid binding configuration
- **THEN** the embedded adapter loads only if its protocol version matches the binary and installs only the effective rewrite and undo chords

#### Scenario: Invalid binding blocks initialization
- **WHEN** the effective binding configuration is invalid
- **THEN** `init` exits nonzero before emitting a partial script and no binding is installed by its output

#### Scenario: Request an unsupported shell
- **WHEN** the user requests setup or initialization for Fish, Nushell, PowerShell, or an unknown shell
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
Ordinary `intent-sh doctor` SHALL check supported platform and architecture, native Bash 4.0+/Zsh compatibility, the existing conditional Bash 3.2 ble.sh contract, config and chord validity, adapter/binary protocol-2 compatibility, effective-key conflicts visible in the selected startup file, configured provider executable and compatible version, and official CLI login readiness. `intent-sh doctor --keys` SHALL additionally perform the bounded opt-in controlling-terminal delivery probe for rewrite, undo, Enter, and cancellation on the native ZLE/Readline qualification paths. Both modes SHALL emit stable check identifiers, actionable guidance, and no tokens, credential-file contents, prompts, shell buffers, history, screen contents, or unbounded received bytes.

#### Scenario: At least one provider is ready
- **WHEN** configuration is valid, the adapter is compatible, effective keys have no detected static conflict, and a configured provider is installed and logged in
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

#### Scenario: Bash lacks a supported editor backend
- **WHEN** doctor inspects Bash older than 4.0 without the exact tested attached ble.sh backend
- **THEN** it reports native Readline as incompatible and recommends the existing conditional ble.sh path, stock Zsh, or an installed modern Bash without modifying the system shell

### Requirement: Document privacy, source installation, removal, and terminal qualification
The repository SHALL document supported systems, the behavioral PTY contract, qualified terminal records, source installation, adapter activation, binding configuration and reset, interactive key probing, tmux and SSH expectations, remote provider locality, first rewrite, regenerate, undo, cancellation, risk behavior, provider login prerequisites, the exact context sent to providers, explicit non-collected data, and complete removal. Documentation MUST distinguish contract-compatible from recorded qualified environments, explain that risk detection is heuristic, and state that no generated command is automatically executed.

#### Scenario: New user follows the default guide
- **WHEN** a user has one supported provider CLI installed and logged in and a terminal that delivers the default chords
- **THEN** the guide takes them from source build through adapter activation, ordinary doctor, optional key probe, a harmless first rewrite, undo, and removal without requesting a new credential

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
