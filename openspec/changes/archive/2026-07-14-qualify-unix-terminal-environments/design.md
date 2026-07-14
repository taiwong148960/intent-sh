## Context

The current baseline implements editor integration through Zsh ZLE, Bash 4.0+ native Readline, and the separately qualified optional ble.sh backend introduced after the MVP. Its automated acceptance suite launches clean shells behind a pseudo-terminal, injects the expected `Escape+g`, `Escape+u`, CR, LF, and `Ctrl+C` bytes, and deliberately uses a minimal `TERM` value. This proves adapter behavior after bytes reach the shell, but it does not prove that a real terminal application, integrated terminal, tmux layer, or SSH path delivers those bytes consistently or redraws the prompt without corrupting the editable buffer.

The product boundary must remain at the shell editor. `intent-sh` does not need terminal application APIs, screen contents, selections, clipboard state, or terminal identity to rewrite a line. The practical compatibility risks are instead key interception or encoding, fixed binding conflicts, nested terminal layers, terminal resize and repaint, controlling-TTY availability, and ambiguity about where the binary and authenticated provider run in an SSH session.

This change follows the archived `build-intent-sh-mvp` and `support-bash-3-via-blesh` changes and modifies capabilities introduced there. The current adapter contract is protocol 2 with explicit editor-backend/version fields and UTF-8 byte cursor offsets. This design preserves that frame and the existing ble.sh behavior while limiting new configurable-binding and terminal-qualification implementation to ZLE and native Readline.

## Goals / Non-Goals

**Goals:**

- Define support in terms of observable PTY and shell-editor behavior rather than terminal brand detection.
- Let users remap rewrite and undo through a small, safe, shell-independent key-chord grammar while retaining `Alt+G` and `Alt+U` as defaults.
- Verify from the controlling terminal that configured rewrite, undo, Enter, and cancellation bytes are delivered as expected without invoking a provider.
- Exercise the full existing Bash/Zsh workflow under representative `TERM` values, tmux, terminal resize, CR/LF acceptance, and detach/reattach.
- Provide a repeatable, dated qualification record for representative macOS, Linux, integrated-terminal, and SSH journeys.
- Keep terminal diagnostics local and preserve the existing privacy and no-auto-execution boundaries.

**Non-Goals:**

- New Fish, Nushell, PowerShell, Windows, WSL, BSD, Bash 3, ble.sh, or other editor-backend behavior; the already supported conditional ble.sh path remains unchanged.
- Reading terminal scrollback, selections, screen cells, clipboard data, shell history, or application-specific settings.
- Detecting or maintaining runtime branches for Terminal.app, iTerm2, VS Code, Kitty, WezTerm, Alacritty, Ghostty, GNOME Terminal, Konsole, or other terminal brands.
- Bridging a local provider login into a remote SSH shell or adding a daemon, socket service, or credential forwarding.
- Arbitrary raw byte bindings, multi-stroke key sequences, function keys, mouse gestures, or modifier protocols beyond the bounded chord grammar.
- Changing the provider request, model prompt, adapter frame, command parser, risk rules, or execution guard semantics.

## Decisions

### 1. Specify terminal support as a behavioral PTY contract

A terminal environment is compatible when it provides an interactive controlling PTY, delivers the configured chord sequences plus CR or LF and `Ctrl+C`, supports ordinary shell-editor repaint, and leaves the current shell process alive across any claimed multiplexer detach/reattach journey. The runtime will not branch on `TERM`, `TERM_PROGRAM`, `WT_SESSION`, tmux variables, application names, or terminal versions.

Compatibility documentation will separate two levels:

1. **Contract-compatible:** an environment satisfies the behavioral checks, including the interactive key probe and harmless workflow validation.
2. **Qualified:** the repository records a dated pass for a named terminal, OS, architecture, shell/version, optional tmux/SSH layer, and `intent-sh` version.

This keeps the support statement extensible without implying that every terminal brand receives custom integration. Terminal names and versions remain local validation metadata and are never added to model-visible context.

Alternatives considered: a hardcoded terminal registry becomes stale and cannot account for user key settings; terminal extensions or proprietary APIs fragment the implementation; treating every PTY as supported without a probe gives users no way to distinguish terminal interception from an adapter failure.

### 2. Add a bounded canonical grammar for rewrite and undo chords

The secret-free TOML configuration will add `rewrite_key` and `undo_key`, defaulting to `alt+g` and `alt+u`. Values are normalized to lowercase and must describe one ASCII key with exactly one supported modifier:

- `alt+<printable-ascii-key>`; or
- `ctrl+<ascii-letter>`.

The two actions must use distinct chords. The validator will reject control chords reserved for Enter, cancellation, terminal flow control or signals, EOF, and the Bash guard's private continuation keys. It will also reject whitespace, non-ASCII text, Shift/Super/Command modifiers, raw escapes, multi-key sequences, and values above a small fixed length.

