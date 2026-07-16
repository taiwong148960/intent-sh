# Shell Rewrite Workflow Specification

## Purpose

Define the interactive Zsh and Bash experience for rewriting, regenerating, undoing, cancelling, and safely accepting generated commands.

## Requirements

### Requirement: Supported interactive shell adapters
The system SHALL preserve protocol-2 interactive adapters for Zsh and Bash 4.0 or newer on macOS. ZLE and Bash native Readline SHALL bind the configured rewrite chord to rewrite or regenerate and the configured undo chord to undo, with defaults of `Alt+G` and `Alt+U`. Binding values MUST satisfy the shared bounded chord grammar, MUST be distinct, and MUST be rendered through fixed shell-specific encoders rather than evaluated as arbitrary shell text. The adapters MUST use the active native editor's editable-line API and MUST NOT use shell history, the clipboard, terminal-screen scraping, Accessibility APIs, simulated global keystrokes, or a third-party Bash line-editor backend.

#### Scenario: Activate a supported adapter with defaults
- **WHEN** a macOS user with no binding overrides loads the version-compatible adapter in an interactive Zsh or native Bash 4.0+ Readline session
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

### Requirement: Preserve the buffer on clarification and failure
A clarification, provider failure, timeout, invalid response, or local validation failure SHALL leave the editable buffer unchanged. The adapter SHALL show a concise question or actionable failure in shell-native status output and MUST NOT insert provider chatter.

#### Scenario: Provider asks for clarification
- **WHEN** the provider returns a valid `clarify` result
- **THEN** the original line stays editable and the clarification question is displayed without opening a chat UI

#### Scenario: Generated command fails validation
- **WHEN** the provider returns a command that the core rejects
- **THEN** the pre-request buffer remains intact and the adapter reports that no command was applied

### Requirement: Cancel an in-progress rewrite
The foreground rewrite SHALL be cancellable with `Ctrl+C`. Cancellation MUST terminate the active provider process tree, MUST NOT try another provider, and MUST preserve the buffer that was present when generation started.

#### Scenario: Cancel a slow provider
- **WHEN** generation is in progress and the user presses `Ctrl+C`
- **THEN** the core terminates the provider process tree, returns a cancelled status, and the adapter restores normal editing with no replacement

### Requirement: Keep rewrite state local to the shell session
Original input, generated output, provider, risk, request ID, and generation index SHALL exist only in the current shell session. The adapter MUST discard a response whose request ID is not the active request ID.

#### Scenario: Open two shell sessions
- **WHEN** rewrites are performed in two concurrent shell sessions
- **THEN** regenerate, undo, and confirmation state in one session does not affect the other

#### Scenario: Receive a stale response
- **WHEN** a response carries a request ID different from the adapter's active request
- **THEN** the adapter ignores it and leaves the current line unchanged

### Requirement: Leave execution to the user and shell
The core and adapters SHALL only propose and insert commands. They MUST NOT automatically execute a generated command; normal or safety-guarded shell acceptance SHALL occur only after the user presses Enter.

#### Scenario: Successful generation completes
- **WHEN** a safe, review, or dangerous result is inserted
- **THEN** no target command runs until the user explicitly accepts the line according to its risk behavior
