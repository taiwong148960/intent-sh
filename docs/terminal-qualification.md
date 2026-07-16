# macOS terminal qualification guide

This guide validates the macOS terminal-to-shell path without adding terminal-specific runtime code. An environment is **contract-compatible** when it provides a controlling PTY, delivers the configured rewrite and undo sequences plus CR or LF and `Ctrl+C`, supports normal editor repaint/resize, and preserves the live shell for any claimed tmux reattach journey. It is **qualified** only after a maintainer completes this guide and adds a dated PASS record to [terminal-qualification-results.md](terminal-qualification-results.md).

Record no prompts, generated commands, screenshots, scrollback, history, selections, clipboard contents, environment values, host addresses, usernames, personal paths, tokens, credential locations, or provider output. Keep only the bounded metadata and PASS/FAIL evidence in the template below.

## Prerequisites and scope

- Use macOS on Apple silicon or Intel.
- Use Zsh or Bash 4.0+ with its native editor.
- Build the candidate and activate it in a new disposable shell. Have one official provider installed and logged in on the Mac where the shell runs.
- Use an empty, non-sensitive working directory. Never put a credential or private text in the editable buffer.
- Run named-terminal checks in that actual macOS application. Do not infer an application pass from repository pseudo-terminal tests.

Runtime behavior must not branch on terminal name, `TERM`, tmux, or SSH client. These are qualification metadata only and are not provider context.

## Automated baseline

From the repository root, run:

```sh
make static-check
make test-unit QUALIFICATION_DIR=/absolute/disposable/results
make native-pty-test QUALIFICATION_DIR=/absolute/disposable/results
make tmux-test QUALIFICATION_DIR=/absolute/disposable/results
```

The required macOS PTY suite covers Bash and Zsh, Emacs and Vi modes, default/custom chords, CR/LF, representative `TERM` values, explicit `C` and verified UTF-8 locales, mixed English/Chinese and combining text, cursor positions, resize, provider failures, cancellation delivery modes, terminal closure, exact buffer restoration, regeneration, undo, safety acceptance, setup, downgrade, and removal. The tmux suite adds detach/reattach and pane/session isolation using a private socket, mode-0700 directory, empty config, and no pane capture.

Required CI contacts no SSH target. It verifies absent-target behavior, target and cleanup-path bounds, disabled forwarding, marker privacy, and Darwin remote-identity parsing locally. End-to-end remote evidence is protected/manual.

For a prepared macOS target with existing BatchMode authentication and a known host key:

```sh
INTENT_SH_TEST_SSH_TARGET=user@prepared-mac make external-ssh-test
```

The harness first requires Darwin on arm64 or amd64. It installs nothing and creates no key, known-host entry, provider login, or daemon. It stages the candidate and fake providers under one mode-0700 remote temporary directory, supplies a verified UTF-8 locale, clears the remote test environment, exercises the allocated PTY, and removes that directory. A lost connection may prevent cleanup; after verifying ownership, remove only the reported `intent-sh-ssh.*` directory. The protected workflow accepts one bounded host or `user@host` token, accepts no identity/port/option string, and uploads no SSH artifact.

Pseudo-terminal success proves behavior after bytes reach a PTY. It does not identify or qualify a named application. Complete the following journey in the actual application when key delivery itself is the claim.

## Named macOS terminal journey

Record the terminal application/version, macOS version/architecture, shell version, `TERM`, configured chords, and candidate version before starting.

1. In a fresh supported shell, run `intent-sh setup zsh` or `intent-sh setup bash`. Confirm the effective bindings, selected startup file, exact removal line, and that the file was not changed.
2. Activate the printed line in that disposable shell and run ordinary `intent-sh doctor`.
3. Run `intent-sh doctor --keys`. Press rewrite, undo, Enter, and `Ctrl+C` only when prompted. Record stable `terminal.keys.*` IDs, never received bytes.
4. Type a harmless intent without Enter. Press rewrite and confirm only the editable line changes. Press rewrite again to confirm regeneration uses the original; press undo to confirm exact restoration.
5. Start a harmless rewrite and press `Ctrl+C` while generation is active. Confirm the original line returns and fallback does not start. If the provider is too fast, mark the named observation NOT RUN and retain the deterministic test as separate evidence.
6. Put non-ASCII text in a harmless intent, move the cursor away from the end, and cause a local failure or cancellation. Confirm complete text and a valid cursor return.
7. Resize during a slow harmless generation. Confirm status repaint, at most one complete result, and no partial replacement.
8. In a disposable directory, request creation of one marker. Confirm generation creates nothing; one deliberate Enter may execute the visible review command.
9. In a new empty disposable directory, request deletion of exactly that directory. Confirm generation has no effect and the first Enter only warns. Use the second unchanged Enter only after verifying the visible path exactly.
10. Repeat the key probe and harmless lifecycle with one allowed custom pair, then restore `alt+g` and `alt+u` in a new shell.

