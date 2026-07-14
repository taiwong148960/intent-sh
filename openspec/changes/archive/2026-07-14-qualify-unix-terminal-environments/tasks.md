## 1. Change Sequencing and Key-Chord Foundation

- [x] 1.1 Confirm that `build-intent-sh-mvp` has been synced or archived into main specifications, then rebase these deltas onto the current adapter contract without adding Bash 3, ble.sh, Fish, Nushell, Windows, WSL, PowerShell, or BSD scope.
- [x] 1.2 Add an internal key-chord package that parses and canonicalizes one supported `alt+<printable-ascii>` or `ctrl+<ascii-letter>` chord, rejects reserved and malformed values, and exposes bounded typed results.
- [x] 1.3 Implement terminal-byte, ZLE-binding, Readline-binding, and display-name derivation from parsed chords with table, fuzz, non-ASCII, control-byte, and shell-injection tests.
- [x] 1.4 Extend configuration defaults and strict TOML validation with canonical `rewrite_key = "alt+g"` and `undo_key = "alt+u"`, distinct-action enforcement, reserved-key rejection, atomic `config set`, and complete `config show` tests.
- [x] 1.5 Add regression tests proving binding configuration and terminal metadata do not enter adapter request frames, provider prompts, provider subprocess arguments, or the model-visible environment context.

## 2. Configured Adapter Initialization and Setup

- [x] 2.1 Refactor embedded adapter emission to load configuration before output and render only validated, derived binding placeholders, with tests proving invalid config emits no partial shell script.
- [x] 2.2 Update the Zsh adapter to register the effective rewrite and undo sequences, expose bounded session binding markers, preserve the existing Enter guard, and fail clearly when re-evaluated with different active bindings.
- [x] 2.3 Update the Bash adapter to register the effective Readline sequences while preserving both private Enter-continuation mappings, cancellation handling, and one/two-Enter behavior for custom chords.
- [x] 2.4 Parameterize setup guidance and bounded startup-file conflict inspection by the effective canonical chords for Bash and Zsh, including default, custom, duplicate, reserved, and adversarial quoting fixtures.
- [x] 2.5 Add command-level tests for `init`, `setup`, `config show`, and `config set` that verify default compatibility, custom-key output, terminal-safe messages, exact removal guidance, and no startup-file or terminal-setting mutation.

## 3. Interactive Key-Delivery Diagnostics

- [x] 3.1 Add the supported Unix terminal dependency and a testable controlling-TTY abstraction that opens `/dev/tty`, enters bounded raw mode, applies per-key deadlines and byte limits, and restores the original state on every return path.
- [x] 3.2 Implement the ordered rewrite, undo, CR-or-LF Enter, and `Ctrl+C` probe with stable check IDs, symbolic bounded byte rendering, mismatch/remapping guidance, and no provider or shell-buffer access.
- [x] 3.3 Extend CLI dispatch with `intent-sh doctor --keys` while preserving ordinary non-interactive `doctor` behavior and returning an actionable failure without consuming stdin when no controlling terminal exists.
- [x] 3.4 Add fake-terminal and PTY tests for success, transformed input, excessive bytes, timeout, context cancellation, read failure, EOF, signal handling, and terminal-state restoration after each case.
- [x] 3.5 Add adversarial privacy tests proving the probe never persists received bytes or renders raw control sequences, arbitrary typed text, prompts, credentials, history, or provider diagnostics.

## 4. Reusable Bash and Zsh Terminal Conformance

