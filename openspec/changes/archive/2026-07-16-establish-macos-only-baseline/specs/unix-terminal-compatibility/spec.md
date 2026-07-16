## REMOVED Requirements

### Requirement: Operate through a terminal-independent PTY contract
**Reason**: The broad terminal capability is replaced by an explicitly macOS-scoped contract.

**Migration**: Use `macos-terminal-compatibility` requirement `Operate through a macOS terminal-independent PTY contract`.

### Requirement: Preserve editor state across terminal behavior
**Reason**: Editor-state behavior now belongs to the supported macOS terminal boundary.

**Migration**: Use `macos-terminal-compatibility` requirement `Preserve editor state across macOS terminal behavior`.

### Requirement: Support qualified tmux sessions
**Reason**: tmux qualification is retained only for macOS sessions.

**Migration**: Use `macos-terminal-compatibility` requirement `Support qualified macOS tmux sessions`.

### Requirement: Define remote SSH execution locality
**Reason**: Remote qualification now requires a prepared macOS target and protected execution.

**Migration**: Use `macos-terminal-compatibility` requirement `Define macOS remote SSH execution locality`.

### Requirement: Probe terminal key delivery without collecting content
**Reason**: The key-delivery diagnostic is retained within the macOS terminal capability.

**Migration**: Use `macos-terminal-compatibility` requirement `Probe macOS terminal key delivery without collecting content`.

### Requirement: Maintain reproducible terminal qualification evidence
**Reason**: The qualification matrix is narrowed to macOS environments and evidence categories.

**Migration**: Use `macos-terminal-compatibility` requirement `Maintain reproducible macOS terminal qualification evidence`.
