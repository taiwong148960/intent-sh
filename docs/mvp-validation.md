# MVP validation record

Validation date: 2026-07-14

## Local release gate

The complete local gate passed on macOS arm64 with Go 1.24.6, Zsh 5.9, stock GNU Bash 3.2.57, GNU Bash 5.2.37, and the pinned ble.sh compatibility build:

```sh
make fmt-check
make vet
make shell-check

INTENT_SH_TEST_BASH=/path/to/bash-5.2.37 \
INTENT_SH_TEST_BASH32=/bin/bash \
INTENT_SH_TEST_BLESH=/path/to/pinned/ble.sh \
go test ./... -count=1

openspec validate support-bash-3-via-blesh --strict
```

An ordinary `go test ./...` run also passes without downloading or building ble.sh; the external matrix skips unless `INTENT_SH_TEST_BLESH` is explicitly set.

The PTY matrix covers fake Claude and fake Codex through ZLE, native Readline, and ble.sh. It verifies initial rewrite, mixed input, regeneration from the preserved original, undo, clarification, malformed and stale responses, timeout, Ctrl+C process-tree cancellation, Unicode cursor conversion, manual-edit invalidation, no automatic execution, review acceptance, and dangerous two-Enter confirmation. Bash coverage includes ble.sh on stock Bash 3.2 and both native Readline and ble.sh in modern Bash, in Emacs and Vi insert modes.

## Bash 3.2 compatibility journey

The pinned test installer verifies the official commit archive before building:

```text
ble.sh commit  d69e4d549a1881a37300fe6b4a05478bd9157dfc
BLE_VERSION    0.4.0-nightly+d69e4d5
SHA-256        db583d869ec5afef0e6bd23bd1af38ec3aa2cc3e6062f8aa499633522b005394
```

`TestBleshSourceSetupDoctorAndRemovalJourneyInPTY` follows the documented workflow in disposable homes without changing the real user environment:

1. Build the `intent-sh` binary from source and run `intent-sh setup bash`.
2. Verify setup explains the pinned dependency and load order and does not create a startup file.
3. Attach the pinned ble.sh build in `/bin/bash` 3.2.57, then initialize adapter protocol 2 with backend `blesh`.
4. Run `intent-sh doctor` and verify the backend, version, API, attachment, load-order, provider-login, and overall-readiness checks pass.
5. Perform a harmless rewrite, prove the generated target is still only editable text, and restore the exact original with undo.
6. Cancel a slow provider with Ctrl+C, verify its process tree stops, and preserve the pre-request buffer and cursor.
7. Remove the `intent-sh` activation, binary, and optional config while leaving the independently managed ble.sh activation intact.

Negative PTY cases separately prove that missing, detached, wrongly ordered, API-incomplete, conflicted, or version-mismatched ble.sh installs leave Bash 3.2 inert and do not invoke a provider.

## Protocol and build targets

The embedded Zsh and Bash adapters and the binary use protocol 2. Requests identify the active editor backend and version and represent the cursor as a UTF-8 byte offset. Protocol-1/protocol-2 mixtures fail before provider invocation or buffer replacement.

The source builds with `CGO_ENABLED=0` for:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

The CI workflow repeats the ordinary suite and cross-builds on hosted Ubuntu and macOS. Separate pinned-ble.sh jobs verify the archive checksum, cache only the test artifact, exercise stock macOS Bash 3.2, and run modern Bash in both native Readline and ble.sh modes. Real-provider compatibility is recorded in [provider-compatibility.md](provider-compatibility.md), the editor compatibility boundary in [blesh-compatibility.md](blesh-compatibility.md), and the security pass in [threat-review.md](threat-review.md).
