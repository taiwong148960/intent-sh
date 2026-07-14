# Terminal qualification results

This record distinguishes deterministic automated evidence from named terminal qualification. A category is qualified only when a dated row is `PASS` after the full [terminal qualification guide](terminal-qualification.md). `FAIL`, `NOT RUN`, and `SKIP` are evidence states, not support claims.

## Initial matrix status

Record completed: 2026-07-14

| Required category | Representative environment | Status | Bounded evidence |
| --- | --- | --- | --- |
| macOS system terminal | Terminal.app 2.15 | NOT RUN | The available UI-control capability refused Terminal.app at its safety boundary. No terminal setting was changed and no pass is inferred. |
| Additional macOS terminal | iTerm2 3.6.11 | NOT RUN | The available UI-control capability refused iTerm2 at its safety boundary. No terminal setting was changed and no pass is inferred. |
| Linux desktop terminal | xterm 407 on Ubuntu 26.04 arm64 | FAIL | Ordinary doctor passed. The bounded key probe failed because the Screen Sharing control path did not deliver modifier keys; xterm settings were left untouched. |
| Modern cross-platform or GPU terminal | Warp 0.2026.07.08.17.54.02 | NOT RUN | The available UI-control capability refused Warp at its safety boundary. No terminal setting was changed and no pass is inferred. |
| Integrated terminal | VS Code 1.128.0 integrated terminal on macOS | FAIL | Ordinary doctor and the custom Ctrl-chord journey passed, but the default Option/Alt chords were transformed by the current terminal configuration. VS Code settings were left untouched. |
| tmux | Private empty-config servers on macOS and Linux | NOT RUN | The deterministic tmux baseline passed on both hosts, including detach/reattach and intercepted-binding diagnostics; the full named-terminal tmux journey was not run. |
| SSH | Prepared Ubuntu VM | NOT RUN | Both absent-target and configured-target automated harnesses passed; the full named SSH and SSH-to-tmux journey was not run. |

The initial matrix is fully recorded, but no named terminal category is currently described as qualified.

## Automated evidence that is not a named-terminal qualification

Candidate for every row: working tree based on commit `c0bdae8`.

| Date | Host | Layer | Shell / TERM | Chords | Result |
| --- | --- | --- | --- | --- | --- |
| 2026-07-14 | macOS 26.5.2 arm64 | direct pseudo-terminal | Zsh 5.9 and Bash 5.3.15 / `dumb`, `xterm-256color`, `screen-256color` | `alt+g` + `alt+u`; allowed Ctrl alternatives | PASS: deterministic fake-provider lifecycle, CR/LF, cancellation, Unicode cursor, resize/repaint, no-auto-execution, and independent sessions. |
| 2026-07-14 | macOS 26.5.2 arm64 | disposable native journey | Zsh 5.9 / `xterm-256color` PTY | defaults; `ctrl+x` + `ctrl+r`; defaults restored | PASS: setup, ordinary and interactive doctor, custom config, rewrite/undo, reset, pre-downgrade key removal, exact activation removal, binary/config removal, and no startup mutation. |
| 2026-07-14 | macOS 26.5.2 arm64 | tmux 3.7b, private socket and empty config | Zsh 5.9 and Bash 5.3.15 | default and custom matrix | PASS: lifecycle, CR/LF, cancellation, resize/repaint, dangerous two-Enter, detach/reattach, session isolation, and intercepted-root-binding diagnostic. |
| 2026-07-14 | Ubuntu 26.04 arm64 | direct pseudo-terminal | Zsh 5.9 and Bash 5.3.9 / supported TERM matrix | default and custom matrix | PASS: full vet, unit, fake-provider integration, PTY, fuzz-seed, syntax, and supported Linux amd64/arm64 build checks. |
| 2026-07-14 | Ubuntu 26.04 arm64 | disposable native journey | Zsh 5.9 / `xterm-256color` PTY | defaults; `ctrl+x` + `ctrl+r`; defaults restored | PASS: setup, key probe, reset, downgrade, removal, and startup-state assertions. |
| 2026-07-14 | Ubuntu 26.04 arm64 | tmux 3.6, private socket and empty config | Zsh 5.9 and Bash 5.3.9 | default and custom matrix | PASS: lifecycle, detach/reattach, session isolation, and intercepted-root-binding diagnostic. |
| 2026-07-14 | macOS client | SSH target absent | target not configured | defaults | PASS: clean skip without target contact, credential lookup, remote directory, or provider creation. |
| 2026-07-14 | macOS client to prepared Ubuntu VM | SSH with allocated remote PTY | remote Zsh 5.9 and Bash 5.3.9 | defaults | PASS: rewrite, regenerate, undo, cancellation of the remote provider tree, privacy boundaries, review no-auto-execution, dangerous two-Enter, and remote temporary-directory cleanup. |

Automated pseudo-terminal evidence proves adapter invariants after a byte path exists; it does not identify or qualify a terminal application.

## Recorded named-environment results

### VS Code integrated terminal

