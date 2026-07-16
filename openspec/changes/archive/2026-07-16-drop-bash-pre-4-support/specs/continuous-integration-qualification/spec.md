## ADDED Requirements

### Requirement: Qualify tmux hermetically
Required integration jobs SHALL run the isolated tmux lifecycle on macOS and Linux. tmux tests MUST use a private socket and empty configuration, MUST qualify supported Bash 4.0+ native Readline and Zsh sessions, and MUST NOT inspect or mutate user state. CI MUST NOT maintain a Bash-3-specific or alternate-Bash-editor fixture, cache, manifest suite, or required job.

#### Scenario: Reattach an isolated tmux client
- **WHEN** the test client detaches and reattaches to a live Bash or Zsh pane
- **THEN** editable-buffer and shell-local rewrite state survive only in that pane and the test leaves no private tmux server running

#### Scenario: Exclude removed shell fixtures
- **WHEN** required CI constructs its shell and editor matrices
- **THEN** it selects supported native Bash/Zsh cases only and contains no Bash-3 or alternate-Bash-editor fixture dependency

## REMOVED Requirements

### Requirement: Qualify tmux and ble.sh hermetically
**Reason**: The ble.sh backend and all Bash versions below 4.0 are no longer supported, so their external fixture and qualification contract must be removed.

**Migration**: Run the existing native Bash/Zsh and isolated tmux suites with Bash 4.0+ native Readline or Zsh.
