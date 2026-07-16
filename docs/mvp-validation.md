# MVP validation record

Validation date: 2026-07-16

## Local release gate

The local gate qualifies Zsh and Bash 4.0 or newer through their native editors:

```sh
make fmt-check
make vet
make shell-check
go test ./... -count=1
make openspec-check
```

The PTY matrix covers fake Claude and fake Codex through ZLE and native Readline. It verifies initial rewrite, mixed input, regeneration from the preserved original, undo, clarification, malformed and stale responses, timeout, Ctrl+C process-tree cancellation, Unicode cursor handling, manual-edit invalidation, no automatic execution, review acceptance, and dangerous two-Enter confirmation. Bash coverage runs in Emacs and Vi insert modes and rejects any Bash generation older than 4.0 before installing bindings.

## Protocol and build targets

The embedded Zsh and Bash adapters and the binary use protocol 2. Requests identify the native editor backend and version and represent the cursor as a UTF-8 byte offset. Protocol-1/protocol-2 mixtures, Bash versions below 4.0, and non-native Bash backends fail before provider invocation or buffer replacement.

The source builds with `CGO_ENABLED=0` for:

- `darwin/amd64`
- `darwin/arm64`

Required CI runs entirely on the explicit macOS runner. Native Bash/Zsh PTY, private-socket tmux, artifact, race, and executable-coverage jobs retain strict manifest and cleanup boundaries. Both Darwin artifacts are reproducibly built and inspected; only the runner-native artifact receives executable qualification. Required SSH checks contact no target, while prepared macOS SSH and authenticated provider compatibility remain protected manual paths. Provider evidence is recorded in [provider-compatibility.md](provider-compatibility.md), and the security pass is recorded in [threat-review.md](threat-review.md).
