## MODIFIED Requirements

### Requirement: Supported interactive shell adapters
The system SHALL preserve protocol-2 interactive adapters for Zsh and Bash on macOS and Linux, including the existing conditional Bash 3.2 ble.sh path. ZLE and Bash 4.0+ native Readline SHALL bind the configured rewrite chord to rewrite or regenerate and the configured undo chord to undo, with defaults of `Alt+G` and `Alt+U`; this change SHALL NOT expand or alter ble.sh binding behavior. Binding values MUST satisfy the shared bounded chord grammar, MUST be distinct, and MUST be rendered through fixed shell-specific encoders rather than evaluated as arbitrary shell text. The adapters MUST use the active editor's editable-line API and MUST NOT use shell history, the clipboard, terminal-screen scraping, Accessibility APIs, or simulated global keystrokes.

#### Scenario: Activate a supported adapter with defaults
- **WHEN** a user with no binding overrides loads the version-compatible adapter in an interactive Zsh or Bash session
- **THEN** rewrite and undo are available on `Alt+G` and `Alt+U` without replacing the terminal application

#### Scenario: Activate configured bindings
- **WHEN** the user configures two valid distinct supported chords and loads the adapter in a new interactive shell
- **THEN** rewrite and undo are registered on the derived ZLE or Readline sequences and the defaults are not additionally installed by `intent-sh`

#### Scenario: Reject an invalid binding
- **WHEN** either configured chord is malformed, reserved, non-ASCII, unsupported, or equal to the other action's chord
- **THEN** initialization fails before emitting or installing partial adapter bindings and reports the exact configuration field to correct

#### Scenario: Reject stock macOS Bash 3.2
- **WHEN** a user attempts to initialize the Bash adapter in Bash 3.2 without the exact tested attached ble.sh backend
- **THEN** initialization fails before installing bindings and explains that native rewrite requires Bash 4.0+ while the existing conditional Bash 3.2 path requires the tested ble.sh editor

#### Scenario: Preserve the existing ble.sh contract
- **WHEN** the exact tested ble.sh backend is active in a previously supported Bash session
- **THEN** protocol-2 negotiation and its existing fixed bindings continue to work without gaining new terminal-qualification or configurable-binding behavior from this change

#### Scenario: Reject an incompatible adapter protocol
- **WHEN** an adapter and binary report different protocol versions
- **THEN** the adapter leaves the current buffer unchanged and displays an actionable compatibility error

### Requirement: Rewrite the complete current buffer
On an initial rewrite action, the adapter SHALL send the complete editable buffer, cursor position, and supported shell context to the core. After a successful validated result, it SHALL replace the complete buffer with the command, move the cursor to the end, and preserve the pre-rewrite buffer as the original.

#### Scenario: Rewrite natural-language input
- **WHEN** the buffer contains `Find out which process is using port 8080` and the user presses the configured rewrite chord
- **THEN** the complete buffer is replaced by one validated shell command and that command is not executed

#### Scenario: Rewrite mixed input
- **WHEN** the buffer contains an existing shell fragment followed by a natural-language instruction
- **THEN** the complete mixed buffer is supplied as intent and the successful result replaces the complete line

#### Scenario: Reject empty input locally
- **WHEN** the editable buffer is empty or contains only whitespace and the user presses the configured rewrite chord
- **THEN** no provider is invoked, the buffer remains unchanged, and the adapter displays a concise message

### Requirement: Regenerate from the preserved original
If the active buffer exactly matches the last generated command, another configured rewrite action SHALL request an alternative using the preserved original intent, the previous command, and an incremented generation index. The system MUST NOT reinterpret the last generated command as the original intent.

#### Scenario: Generate an alternative
- **WHEN** the user presses the configured rewrite chord again without editing the generated command
- **THEN** the provider receives the original pre-rewrite buffer and previous command and a successful materially different command replaces the line

#### Scenario: Manual editing starts a new chain
- **WHEN** the user changes any part of a generated command and then presses the configured rewrite chord
- **THEN** the previous rewrite state is cleared and the edited line becomes the original input for a new rewrite chain

### Requirement: Undo without overwriting user edits
The configured undo action SHALL restore the original buffer only when the current buffer still exactly matches the active generated command. Undo SHALL clear the generation and danger-confirmation state.

#### Scenario: Restore the original input
- **WHEN** the current line is an unchanged generated command and the user presses the configured undo chord
- **THEN** the adapter restores the exact original buffer and its valid cursor position

#### Scenario: Preserve a manually edited command
- **WHEN** the generated command has been manually edited before the user presses the configured undo chord
- **THEN** the adapter does not overwrite the edited line and clears stale generated state
