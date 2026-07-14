## MODIFIED Requirements

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

## ADDED Requirements

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