The core key package will parse a canonical chord once and derive its expected terminal bytes plus the safely quoted ZLE and Readline binding forms. `intent-sh init` will load and validate configuration before emitting any adapter text, then render only these derived bounded forms into fixed adapter placeholders. The adapters will not evaluate arbitrary configuration strings. `setup`, static conflict inspection, `config show|set`, ordinary doctor, and the interactive probe will all use the same parser.

Defaults preserve current behavior. A custom binding is explicit user intent, but setup and doctor will still report a statically visible startup-file conflict. The adapter will continue to own CR and LF for dangerous confirmation and `Ctrl+C` for in-flight cancellation; those keys are not configurable in P0.

Alternatives considered: shell-native raw strings expose incompatible quoting and injection surfaces; environment-only overrides are hard to inspect and reproduce; arbitrary byte sequences complicate Escape timing and conflict detection; per-shell config duplicates user intent and makes Bash/Zsh behavior drift.

### 3. Make key delivery an explicit opt-in doctor mode

`intent-sh doctor` remains non-interactive. `intent-sh doctor --keys` runs the ordinary checks and, when attached to a controlling terminal, opens `/dev/tty`, switches it temporarily to raw mode, and asks the user to press the configured rewrite chord, undo chord, Enter, and `Ctrl+C` in that order. It compares bounded input to the canonical expected bytes; Enter accepts CR or LF. The probe never invokes an adapter action or provider.

The implementation will use a small terminal-state abstraction backed by `golang.org/x/term` on supported Unix hosts. Every exit path restores the original terminal state: success, mismatch, timeout, read error, context cancellation, and handled termination signals. Each prompt has a deadline and a maximum byte count. Received bytes are rendered only as bounded symbolic names or hexadecimal escapes, never as arbitrary terminal text, and are discarded after comparison.

Stable check IDs will distinguish controlling-TTY availability, rewrite delivery, undo delivery, Enter delivery, cancellation delivery, and terminal restoration. A mismatch reports whether the terminal likely intercepted or transformed the chord and points to `config set rewrite_key|undo_key`; it never edits terminal preferences or the config automatically. Non-interactive invocation returns an actionable failure for `--keys` while ordinary doctor behavior remains scriptable.

Alternatives considered: a shell widget can test the editor binding but cannot reliably distinguish a terminal interception from another shell mapping; reading ordinary stdin may consume a pipe rather than the user's terminal; automatically capturing all keystrokes creates an unnecessary privacy surface; modifying terminal application settings is outside the product boundary.

### 4. Render configured bindings at initialization without changing the adapter protocol

The protocol-2 adapter request already reports shell and editor name/version and carries no keybinding fields because the provider and safety engine do not need them. Binding configuration is resolved only while emitting the embedded adapter and is stored in namespaced shell-session variables for status and diagnostics. The NUL-framed protocol remains version 2 for this change.

Zsh will register the derived sequence in the active ZLE keymap used by the current MVP and Bash will use the derived Readline sequence with `bind -x`. Rewrite, regeneration, undo, manual-edit invalidation, cancellation, and dangerous Enter state remain the same functions; only registration and user-facing key names become data-driven. Re-evaluating an already loaded adapter with different configured bindings will fail with guidance to start a new shell or explicitly unload/reinitialize, rather than leaving both old and new bindings active.

Static startup-file conflict inspection will be parameterized by the configured sequences. Runtime binding behavior is verified by adapter PTY tests and the harmless qualification journey; P0 does not attempt to classify every plugin-defined runtime keymap.

Alternatives considered: adding key fields to every rewrite frame leaks irrelevant state into a versioned boundary; generating separate adapter assets for each key is unnecessary; mutating bindings in an already active session risks stale old mappings and ambiguous rollback.

### 5. Use a qualification pyramid instead of a full terminal-product matrix

The automated layers will be:

1. Unit tests for chord parsing, reserved keys, byte derivation, shell quoting, configuration, and terminal-safe rendering.
2. Existing PTY workflows repeated for default and custom chords under representative `TERM` values such as `dumb`, `xterm-256color`, and `screen-256color` or `tmux-256color`.
3. A deterministic tmux suite launched with an isolated server/config that covers Meta and Ctrl bindings, CR/LF, resize/repaint, dangerous confirmation, cancellation, and detach/reattach with shell-session state intact.
4. An opt-in SSH smoke target that runs the harmless workflow on a user-supplied remote host without creating credentials or assuming an SSH daemon in ordinary CI.
5. A documented manual release checklist using `doctor --keys` and fake-provider harmless flows in representative macOS, Linux, and integrated terminals, with a checked-in dated result table.

