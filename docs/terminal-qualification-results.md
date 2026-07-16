# macOS terminal qualification results

This record distinguishes deterministic automated evidence from named macOS terminal qualification. A category is qualified only when a dated row is `PASS` after the full [terminal qualification guide](terminal-qualification.md). `FAIL`, `NOT RUN`, and `SKIP` are evidence states, not support claims.

## Current named-environment matrix

Record reviewed: 2026-07-16

| Required category | Representative environment | Status | Bounded evidence |
| --- | --- | --- | --- |
| macOS system terminal | Terminal.app 2.15 | NOT RUN | The available UI-control capability stopped at its safety boundary. No setting was changed and no pass is inferred. |
| Additional macOS terminal | iTerm2 3.6.11 | NOT RUN | The available UI-control capability stopped at its safety boundary. No setting was changed and no pass is inferred. |
| macOS integrated terminal | Visual Studio Code 1.128.0 integrated terminal | FAIL | Ordinary doctor and an allowed Ctrl-chord journey passed, but the default Option/Alt chords were transformed by the current terminal configuration. Settings were left untouched. |
| macOS tmux | Private empty-config automated server | NOT RUN | Deterministic tmux behavior passed on 2026-07-14; the complete named-terminal tmux journey was not run. |
| macOS SSH | Prepared macOS target | NOT RUN | Required CI contacts no target, and no dated protected macOS-to-macOS journey has been recorded. |

No named terminal category is currently described as qualified.

## Automated evidence that is not named-terminal qualification

Candidate for these historical rows: working tree based on commit `c0bdae8`.

| Date | Host | Layer | Shell / TERM | Chords | Result |
| --- | --- | --- | --- | --- | --- |
| 2026-07-14 | macOS 26.5.2 arm64 | direct pseudo-terminal | Zsh 5.9 and Bash 5.3.15 / `dumb`, `xterm-256color`, `screen-256color` | defaults and allowed Ctrl alternatives | PASS: deterministic fake-provider lifecycle, CR/LF, cancellation, Unicode cursor, resize/repaint, no-auto-execution, and independent sessions. |
| 2026-07-14 | macOS 26.5.2 arm64 | disposable native journey | Zsh 5.9 / `xterm-256color` | defaults; `ctrl+x` + `ctrl+r`; defaults restored | PASS: setup, ordinary and interactive doctor, custom config, rewrite/undo, reset, downgrade preparation, exact activation removal, binary/config removal, and no startup mutation. |
| 2026-07-14 | macOS 26.5.2 arm64 | tmux 3.7b, private socket and empty config | Zsh 5.9 and Bash 5.3.15 | default and custom matrix | PASS: lifecycle, CR/LF, cancellation, resize/repaint, dangerous two-Enter, detach/reattach, session isolation, and intercepted-root-binding diagnostics. |
| 2026-07-14 | macOS 26.5.2 arm64 | SSH target absent | target not configured | defaults | PASS: clean opt-in behavior without target contact, credential lookup, remote directory, or provider creation. |

Automated pseudo-terminal evidence proves adapter invariants after a byte path exists; it does not identify or qualify a terminal application. The current required graph adds macOS-only manifests, verified UTF-8 locale injection, native-versus-inspected Darwin artifact evidence, and executable coverage. Those jobs become evidence only after their hosted run succeeds; configuration alone does not create a PASS.

## Recorded named-environment result

### Visual Studio Code integrated terminal on macOS

```text
Date (YYYY-MM-DD): 2026-07-14
Maintainer: Codex qualification run
Category: macOS integrated
Terminal application/version: Visual Studio Code 1.128.0 integrated terminal
macOS version: 26.5.2
Architecture: arm64
Shell/version: zsh 5.9
Layer: direct
TERM: xterm-256color
rewrite_key / undo_key: alt+g / alt+u; ctrl+x / ctrl+r remediation; defaults restored
intent-sh version or commit: c0bdae8 plus the then-current change

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
Bounded note (no prompt, command, path, address, credential, or terminal content): The default Option/Alt chords were transformed by the current integrated-terminal configuration. The allowed Ctrl remediation passed the probe and lifecycle, defaults were restored, and terminal settings were not edited.
```

## Disposable journey and state audit

- PASS: the macOS disposable native journey covered default keys, allowed custom keys, the bounded key probe, defaults restoration, downgrade cleanup, exact activation removal, binary removal, and config removal.
- PASS: private tmux sockets and empty configs covered Bash and Zsh flows; the harness removed its socket state and did not inspect or modify the user's tmux server.
- PASS: the SSH harness's no-target, bounded-target, cleanup-path, forwarding, marker-privacy, and Darwin identity checks execute without contacting a host.
- NOT RUN: protected end-to-end macOS SSH and SSH-to-tmux qualification has no dated evidence.
- PASS: GUI qualification used a disposable home and deterministic fake providers. Setup changed no startup file, and bounded post-run checks found no created activation file.
- PASS: defaults were restored after the custom integrated-terminal journey; application settings were not edited.

## OpenSpec requirement coverage

The active `macos-terminal-compatibility` capability maps every current requirement to deterministic or explicit manual evidence:

| Requirement | Deterministic coverage | Explicit manual coverage |
| --- | --- | --- |
| macOS terminal-independent PTY contract | `TestNativeTerminalConformanceLifecycleMatrix`; provider/frame/context privacy regressions | Named terminal steps 1–5 |
| Editor state and explicit locale | `TestTERMResizeAndUnicodeFailureConformance`; `TestNativeEditorsUnicodeCursorRoundTripInPTY` under an inherited `C` locale | Named terminal steps 6–9 |
| macOS tmux sessions | tmux lifecycle, detach/reattach, isolation, and intercepted-binding tests | tmux journey |
| macOS remote locality | no-target, harness-safety, marker-privacy, and Darwin remote-boundary tests; protected conformance remains opt-in | macOS SSH journey |
| Bounded key delivery | fake-session and real-PTY keyprobe suites, including restoration, signals, EOF, byte wiping, and privacy | `doctor --keys` in every named journey |
| Reproducible evidence | privacy-safe manifest/evidence tests and this bounded template | current matrix plus dated result template |

Manual rows do not imply a PASS. They identify the evidence path for named GUI terminals and caller-owned macOS SSH targets that required CI cannot drive safely.
