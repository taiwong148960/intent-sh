## Why

The MVP proves the Bash and Zsh workflows in clean pseudo-terminals, but it does not yet define or verify a real Unix terminal compatibility contract. Fixed Meta bindings, terminal-specific key handling, multiplexers, remote sessions, and redraw behavior can make an otherwise supported shell unreliable without actionable diagnostics.

## What Changes

- Define a terminal-agnostic support contract for interactive Bash 4.0+ and Zsh sessions on macOS and Linux using conventional PTYs, integrated terminals, tmux, and SSH.
- Turn `Alt+G` and `Alt+U` into configurable defaults using a bounded, shell-independent chord syntax while preserving the existing dangerous-command Enter guard and `Ctrl+C` cancellation behavior.
- Add an explicit, opt-in interactive key-delivery probe that verifies the configured rewrite, undo, Enter, and cancellation sequences without invoking a provider, reading shell history, or capturing terminal contents.
- Add deterministic PTY and tmux coverage for key delivery, CR/LF acceptance, resize and redraw behavior, detach/reattach, cancellation, and exact buffer ownership; add a repeatable manual qualification record for representative macOS, Linux, and integrated terminal applications plus SSH sessions.
- Extend setup, doctor guidance, configuration validation, and documentation with supported-environment criteria, key-conflict/remapping help, certified combinations, and clear local-versus-remote dependency expectations.
- Keep new shell adapters, Fish, Nushell, Windows, WSL, PowerShell, BSD support, terminal-screen capture, clipboard access, Accessibility APIs, and terminal-specific runtime integrations outside this change.

## Capabilities

### New Capabilities

- `unix-terminal-compatibility`: Behavioral compatibility contract and qualification evidence for conventional macOS/Linux PTYs, integrated terminals, tmux, and SSH.

### Modified Capabilities

- `shell-rewrite-workflow`: Treat the current rewrite and undo bindings as validated configurable defaults and require workflow parity across qualified local, multiplexed, and remote terminal paths.
- `installation-diagnostics`: Validate key configuration, provide an opt-in interactive key-delivery probe, and document qualified terminal environments and remediation without changing terminal or shell configuration automatically.

## Impact

The change affects the secret-free configuration schema, `init` and `setup` output, doctor command handling, embedded Bash/Zsh bindings, terminal-safe rendering, PTY/tmux integration tests, CI dependencies, and compatibility documentation. It does not change provider routing, prompts, provider authentication, model-visible context, command validation policy, automatic-execution boundaries, or the adapter frame shape. The change follows `build-intent-sh-mvp`; any concurrent editor-backend change must rebase its binding and conformance behavior onto the resulting contract.