```text
Date (YYYY-MM-DD): 2026-07-14
Maintainer: Codex qualification run
Category: integrated
Terminal application/version: Visual Studio Code 1.128.0 integrated terminal
OS/version: macOS 26.5.2
Architecture: arm64
Shell/version: zsh 5.9
Layer: direct
TERM: xterm-256color
rewrite_key / undo_key: alt+g / alt+u; ctrl+x / ctrl+r remediation; defaults restored
intent-sh version or commit: c0bdae8 plus the uncommitted change

doctor ordinary: PASS
terminal.keys.tty: PASS
terminal.keys.rewrite: FAIL
terminal.keys.undo: FAIL
terminal.keys.enter: PASS
terminal.keys.cancel: PASS
terminal.keys.restore: PASS
rewrite / regenerate / undo: PASS
cancellation / no fallback: PASS
Unicode buffer / cursor: NOT RUN
resize / repaint: PASS
review no-auto-exec / acceptance: PASS
danger no-auto-exec / two-Enter: PASS
custom chords / defaults restored: PASS
detach / reattach / state isolation: SKIP
privacy review (bounded metadata only): PASS

Overall: FAIL
Bounded note (no prompt, command, path, address, credential, or terminal content): The default Option/Alt chords were transformed by the current integrated-terminal configuration. The allowed Ctrl remediation passed the complete probe and lifecycle, defaults were restored, and terminal settings were not edited.
```

### Linux xterm through Screen Sharing

```text
Date (YYYY-MM-DD): 2026-07-14
Maintainer: Codex qualification run
Category: Linux desktop
Terminal application/version: xterm 407
OS/version: Ubuntu 26.04
Architecture: arm64
Shell/version: zsh 5.9
Layer: direct
TERM: xterm
rewrite_key / undo_key: alt+g / alt+u
intent-sh version or commit: c0bdae8 plus the uncommitted change

doctor ordinary: PASS
terminal.keys.tty: PASS
terminal.keys.rewrite: FAIL
terminal.keys.undo: FAIL
terminal.keys.enter: FAIL
terminal.keys.cancel: FAIL
terminal.keys.restore: PASS
rewrite / regenerate / undo: NOT RUN
cancellation / no fallback: NOT RUN
Unicode buffer / cursor: NOT RUN
resize / repaint: NOT RUN
review no-auto-exec / acceptance: NOT RUN
danger no-auto-exec / two-Enter: NOT RUN
custom chords / defaults restored: NOT RUN
detach / reattach / state isolation: SKIP
privacy review (bounded metadata only): PASS

Overall: FAIL
Bounded note (no prompt, command, path, address, credential, or terminal content): The Screen Sharing qualification-control path did not forward modifier keys reliably. This result does not isolate xterm from the viewer path; no terminal setting was edited and no pass is inferred.
```

## Disposable journey and state audit

- PASS: macOS and Linux disposable native journeys covered default keys, allowed custom keys, the bounded key probe, defaults restoration, downgrade cleanup, exact activation removal, binary removal, and config removal.
- PASS: private tmux sockets and empty configs covered default/custom Bash and Zsh flows; the harness removed its sockets and did not inspect or modify the user's tmux server.
- PASS: the SSH harness was verified both absent and configured, used only a prepared disposable VM, created no credentials or provider login, and removed its remote temporary state.
- PASS: GUI qualification used disposable homes and deterministic fake providers. Setup changed no startup file; bounded post-run checks found no qualification-created `.zshrc` or `.bashrc`.
- PASS: intent-sh defaults were restored after the custom VS Code journey; terminal application settings were not edited.
- PASS: the disposable VM and all host-side qualification temporary directories were removed after evidence capture. Deliberately installed local qualification dependencies remain available for reruns.

## OpenSpec requirement coverage

Strict validation passed for `qualify-unix-terminal-environments` on 2026-07-14. Each added or modified requirement has a deterministic test boundary or an explicit manual qualification step:

| Requirement | Deterministic coverage | Explicit manual coverage |
| --- | --- | --- |
| Terminal-independent PTY contract | `TestNativeTerminalConformanceLifecycleMatrix`; provider/frame/context privacy regressions | Named terminal journey, steps 1–5 |
| Preserve editor state across terminal behavior | `TestTERMResizeAndUnicodeFailureConformance`; CR/LF and danger cases in the lifecycle matrix | Named terminal journey, steps 6–9 |
| Qualified tmux sessions | `TestTmuxLifecycleMatrix`; `TestTmuxDetachReattachAndSessionStateIsolation`; `TestTmuxInterceptedRootBindingFailsKeyDeliveryDiagnostic` | tmux journey |
| Remote SSH locality | Opt-in `TestSSHRemoteBashAndZshConformance`; SSH marker/provider-boundary regressions | SSH and SSH-to-tmux journey |
| Bounded key-delivery probe | Fake-session and real-PTY keyprobe suites, including restoration, signals, EOF, byte wiping, and privacy | `doctor --keys` in every named journey |
| Reproducible qualification evidence | Template validation is documentation-owned; no runtime consumes terminal identity | Initial matrix plus dated result template |
| Explicit reversible activation | CLI init/setup tests and configured adapter re-evaluation tests | Setup, reset, downgrade, and removal journeys |
| Strict secret-free configuration | Key-chord table/fuzz tests; config canonicalization, reserved/equal-key, and atomic rollback tests | Custom-key/default-reset and downgrade steps |
| Readiness diagnostics | Doctor/CLI tests for ordinary and interactive modes; setup conflict fixtures | Ordinary doctor and key probe steps |
| Privacy/source/removal documentation | Link and content review plus terminal-safe CLI tests | Every bounded metadata/privacy checklist |
| Supported shell adapters and rewrite workflow | Embedded rendering tests plus direct PTY, tmux, and opt-in SSH conformance matrices | Harmless rewrite/regenerate/undo/cancel/acceptance journeys |

The manual rows do not imply a PASS. They identify the required evidence path for environments that cannot be represented deterministically in the current process, especially named GUI terminals and caller-owned SSH targets.
