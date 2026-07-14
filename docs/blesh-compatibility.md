# ble.sh Compatibility Contract

`intent-sh` uses ble.sh only as an optional, user-managed line-editor backend. Production setup and initialization never download, source, update, or remove ble.sh.

## Supported versions

The tested compatibility tuple is:

- Bash 3.2.x, including Apple's `/bin/bash` 3.2.57, with ble.sh `0.4.0-nightly+d69e4d5` attached.
- Bash 4.0 or newer with ble.sh `0.4.0-nightly+d69e4d5` attached; attached ble.sh takes precedence over native Readline.
- Bash 4.0 or newer without ble.sh through native Readline.

Bash 3.0 and 3.1 are not advertised. Upstream ble.sh supports Bash 3.0+, but this project cannot keep reproducible PTY safety coverage for those obsolete releases. Bash 3.x without the exact tested ble.sh release fails closed.

The pinned upstream commit is [`d69e4d549a1881a37300fe6b4a05478bd9157dfc`](https://github.com/akinomyoga/ble.sh/commit/d69e4d549a1881a37300fe6b4a05478bd9157dfc). The compatibility harness verifies and builds GitHub's official source archive for that commit. The older stable release `0.4.0-devel3` was rejected by the stock macOS Bash 3.2 probe because its input loop stopped responding after attachment on the current macOS runner.

```text
Source archive SHA-256 db583d869ec5afef0e6bd23bd1af38ec3aa2cc3e6062f8aa499633522b005394
```

## Required editor contract

The integration probes these names before installing bindings:

- `BLE_VERSION=0.4.0-nightly+d69e4d5` and `BLE_ATTACHED=1`;
- the documented `ble-bind -x` edit-command interface, which exposes mutable `READLINE_LINE` and `READLINE_POINT` values;
- `ble-bind -f`, `blehook`, and the editor widgets used to redraw and delegate ordinary acceptance;
- `ble/function#advice`, `ble/function#advice/do`, `ble/widget/default/accept-line`, and `_ble_edit_str` for the guarded-accept wrapper.

The last group is a pinned internal interface. The ble.sh maintainer has recommended this advice pattern for intercepting all normal accept-line paths, but does not describe it as a stable public API. For that reason `intent-sh` accepts only the exact tested ble.sh version, detects an existing around-advice conflict, and fails before installing any binding when the contract differs.

The adapter registers only namespaced callbacks. It sends generated commands through the existing data-only adapter protocol, never through an evaluator. On detachment or loss of a required API it preserves the buffer, marks the backend unavailable, and requires the user to re-evaluate `intent-sh init bash` after reattaching the tested editor.

## Activation and diagnostics

Install and manage the exact commit through the [official ble.sh project](https://github.com/akinomyoga/ble.sh). Production `intent-sh` commands never fetch or execute it. In the Bash startup file, source the user's chosen path first and initialize `intent-sh` second:

```bash
source "/path/to/pinned/ble.sh"
eval "$(intent-sh init bash)"
```

The first line must leave `BLE_ATTACHED=1`. Loading with `--attach=none`, sourcing `intent-sh` first, or attaching another version fails closed. `intent-sh setup bash` performs only bounded text inspection: it can flag obvious reversed order and common `ble-bind`/accept-line conflicts, but it never sources the file. Runtime capability markers remain authoritative.

Run `intent-sh doctor` from the initialized shell. A usable session reports `PASS` for `shell.editor_backend`, `shell.blesh.version`, `shell.blesh.api`, `shell.blesh.attachment`, `shell.blesh.load_order`, and `shell.backend_keys`. These results come only from fixed, bounded marker values; buffer text, generated commands, binding bodies, and arbitrary environment values cannot become diagnostic output.

Bash 3.2 and ble.sh can process or redraw large buffers more slowly than modern Bash or Zsh. No performance condition enables a capture fallback: the integration never reads history, the clipboard, terminal output, or simulated keystrokes.

Removing the `intent-sh` activation line leaves ble.sh installed and otherwise untouched. Remove the optional config and binary independently if desired. If ble.sh was installed only for this compatibility path, remove it separately using its own project instructions; no `intent-sh` command removes third-party files.

## Protocol upgrade and rollback

This backend requires adapter protocol 2, whose requests identify `editorBackend=blesh`, carry the bounded editor version, and use UTF-8 byte cursor offsets. Reopen the shell or re-evaluate the activation line after upgrading the binary. Mixed protocol-1/protocol-2 adapter and binary versions reject the request before provider invocation or line replacement.

Rollback means installing the previous binary and re-evaluating the adapter it emits, or deleting the `intent-sh` activation line. It never changes the independently managed ble.sh checkout.

## Reproducible test setup

The test installer is deliberately outside production setup:

```bash
bash .github/scripts/install-blesh-test.sh
INTENT_SH_TEST_BLESH=/path/to/pinned/out/ble.sh go test ./internal/shelltest -run Blesh -count=1 -v
```

Set `INTENT_SH_TEST_BLESH_ARCHIVE` to an already downloaded official commit archive to run offline. The installer verifies the pinned checksum before building it with GNU awk. Ordinary `go test ./...` skips the external ble.sh matrix when `INTENT_SH_TEST_BLESH` is unset.