A failure is evidence, not permission to change terminal settings automatically. Record FAIL, remediate manually, and rerun the affected journey.

## tmux journey

Run the named journey first outside tmux, then inside a fresh user-created macOS tmux session. Record the tmux version and inner `TERM`.

1. Run `intent-sh doctor --keys`. If a chord fails only there, inspect `tmux list-keys -T root` and the relevant prefix table yourself.
2. Either change the conflicting tmux binding in your own config or select another allowed `intent-sh` chord and start a new shell. `intent-sh` applies neither remedy.
3. Generate a harmless command, detach without accepting it, reattach to the same live pane, and confirm the buffer and undo state survive.
4. Arm a dangerous disposable-directory command with the first Enter, detach/reattach, and confirm only the second unchanged Enter can accept it.
5. Open another pane and session; confirm neither inherits the first shell's rewrite or confirmation state.
6. Resize during a slow harmless generation and confirm repaint without partial replacement.

Do not use pane capture, screenshots, scrollback export, history, or clipboard capture as evidence.

## macOS SSH and SSH-to-tmux journey

The remote Mac must independently have a supported shell, the candidate binary, and an official provider CLI with its own remote login. A client-only provider or login is intentionally unavailable. Do not copy or forward credential files.

1. Connect normally, activate the remote adapter, and run remote `intent-sh doctor` and `intent-sh doctor --keys`.
2. Repeat rewrite, regenerate, undo, cancellation, resize, review, dangerous confirmation, custom-key, and no-auto-execution checks. The provider process and marker directory must be remote.
3. Treat a plain disconnected shell as terminated unless a separate session manager keeps it alive.
4. For reattach evidence, run tmux remotely, generate harmless input, detach, end SSH, reconnect, and reattach to that pane. Confirm buffer and undo state survive. Use only a disposable remote directory for the armed-danger reattach check.
5. Confirm a new remote pane/session has independent state and no client-local `intent-sh` process or provider is required.

Record a non-identifying target label such as `prepared macOS test host`; never record its address, username, marker values, key paths, provider account, prompts, or terminal contents.

## Reset, downgrade, and removal

Restore defaults and open a new shell:

```sh
intent-sh config set rewrite_key alt+g
intent-sh config set undo_key alt+u
```

Before downgrading to a strict schema that predates binding keys, remove the `rewrite_key` and `undo_key` lines from the secret-free TOML file. To remove the integration, delete the exact activation line reported by setup, open a new shell, remove the binary, and optionally remove the config. There is no terminal or tmux setting to roll back because `intent-sh` never modified one.

## Dated result template

Use `PASS`, `FAIL`, `NOT RUN`, or `SKIP`; never leave an ambiguous blank.

```text
Date (YYYY-MM-DD):
Maintainer:
Category: macOS system | macOS additional | macOS integrated | macOS tmux | macOS SSH
Terminal application/version:
macOS version:
Architecture: amd64 | arm64
Shell/version: zsh ... | bash ...
Layer: direct | tmux <version> | SSH target label | SSH + tmux <version>
TERM:
rewrite_key / undo_key:
intent-sh version or commit:

doctor ordinary: PASS|FAIL
terminal.keys.tty: PASS|FAIL
terminal.keys.rewrite: PASS|FAIL
terminal.keys.undo: PASS|FAIL
terminal.keys.enter: PASS|FAIL
terminal.keys.cancel: PASS|FAIL
terminal.keys.restore: PASS|FAIL
rewrite / regenerate / undo: PASS|FAIL|NOT RUN
cancellation / no fallback: PASS|FAIL|NOT RUN
Unicode buffer / cursor: PASS|FAIL|NOT RUN
resize / repaint: PASS|FAIL|NOT RUN
review no-auto-exec / acceptance: PASS|FAIL|NOT RUN
danger no-auto-exec / two-Enter: PASS|FAIL|NOT RUN
custom chords / defaults restored: PASS|FAIL|NOT RUN
detach / reattach / state isolation: PASS|FAIL|SKIP|NOT RUN
privacy review (bounded metadata only): PASS|FAIL

Overall: PASS|FAIL|NOT RUN
Bounded note (no prompt, command, path, address, credential, or terminal content):
```

A category is qualified only when its representative row is Overall PASS. Refresh affected rows whenever key parsing, adapter registration, Enter guarding, cancellation, or repaint behavior changes.