The real-terminal matrix will cover categories rather than every Cartesian product: the macOS system terminal, at least one additional macOS terminal, at least one Linux desktop terminal, at least one modern cross-platform/GPU terminal, VS Code's integrated terminal, tmux, and SSH. Each qualification exercises initial rewrite, regenerate, undo, cancellation, review acceptance, dangerous two-Enter behavior, custom-key delivery, Unicode buffer preservation, and resize/redraw.

Alternatives considered: GUI-driving every terminal in CI is brittle and unavailable on hosted runners; a purely manual matrix regresses easily; testing every terminal/shell/OS combination grows combinatorially without improving the invariant coverage.

### 6. Keep SSH execution and authentication entirely remote

When a user runs inside an SSH session, the shell adapter, `intent-sh` binary, provider CLI, provider login, current directory, and generated target command all belong to the remote host. The existing model context continues to receive only `remote: true`, not SSH marker values or local terminal identity. P0 will document that a provider installed only on the client cannot serve the remote adapter.

An SSH qualification must demonstrate buffer ownership, cancellation of the remote provider process group, no fallback after cancellation, and no automatic execution across the transport. Losing the SSH connection is treated as external session termination; P0 does not promise recovery unless the shell itself remains alive under tmux and is reattached.

Alternatives considered: forwarding requests to a local provider introduces IPC, authentication, host-identity, and credential-boundary decisions that are much larger than terminal qualification.

### 7. Sequence overlapping editor work through the shared conformance contract

This change targets the existing ZLE and native Readline backends only. It will organize terminal/editor acceptance as a reusable conformance suite so a later Fish, Nushell, or revised ble.sh backend can opt into the same invariants. It preserves the existing protocol-2 editor-backend fields and does not allocate a new adapter protocol version.

The archived `support-bash-3-via-blesh` change landed first. Its protocol-2 frame, conditional Bash 3.2 support, and fixed ble.sh behavior remain part of the baseline; this change must not restore the earlier frame or expand the ble.sh contract.

Alternatives considered: folding ble.sh into P0 violates the requested scope; independently designing two binding systems would create contradictory setup and diagnostic contracts.

## Risks / Trade-offs

- [A terminal encodes Alt as text or intercepts the chord] → `doctor --keys` reports the mismatch and guides the user to an allowed Ctrl binding or terminal-side Meta configuration without changing it automatically.
- [Raw-mode probing leaves the terminal unusable after failure] → Centralize state restoration, use deadlines and bounded reads, handle signals, and add PTY tests that inspect terminal attributes after every exit path.
- [A custom key replaces an important shell binding] → Restrict reserved controls, report static conflicts, require explicit configuration, and document how to restore defaults.
- [An older binary rejects the new strict TOML keys during rollback] → Document removing `rewrite_key` and `undo_key` before downgrading; defaults require no file migration.
- [tmux or SSH behavior varies by configuration] → Test with an isolated tmux baseline, record qualification configuration, expose the behavioral probe, and avoid claiming untested custom setups.
- [Manual terminal records become stale] → Store application/OS/shell versions and validation date, make the checklist reproducible, and require refresh for a release that changes adapter or key handling.
- [Terminal application names become an accidental runtime dependency] → Keep names only in documentation and qualification artifacts; runtime decisions use bytes and shell capabilities.
- [A terminal change regresses the current ble.sh/protocol-2 contract] → Keep protocol fields and existing ble.sh coverage unchanged while adding native ZLE/Readline qualification.

## Migration Plan

1. Complete and sync or archive `build-intent-sh-mvp` so the modified capabilities exist as main specifications.
2. Add chord parsing and configuration with unchanged defaults, then update `init`, setup, doctor, and both embedded adapters as one compatibility unit.
3. Add the raw key probe and prove terminal restoration before advertising interactive diagnostics.
4. Add PTY and tmux conformance coverage, followed by the opt-in SSH harness.
5. Run and record the representative real-terminal qualification matrix before changing documentation from MVP-only to qualified support.
6. Rebase any later editor-backend change onto the finalized binding and conformance interfaces.

Rollback uses the previous binary and a newly opened shell session. Users who configured non-default binding keys must remove those keys from the strict TOML file before running an older binary. No terminal preference, startup file, provider login, or remote host is modified automatically.

## Open Questions

- Which additional macOS terminal, Linux desktop terminal, and cross-platform/GPU terminal should form the first recorded matrix depends on machines available to the release maintainer; the required categories remain fixed even if individual applications change.
- Whether an automated loopback OpenSSH job is sufficiently stable on both hosted operating systems should be decided during the SSH harness task; the opt-in remote smoke path remains required either way.
- Runtime plugin keymap introspection may become valuable after real qualification results, but P0 limits conflict detection to validated config, static startup inspection, adapter tests, and observable key delivery.
