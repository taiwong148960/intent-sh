# macOS Terminal Compatibility Specification

## Purpose

Define the macOS PTY contract, editor-state guarantees, qualified tmux and SSH behavior, bounded key diagnostics, and reproducible terminal evidence.

## Requirements

### Requirement: Operate through a macOS terminal-independent PTY contract
The system SHALL support an interactive macOS terminal environment when it provides a controlling PTY, delivers configured sequences to supported Bash or Zsh, and supports ordinary shell repaint. Runtime behavior MUST depend on shell-editor and byte-delivery capabilities rather than application identity and MUST NOT use terminal-specific APIs, screen contents, selections, clipboard state, Accessibility APIs, or simulated keystrokes.

#### Scenario: Use a conventional local macOS terminal
- **WHEN** a supported shell runs in a macOS PTY that delivers the configured keys
- **THEN** rewrite, regenerate, undo, cancellation, and guarded acceptance work without terminal integration

#### Scenario: Use an integrated macOS terminal
- **WHEN** a supported shell runs in an integrated macOS terminal satisfying the same contract
- **THEN** it keeps the same buffer ownership and no-auto-execution behavior

#### Scenario: Terminal identity changes
- **WHEN** the same workflow uses another macOS application or compatible `TERM`
- **THEN** identity is not added to provider context and selects no different implementation

### Requirement: Preserve editor state across macOS terminal behavior
Qualified macOS paths SHALL preserve the complete editable buffer, editor-native cursor restoration, rewrite-chain state, warnings, and dangerous confirmation across repaint and resize. Adapters SHALL accept CR and LF as Enter and MUST preserve the pre-request buffer after redraw failure, cancellation, clarification, timeout, malformed response, or resize. Character-aware journeys MUST use an explicit verified UTF-8 locale.

#### Scenario: Resize during generation
- **WHEN** the terminal resizes during a provider request
- **THEN** status repaints without applying a partial response or changing buffer/cursor

#### Scenario: Terminal sends carriage return
- **WHEN** an unchanged dangerous command is visible and Enter arrives as CR
- **THEN** the first Enter warns and the second unchanged Enter delegates to native acceptance

#### Scenario: Terminal sends line feed
- **WHEN** an unchanged dangerous command is visible and Enter arrives as LF
- **THEN** the first Enter warns and the second unchanged Enter delegates to native acceptance

#### Scenario: Preserve a Unicode buffer
- **WHEN** a UTF-8 buffer has non-ASCII text and a non-terminal cursor during failure, cancellation, or resize
- **THEN** the exact editor buffer/cursor return and the protocol cursor remains on a UTF-8 byte boundary

### Requirement: Support qualified macOS tmux sessions
The system SHALL preserve the Bash/Zsh workflow inside tmux on macOS when the outer terminal and isolated tmux deliver the sequences. Rewrite state SHALL remain local to the shell and survive detach/reattach while it lives. `intent-sh` MUST NOT capture panes, inspect scrollback, or modify tmux.

#### Scenario: Rewrite inside tmux
- **WHEN** a supported macOS shell inside tmux receives rewrite or undo
- **THEN** it performs the same action and safety behavior as a direct PTY

#### Scenario: Detach and reattach
- **WHEN** a user detaches after generation and reattaches to the same shell
- **THEN** visible buffer, rewrite, undo, and confirmation state remain with that shell only

#### Scenario: tmux intercepts a chord
- **WHEN** a root binding consumes a chord before the shell receives it
- **THEN** the key probe reports failed delivery and documentation gives manual remediation

### Requirement: Define macOS remote SSH execution locality
In a qualified macOS SSH session, the adapter, binary, provider CLI/login/process, directory, and generated target command SHALL belong to the prepared remote Mac. The harness MUST verify Darwin before staging or execution. The system MUST NOT forward credentials, buffers, or commands to a client-side service. Model context SHALL represent SSH only as a boolean without marker values or client terminal identity.

#### Scenario: Rewrite on a prepared macOS remote host
- **WHEN** the remote Mac has a supported shell, binary, authenticated provider, and delivered keys
- **THEN** the remote buffer is rewritten and remains unexecuted until remote acceptance

#### Scenario: Provider exists only on the client
- **WHEN** the client has a provider login but the remote Mac has none
- **THEN** remote readiness/rewrite reports the missing dependency without forwarding

#### Scenario: Cancel a remote provider
- **WHEN** the user presses `Ctrl+C` during remote generation
- **THEN** the remote provider tree stops, fallback stops, and the remote buffer stays unchanged

#### Scenario: Reattach to remote tmux
- **WHEN** SSH ends while the remote shell lives in tmux and the user reconnects
- **THEN** surviving state belongs to that remote shell and needs no client-local state

### Requirement: Probe macOS terminal key delivery without collecting content
`intent-sh doctor --keys` SHALL explicitly read only bounded key sequences from the macOS controlling terminal in temporary raw mode and compare rewrite, undo, CR/LF, and `Ctrl+C`. It MUST NOT invoke a provider, inspect the shell buffer, read history/screen contents, persist bytes, change configuration, or leave terminal mode changed.

#### Scenario: All configured keys are delivered
- **WHEN** the user presses each requested key
- **THEN** stable passing identifiers are emitted and terminal mode is restored

#### Scenario: A key is intercepted or transformed
- **WHEN** bounded bytes do not match
- **THEN** the probe reports symbolic terminal-safe information and manual guidance without changing settings

#### Scenario: Probe has no controlling terminal
- **WHEN** it runs without an accessible controlling terminal
- **THEN** it exits nonzero without consuming ordinary stdin

#### Scenario: Probe is cancelled or times out
- **WHEN** context cancellation, read failure, or deadline occurs
- **THEN** terminal mode is restored and no captured sequence remains

### Requirement: Maintain reproducible macOS terminal qualification evidence
The repository SHALL keep dated records of application/version, macOS version/architecture, shell/version, optional tmux/SSH layer, `TERM`, chords, candidate version, and harmless conformance result. Before a category is called qualified, records SHALL cover the macOS system terminal, another macOS application, an integrated macOS terminal, tmux on macOS, and every claimed macOS-to-macOS SSH path. Generic PTY success and static artifact inspection MUST NOT be presented as named-terminal, remote-host, or native-architecture qualification.

#### Scenario: Qualify a named macOS terminal
- **WHEN** a maintainer completes the key probe and harmless checklist
- **THEN** the record stores bounded non-secret metadata and results without prompts, credentials, history, or terminal contents

#### Scenario: Environment has not been qualified
- **WHEN** a macOS environment satisfies the behavioral contract without a dated result
- **THEN** documentation calls it compatible or unverified rather than qualified

#### Scenario: Adapter key behavior changes
- **WHEN** a release changes key parsing, registration, Enter guarding, cancellation, or repaint
- **THEN** affected macOS records are refreshed or marked as earlier-version evidence
