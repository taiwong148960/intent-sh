## ADDED Requirements

### Requirement: Supported interactive shell adapters
The system SHALL provide interactive adapters for Zsh and Bash 4.0 or newer on macOS and Linux, with `Alt+G` bound to rewrite or regenerate and `Alt+U` bound to undo. The adapters MUST use the shell's editable-line API and MUST NOT use the clipboard, terminal-screen scraping, Accessibility APIs, or simulated global keystrokes.

#### Scenario: Activate a supported adapter
- **WHEN** a user loads the version-compatible adapter in an interactive Zsh or Bash session
- **THEN** the rewrite and undo widgets are available on the default bindings without replacing the terminal application

#### Scenario: Reject stock macOS Bash 3.2
- **WHEN** a user attempts to initialize the Bash adapter in Bash older than 4.0
- **THEN** initialization fails before installing bindings and explains that the shell lacks the required `READLINE_LINE` and `READLINE_POINT` interface

#### Scenario: Reject an incompatible adapter protocol
- **WHEN** an adapter and binary report different protocol versions
- **THEN** the adapter leaves the current buffer unchanged and displays an actionable compatibility error

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
