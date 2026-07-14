# Terminal qualification guide

This guide validates the terminal-to-shell path without adding terminal-specific runtime code. A terminal environment is **contract-compatible** when it provides a controlling PTY, delivers the configured rewrite and undo sequences plus CR or LF and `Ctrl+C`, supports normal editor repaint/resize, and preserves the live shell for any claimed tmux reattach journey. It is **qualified** only after a maintainer completes this guide and adds a dated PASS record to [terminal-qualification-results.md](terminal-qualification-results.md).

Do not record prompts, generated commands, terminal screenshots, scrollback, history, selections, clipboard contents, environment values, host addresses, usernames, paths containing personal data, tokens, credential locations, or provider output. Record only the bounded metadata and PASS/FAIL evidence in the template below.

## Prerequisites and scope

- Use macOS or Linux on amd64 or arm64.
- Use Zsh or Bash 4.0+ for configurable-chord qualification. The separately tested Bash 3.2 ble.sh backend retains fixed `Alt+G`/`Alt+U` bindings and is not part of this native binding matrix.
- Build the candidate `intent-sh` version and activate it in a new disposable shell. Have one official provider installed and logged in on the host where the shell runs.
- Use an empty, non-sensitive working directory. Never put a credential or private text in the editable buffer.
- For a named terminal record, run the checks in that actual terminal application. Do not infer a GUI-terminal pass from the repository's pseudo-terminal tests.

The runtime must not branch on the terminal name, `TERM`, tmux, or SSH client. Those values are qualification metadata only and are not provider context.

## Automated baseline

From the repository root, run:

```sh
make fmt-check
make vet
make shell-check
go test ./... -count=1
make tmux-test                 # when tmux is installed
```

The tmux suite creates a private explicit socket in a mode-0700 temporary directory with an empty config, kills only that server, and never calls `capture-pane`. Ordinary tests skip the opt-in SSH smoke without contacting a host. On a prepared target with existing BatchMode authentication and a known host key:

```sh
INTENT_SH_TEST_SSH_TARGET=user@prepared-host make ssh-test
```

The SSH harness installs nothing and creates no key, known-host entry, provider login, or daemon. It cross-builds an ephemeral candidate and fake providers locally, stages them under a mode-0700 remote temporary directory, clears the remote test environment, exercises the allocated SSH PTY, and removes the directory. A lost connection may prevent automatic cleanup; the failure reports the bounded test phase, and the maintainer should remove only a leftover directory named `intent-sh-ssh.*` under the remote temporary directory after verifying ownership.

## Named terminal journey

Record the terminal/OS/architecture, shell version, `TERM`, configured chords, and candidate version before starting. Terminal identity is entered manually into the record; `intent-sh` does not detect or send it.

1. In a fresh supported shell, run `intent-sh setup zsh` or `intent-sh setup bash`. Confirm the effective bindings and exact removal line, and confirm the startup file was not changed.
2. Activate the printed line in that disposable shell and run ordinary `intent-sh doctor`.
3. Run `intent-sh doctor --keys`. Press rewrite, undo, Enter, and `Ctrl+C` only when prompted. Record the six stable `terminal.keys.*` IDs as PASS/FAIL, not the received bytes.
4. Type a harmless intent such as `print the current working directory` without Enter. Press rewrite. Confirm the line changes but nothing executes. Press rewrite again and confirm regeneration still uses the original intent. Press undo and confirm exact restoration.
5. Start another harmless rewrite and press `Ctrl+C` while generation is active. Confirm cancellation preserves the original line and does not start a fallback. If the provider returns too quickly, use the deterministic fake-provider PTY/SSH suite as cancellation evidence and mark the named-terminal cancellation observation NOT RUN rather than guessing.
6. Put non-ASCII text in a harmless intent, move the cursor away from the end, and cause a local failure or cancellation. Confirm the complete text and a valid cursor position return.
7. Start a harmless generation and resize the terminal while it is active. Confirm status repaint, one complete result at most, and no partial buffer replacement.
8. For review acceptance, use a disposable directory and request creation of one marker file inside it. Confirm generation alone creates nothing; one deliberate Enter may execute the visible review command.
9. For dangerous confirmation, create a new empty disposable directory and request deletion of that exact directory with recursive `rm`. Review the generated path carefully. Confirm generation creates no effect and the first Enter only warns. If and only if the visible command targets that disposable directory exactly, a second unchanged Enter may be used to verify native acceptance. Never use a real data path.
10. Repeat the key probe and rewrite/undo checks with one allowed custom pair such as `ctrl+x` and `ctrl+r`, then restore `alt+g` and `alt+u`. Each change requires a new shell. Inside tmux, do not choose its current prefix (commonly `Ctrl+B`) unless you first change that tmux binding yourself.

