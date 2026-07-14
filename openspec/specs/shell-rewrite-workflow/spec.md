# Shell Rewrite Workflow Specification

## Purpose

Define the interactive Zsh and Bash experience for rewriting, regenerating, undoing, cancelling, and safely accepting generated commands.

## Requirements

### Requirement: Supported interactive shell adapters
The system SHALL provide interactive adapters for Zsh and Bash on macOS and Linux. Bash 4.0 or newer SHALL work through the native Readline backend when no supported replacement editor is active. Bash 3.2 SHALL work only through the exact tested, attached ble.sh backend. Bash 3.0 and 3.1 are unsupported. The adapters SHALL bind `Alt+G` to rewrite or regenerate and `Alt+U` to undo, MUST use the active editor's supported editable-line API, and MUST NOT use shell history, the clipboard, terminal-screen scraping, Accessibility APIs, or simulated keystrokes to capture or replace the buffer.

#### Scenario: Activate a native adapter
- **WHEN** a user loads the version-compatible adapter in Zsh or in Bash 4.0 or newer without ble.sh attached
- **THEN** the adapter selects the native ZLE or Readline backend and installs the rewrite and undo widgets on the default bindings

#### Scenario: Activate stock macOS Bash with ble.sh
- **WHEN** a user running Bash 3.2 loads a compatible ble.sh editor before initializing `intent-sh`
- **THEN** the adapter selects the ble.sh backend and installs rewrite, undo, and guarded-accept widgets without replacing the terminal application

#### Scenario: Use ble.sh in modern Bash
- **WHEN** compatible ble.sh is the active editor in Bash 4.0 or newer
- **THEN** the adapter selects the ble.sh backend instead of installing bindings against inactive native Readline state

#### Scenario: Reject Bash 3 without ble.sh
- **WHEN** a user initializes the Bash adapter in Bash 3.x without a compatible attached ble.sh editor
- **THEN** initialization fails before installing any binding and explains how to load ble.sh first, use stock Zsh, or install a modern Bash

#### Scenario: Reject an unsupported Bash generation
- **WHEN** a user initializes the adapter in Bash older than 3.2
- **THEN** initialization fails before installing bindings and reports Bash 3.2 as the conditional minimum

#### Scenario: Reject an incompatible adapter protocol
- **WHEN** an adapter and binary report different protocol versions
- **THEN** the adapter leaves the current buffer unchanged and displays an actionable compatibility error

### Requirement: Select the Bash editor backend by active capability
The Bash adapter SHALL identify the editor that owns the active line before installing bindings. It SHALL select the ble.sh backend only when the required ble.sh version and widget APIs are attached, SHALL otherwise select native Readline only on Bash 4.0 or newer, and MUST fail closed when neither backend is available. Backend identity SHALL be included in adapter compatibility negotiation and session diagnostics.

#### Scenario: ble.sh is loaded before intent-sh
- **WHEN** compatible ble.sh owns the active Bash editor when `intent-sh` initializes
- **THEN** initialization records the `blesh` backend and uses ble.sh binding and accept-line APIs

#### Scenario: ble.sh is loaded in the wrong order
- **WHEN** Bash 3.x evaluates the `intent-sh` activation before ble.sh is attached
- **THEN** initialization installs no partial bindings and reports the required activation order

#### Scenario: No history-based fallback is available
- **WHEN** neither a native editable-buffer API nor a compatible ble.sh API is available
- **THEN** the adapter refuses interactive rewrite rather than accepting the line as a comment, reading it back from history, injecting keystrokes, or evaluating generated output

### Requirement: Preserve workflow and safety parity in the ble.sh backend
The ble.sh backend SHALL expose the complete current buffer and cursor to the existing rewrite protocol and SHALL preserve the same original-buffer, regeneration, undo, request-ID, failure, cancellation, and risk state semantics as the native adapters. It SHALL implement dangerous-command confirmation as a ble.sh editor widget that blocks the first Enter for an unchanged dangerous result and delegates the second unchanged Enter to ble.sh's normal acceptance path. It MUST NOT execute generated output itself.

#### Scenario: Rewrite and undo in Bash 3.2
- **WHEN** a Bash 3.2 user rewrites a non-empty buffer through ble.sh and then presses `Alt+U` without editing the generated command
- **THEN** the validated result first replaces the complete line without executing and undo later restores the exact original buffer and cursor

#### Scenario: Manual editing invalidates ble.sh state
- **WHEN** a user changes any part of a generated command in the ble.sh editor
- **THEN** regenerate, undo, and dangerous-confirmation logic treat the edited buffer as user-owned and do not apply stale state

#### Scenario: Dangerous result requires two Enters
- **WHEN** an unchanged dangerous generated command is visible in the ble.sh editor
- **THEN** the first Enter leaves it editable and warns, while the second consecutive unchanged Enter delegates to ble.sh's normal accept-line behavior

#### Scenario: Cancel a ble.sh rewrite
- **WHEN** a Bash 3.2 user presses `Ctrl+C` while a provider is running
- **THEN** the provider process tree is terminated, fallback stops, the pre-request buffer is preserved, and ble.sh returns to normal editing

#### Scenario: Rewrite a Unicode buffer
- **WHEN** a ble.sh buffer contains non-ASCII text and the cursor is not at the beginning
- **THEN** the adapter reports a protocol-consistent cursor position, replaces the complete buffer correctly, and can restore the original editor-native cursor without splitting a character

### Requirement: Rewrite the complete current buffer
On an initial rewrite, the adapter SHALL send the complete editable buffer, cursor position, and supported shell context to the core. After a successful validated result, it SHALL replace the complete buffer with the command, move the cursor to the end, and preserve the pre-rewrite buffer as the original.

#### Scenario: Rewrite natural-language input
- **WHEN** the buffer contains `Find out which process is using port 8080` and the user presses `Alt+G`
- **THEN** the complete buffer is replaced by one validated shell command and that command is not executed

#### Scenario: Rewrite mixed input
- **WHEN** the buffer contains an existing shell fragment followed by a natural-language instruction
- **THEN** the complete mixed buffer is supplied as intent and the successful result replaces the complete line

#### Scenario: Reject empty input locally
- **WHEN** the editable buffer is empty or contains only whitespace and the user presses `Alt+G`
- **THEN** no provider is invoked, the buffer remains unchanged, and the adapter displays a concise message

### Requirement: Regenerate from the preserved original
If the active buffer exactly matches the last generated command, another `Alt+G` SHALL request an alternative using the preserved original intent, the previous command, and an incremented generation index. The system MUST NOT reinterpret the last generated command as the original intent.

#### Scenario: Generate an alternative
- **WHEN** the user presses `Alt+G` again without editing the generated command
- **THEN** the provider receives the original pre-rewrite buffer and previous command and a successful materially different command replaces the line

#### Scenario: Manual editing starts a new chain
- **WHEN** the user changes any part of a generated command and then presses `Alt+G`
- **THEN** the previous rewrite state is cleared and the edited line becomes the original input for a new rewrite chain

### Requirement: Undo without overwriting user edits
`Alt+U` SHALL restore the original buffer only when the current buffer still exactly matches the active generated command. Undo SHALL clear the generation and danger-confirmation state.

#### Scenario: Restore the original input
- **WHEN** the current line is an unchanged generated command and the user presses `Alt+U`
- **THEN** the adapter restores the exact original buffer and its valid cursor position

#### Scenario: Preserve a manually edited command
- **WHEN** the generated command has been manually edited before the user presses `Alt+U`
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
