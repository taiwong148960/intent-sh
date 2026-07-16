## MODIFIED Requirements

### Requirement: Supported interactive shell adapters
The system SHALL preserve protocol-2 interactive adapters for Zsh and Bash 4.0 or newer on macOS and Linux. ZLE and Bash native Readline SHALL bind the configured rewrite chord to rewrite or regenerate and the configured undo chord to undo, with defaults of `Alt+G` and `Alt+U`. Binding values MUST satisfy the shared bounded chord grammar, MUST be distinct, and MUST be rendered through fixed shell-specific encoders rather than evaluated as arbitrary shell text. The adapters MUST use the active native editor's editable-line API and MUST NOT use shell history, the clipboard, terminal-screen scraping, Accessibility APIs, simulated global keystrokes, or a third-party Bash line-editor backend.

#### Scenario: Activate a supported adapter with defaults
- **WHEN** a user with no binding overrides loads the version-compatible adapter in an interactive Zsh or native Bash 4.0+ Readline session
- **THEN** rewrite and undo are available on `Alt+G` and `Alt+U` without replacing the terminal application

#### Scenario: Activate configured bindings
- **WHEN** the user configures two valid distinct supported chords and loads a ZLE or Readline adapter in a new interactive shell
- **THEN** rewrite and undo are registered on the derived ZLE or Readline sequences and the defaults are not additionally installed by `intent-sh`

#### Scenario: Reject an invalid binding
- **WHEN** either configured chord is malformed, reserved, non-ASCII, unsupported, or equal to the other action's chord
- **THEN** initialization fails before emitting or installing partial adapter bindings and reports the exact configuration field to correct

#### Scenario: Reject an unsupported Bash generation
- **WHEN** a user initializes the Bash adapter in Bash older than 4.0
- **THEN** initialization fails before editor selection or binding installation and reports Bash 4.0 as the minimum

#### Scenario: Reject an incompatible adapter protocol
- **WHEN** an adapter and binary report different protocol versions
- **THEN** the adapter leaves the current buffer unchanged and displays an actionable compatibility error

### Requirement: Select the Bash editor backend by active capability
The Bash adapter SHALL verify Bash 4.0 or newer before installing bindings and SHALL use native Readline as its only editor backend. The adapter and binary MUST reject any other reported Bash editor backend before provider invocation. Backend identity SHALL remain included in adapter compatibility negotiation and session diagnostics.

#### Scenario: Initialize native Bash Readline
- **WHEN** Bash 4.0+ initializes `intent-sh` in its native Readline editor
- **THEN** initialization records the `readline` backend and installs the configured Readline bindings

#### Scenario: Reject an old Bash before binding
- **WHEN** Bash older than 4.0 initializes `intent-sh`
- **THEN** the version check rejects the session before binding installation

#### Scenario: Reject a non-native Bash backend
- **WHEN** an adapter request reports a Bash editor backend other than `readline`
- **THEN** the binary rejects it as incompatible before invoking a provider or returning a replacement

#### Scenario: No history-based fallback is available
- **WHEN** the native editable-buffer API is unavailable
- **THEN** the adapter refuses interactive rewrite rather than accepting the line as a comment, reading it back from history, injecting keystrokes, evaluating generated output, or switching to another editor backend

## REMOVED Requirements

### Requirement: Preserve workflow and safety parity in the ble.sh backend
**Reason**: The third-party ble.sh editor backend is removed; Bash 4.0+ native Readline supplies the supported editable-buffer contract without a second integration stack.

**Migration**: Use `intent-sh` from a Bash 4.0+ session using native Readline, or use the supported Zsh ZLE adapter.