A failure is evidence, not permission for the tool to edit terminal settings. Record FAIL, remediate manually, and rerun the entire affected journey.

## tmux journey and troubleshooting

Run the named terminal journey first outside tmux, then inside a fresh user-created tmux session. Record the tmux version and inner `TERM`.

1. Run `intent-sh doctor --keys` inside tmux. If a key fails only there, inspect `tmux list-keys -T root` and the relevant prefix table yourself. A root binding may consume a Meta or Ctrl chord before `/dev/tty` receives it.
2. Choose one manual remedy: remove/change the conflicting tmux binding in your own config, or run `intent-sh config set rewrite_key <allowed-chord>` (or `undo_key`) and start a new shell. `intent-sh` never applies either remedy.
3. Generate a harmless command, detach without accepting it, reattach to the same live pane, and confirm the visible buffer plus rewrite/undo state remain. Undo must still restore the original.
4. Generate a dangerous command against a disposable directory, press Enter once to arm it, detach, reattach, and confirm only the second unchanged Enter can accept it.
5. Open another pane and another session. Confirm neither inherits the first shell's original buffer, rewrite index, undo state, or armed confirmation.
6. Resize during a slow harmless generation and confirm repaint without partial replacement.

Do not use `capture-pane`, screenshots, scrollback export, shell history, or clipboard capture as evidence. A bounded PASS/FAIL row is sufficient.

## SSH and SSH-to-tmux journey

The remote host must independently have a supported shell, the candidate `intent-sh` binary, and an official provider CLI with its own remote login. A client-only provider or login is intentionally unavailable to the remote adapter. Do not copy or forward provider credential files for qualification.

1. Connect normally to the prepared remote host, activate the remote adapter, and run remote `intent-sh doctor` and `intent-sh doctor --keys`.
2. Repeat rewrite, regenerate, undo, cancellation, resize, review, dangerous confirmation, custom-key, and no-auto-execution checks. The provider process and any marker directory must be remote.
3. Disconnect a plain SSH session only after clearing test input. Treat that shell as terminated unless an external session manager keeps it alive; `intent-sh` promises no plain-SSH reconnection state.
4. For the reattach case, start tmux on the remote host, generate a harmless command in a pane, detach tmux, end SSH, reconnect, and reattach to the same remote pane. Confirm the visible buffer and undo state survive. Repeat the first-danger-Enter reattach check only with a disposable remote directory.
5. Confirm a new remote pane/session has independent state and that no client-local `intent-sh` process or provider is required.

Record only a target label such as `prepared Linux VM`; do not record hostname, address, username, SSH marker values, key paths, provider account identifiers, prompts, or terminal contents.

## Downgrade, reset, and removal journey

Reset native bindings and open a new shell:

```sh
intent-sh config set rewrite_key alt+g
intent-sh config set undo_key alt+u
```

Before downgrading to a binary whose strict schema predates binding keys, remove the `rewrite_key` and `undo_key` lines from the secret-free TOML file. To remove the integration, delete the exact activation line reported by `intent-sh setup`, open a new shell, remove the binary, and optionally remove the config. No terminal or tmux setting should need rollback because `intent-sh` never modified one.

## Dated result template

Copy this block into [terminal-qualification-results.md](terminal-qualification-results.md). Use `PASS`, `FAIL`, `NOT RUN`, or `SKIP`; never leave an ambiguous blank.

```text
Date (YYYY-MM-DD):
Maintainer:
Category: macOS system | macOS additional | Linux desktop | cross-platform/GPU | integrated | tmux | SSH
Terminal application/version:
OS/version:
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

A category may be described as qualified only when its representative row is Overall PASS. When key handling, Enter guarding, cancellation, adapter repaint, or configuration grammar changes, refresh affected rows or label them as evidence for the older version.
