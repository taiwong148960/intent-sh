## ADDED Requirements

### Requirement: Operate through a terminal-independent PTY contract
The system SHALL support an interactive terminal environment when it provides a controlling PTY, delivers the configured key sequences to a supported Bash or Zsh editor, and supports ordinary shell repaint. Runtime rewrite behavior MUST depend on shell-editor and byte-delivery capabilities rather than terminal application identity, and MUST NOT use terminal-specific APIs, screen contents, selections, clipboard state, Accessibility APIs, or simulated keystrokes.

#### Scenario: Use a conventional local terminal
- **WHEN** a supported Bash or Zsh session runs in a conventional macOS or Linux PTY that delivers the configured keys
- **THEN** rewrite, regenerate, undo, cancellation, and guarded acceptance work without detecting or integrating with the terminal application

#### Scenario: Use an integrated terminal
- **WHEN** a supported shell runs in an integrated terminal that satisfies the same PTY and key-delivery contract
- **THEN** the workflow has the same buffer ownership and no-auto-execution behavior as a standalone terminal

#### Scenario: Terminal identity changes
- **WHEN** the same supported shell workflow is run with a different terminal application or compatible `TERM` description
- **THEN** terminal identity is not added to provider context and does not select a different rewrite implementation

### Requirement: Preserve editor state across terminal behavior
Qualified terminal paths SHALL preserve the complete editable buffer, editor-native cursor restoration, rewrite-chain state, warnings, and dangerous-command confirmation across terminal repaint and resize. The adapters SHALL accept both CR and LF as Enter and MUST preserve the pre-request buffer after a failed redraw, cancellation, clarification, timeout, malformed response, or terminal resize.

#### Scenario: Resize during generation
- **WHEN** the terminal is resized while a provider request is in progress
- **THEN** status output repaints without applying a partial response or changing the pre-request buffer and cursor

#### Scenario: Terminal sends carriage return
- **WHEN** an unchanged dangerous generated command is visible and Enter arrives as CR
- **THEN** the first Enter warns without execution and the second consecutive unchanged Enter delegates to native shell acceptance

#### Scenario: Terminal sends line feed
- **WHEN** an unchanged dangerous generated command is visible and Enter arrives as LF
- **THEN** the first Enter warns without execution and the second consecutive unchanged Enter delegates to native shell acceptance

#### Scenario: Preserve a Unicode buffer
- **WHEN** a buffer contains non-ASCII text and its cursor is not at the end during a terminal failure, cancellation, or resize
- **THEN** the adapter preserves or restores the exact editor buffer and its valid editor-native cursor without splitting a character

### Requirement: Support qualified tmux sessions
The system SHALL preserve the supported Bash/Zsh workflow inside tmux when the outer terminal and isolated tmux configuration deliver the configured sequences. Rewrite state SHALL remain local to the shell process and therefore SHALL survive tmux detach and reattach while that shell remains alive. `intent-sh` MUST NOT capture panes, inspect scrollback, or modify tmux configuration.

#### Scenario: Rewrite inside tmux
- **WHEN** a supported shell inside tmux receives the configured rewrite or undo chord
- **THEN** the adapter performs the same action and safety behavior as it does on a direct PTY

#### Scenario: Detach and reattach
- **WHEN** a user detaches from tmux after generating a command and later reattaches to the same live shell
- **THEN** the visible buffer and shell-session rewrite, undo, and danger-confirmation state remain associated with that shell only

#### Scenario: tmux intercepts a configured chord
- **WHEN** a tmux root binding consumes a configured chord before the shell receives it
- **THEN** the interactive key probe reports failed delivery and documentation explains that the tmux binding or `intent-sh` chord must be changed manually

### Requirement: Define remote SSH execution locality
In a qualified SSH session, the shell adapter, `intent-sh` binary, provider CLI, provider login, current directory, provider process, and generated target command SHALL all belong to the remote host. The system MUST NOT forward provider credentials, shell buffers, or generated commands to a client-side `intent-sh` service. Model context SHALL continue to represent SSH only as a boolean remote signal without sending SSH marker values or local terminal identity.

#### Scenario: Rewrite on a prepared remote host
- **WHEN** a supported remote shell has `intent-sh` and one authenticated provider installed and the SSH path delivers the configured keys
- **THEN** the remote buffer is rewritten and remains unexecuted until the remote user accepts it

#### Scenario: Provider exists only on the client
- **WHEN** the SSH client has a provider login but the remote host has no configured usable provider
- **THEN** remote doctor and rewrite report the missing remote dependency without attempting credential or request forwarding

#### Scenario: Cancel a remote provider
- **WHEN** the user presses `Ctrl+C` during generation over SSH
- **THEN** the remote provider process tree is terminated, fallback stops, and the remote editable buffer remains unchanged

#### Scenario: Reattach to a remote tmux shell
- **WHEN** an SSH connection ends while the shell remains alive in tmux and the user later reconnects and reattaches
- **THEN** any surviving rewrite state belongs to that remote shell session and no client-local state is required

### Requirement: Probe terminal key delivery without collecting content
`intent-sh doctor --keys` SHALL be an explicit interactive diagnostic that reads only bounded key sequences from the controlling terminal in temporary raw mode and compares them with the configured rewrite chord, undo chord, CR or LF, and `Ctrl+C`. It MUST NOT invoke a provider, inspect the active shell buffer, read history or screen contents, persist received bytes, modify configuration, or leave the terminal mode changed after any exit path.

#### Scenario: All configured keys are delivered
- **WHEN** the user runs the probe from a controlling terminal and presses each requested key
- **THEN** the probe emits stable passing check identifiers and restores the original terminal mode

#### Scenario: A key is intercepted or transformed
- **WHEN** received bounded bytes do not match the configured chord
- **THEN** the probe reports the failed action with terminal-safe symbolic byte information and gives manual remapping guidance without changing settings

#### Scenario: Probe has no controlling terminal
- **WHEN** `doctor --keys` runs from a pipe, background job, or environment without an accessible controlling terminal
- **THEN** it exits nonzero with actionable guidance and does not consume ordinary stdin

#### Scenario: Probe is cancelled or times out
- **WHEN** the probe context is cancelled, a read fails, or a key deadline expires
- **THEN** the original terminal mode is restored and no captured byte sequence is retained

### Requirement: Maintain reproducible terminal qualification evidence
The repository SHALL maintain a dated compatibility record that identifies the tested terminal application and version, operating system and architecture, shell and version, optional tmux or SSH layer, `TERM` value, configured chords, `intent-sh` version, and result of the harmless conformance journey. The recorded matrix SHALL cover the macOS system terminal, another macOS terminal, a Linux desktop terminal, a modern cross-platform or GPU terminal, an integrated terminal, tmux, and SSH before those categories are described as qualified.

#### Scenario: Qualify a named terminal environment
- **WHEN** a maintainer completes the documented key probe and harmless workflow checklist in a representative environment
- **THEN** the compatibility record stores bounded non-secret environment metadata and pass/fail results without prompts, credentials, shell history, or terminal contents

#### Scenario: Environment has not been qualified
- **WHEN** a PTY environment satisfies the behavioral contract but has no recorded validation result
- **THEN** documentation describes it as contract-compatible or unverified rather than claiming a named qualification

#### Scenario: Adapter key behavior changes
- **WHEN** a release changes key parsing, adapter registration, Enter guarding, cancellation, or repaint behavior
- **THEN** the affected qualification records are refreshed or marked as belonging to the earlier version