- [x] 4.1 Refactor the existing Bash/Zsh PTY helpers into a reusable terminal-conformance matrix parameterized by shell, effective chords, `TERM`, CR/LF acceptance, and terminal size.
- [x] 4.2 Run the complete fake-provider lifecycle for default Alt chords and allowed Ctrl alternatives in clean Bash and Zsh sessions, covering rewrite, regenerate, undo, manual edit, clarification, fallback, cancellation, review, dangerous confirmation, and no automatic execution.
- [x] 4.3 Add PTY cases for `dumb`, `xterm-256color`, and `screen-256color` or `tmux-256color`, including resize during slow generation, redraw after response, CR and LF guarding, Unicode buffer preservation, and cursor restoration on failure.
- [x] 4.4 Verify incompatible or intercepted bindings fail diagnostically without changing the editable buffer, and verify two concurrent terminal sessions retain independent rewrite and danger-confirmation state.

## 5. tmux Qualification

- [x] 5.1 Add an isolated tmux test harness with a private server socket and empty configuration, plus reproducible local and CI dependency setup on macOS and Linux.
- [x] 5.2 Run the Bash/Zsh conformance lifecycle inside tmux for default and custom chords, CR/LF, cancellation, resize/repaint, and dangerous two-Enter acceptance without capturing panes or modifying user tmux state.
- [x] 5.3 Add detach/reattach tests proving a live shell retains its visible buffer and session-local rewrite, undo, and confirmation state while other panes and sessions remain independent.
- [x] 5.4 Add an intercepted-root-binding case that demonstrates failed key delivery through the diagnostic path and documents manual tmux-binding or `intent-sh` chord remediation.

## 6. SSH Qualification

- [x] 6.1 Add an opt-in SSH smoke harness that accepts an explicit test target, installs nothing, creates no credentials, records no prompts, and skips cleanly when no target is configured.
- [x] 6.2 Exercise harmless remote Bash and Zsh flows with fake providers, including rewrite, regenerate, undo, cancellation of the remote provider tree, no client-side provider fallback, and no automatic execution.
- [x] 6.3 Add an SSH-to-tmux reattach journey or documented manual equivalent proving surviving state belongs to the live remote shell and requires no client-local `intent-sh` state.
- [x] 6.4 Verify remote context remains the existing boolean only and that SSH marker values, client terminal identity, local provider credentials, and remote terminal contents never enter logs or provider input.

## 7. Compatibility Documentation and Recorded Matrix

- [x] 7.1 Update the README and setup/doctor help with the behavioral PTY contract, canonical chord grammar, defaults, remapping/reset workflow, `doctor --keys`, supported Bash/Zsh boundary, and downgrade cleanup for new strict config keys.
- [x] 7.2 Add a terminal qualification guide and dated result template covering terminal/OS/architecture/shell versions, optional tmux/SSH layer, `TERM`, configured chords, `intent-sh` version, harmless workflow cases, and bounded pass/fail evidence.
- [x] 7.3 Document tmux troubleshooting, SSH remote dependency locality, loss-of-connection semantics, contract-compatible versus qualified terminology, and explicit non-collection of terminal identity, screen, selection, clipboard, history, and credentials.
- [x] 7.4 Run and record the initial representative matrix for the macOS system terminal, another macOS terminal, a Linux desktop terminal, a modern cross-platform or GPU terminal, VS Code's integrated terminal, tmux, and SSH before claiming those categories as qualified.

## 8. Release and Security Verification

- [x] 8.1 Complete a threat-focused review of chord parsing, shell rendering, strict-config rollback, raw-terminal restoration, byte redaction, tmux isolation, SSH locality, and terminal-identity privacy; add a regression test for every finding.
- [x] 8.2 Run formatting, vet, unit, fuzz-seed, fake-provider integration, PTY, tmux, shell-syntax, and supported build checks on macOS and Linux, with opt-in SSH behavior verified both skipped and configured.
- [x] 8.3 Validate the change with strict OpenSpec validation and confirm every new or modified requirement has deterministic automated coverage or an explicit recorded manual qualification step.
- [x] 8.4 Follow the documented default-key, custom-key, key-probe, tmux, SSH, downgrade, and removal journeys from disposable homes and confirm no terminal, tmux, startup, provider-login, or remote-host state is modified unexpectedly.
