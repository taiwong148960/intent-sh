# MVP validation record

Validation date: 2026-07-13

## Local release gate

The complete local gate passed on macOS arm64 with Go 1.24.6, Zsh 5.9, and GNU Bash 5.3:

```sh
make check
```

This ran formatting verification, `go vet ./...`, every unit/integration/schema package, the fake-Claude and fake-Codex Bash/Zsh PTY workflows, opt-in smoke-test skip behavior, and Bash/Zsh syntax checks. The PTY workflow includes initial rewrite, regeneration from the preserved original, undo, clarification, cancellation, fallback, privacy assertions, no automatic execution, review acceptance, and dangerous two-Enter confirmation.

OpenSpec strict validation also passed:

```sh
openspec validate build-intent-sh-mvp --strict
```

## Source-install journey

A disposable home was used to follow the README without altering a real startup or config file:

1. Build the native Darwin arm64 binary with `go build -trimpath`.
2. Install it as `.local/bin/intent-sh` with mode `0755`.
3. Verify `intent-sh version` and adapter protocol 1.
4. Run `intent-sh setup zsh`; verify the activation/removal guidance and that `.zshrc` remains absent.
5. Load `eval "$(intent-sh init zsh)"` in `zsh -f`; verify the adapter and protocol session variables.
6. Run `intent-sh doctor` using the existing official Codex login; verify adapter, Codex login, and overall readiness checks pass without creating config.
7. Exercise harmless rewrite/regenerate/undo behavior through the disposable-home PTY suite.
8. Remove only the installed binary and verify it is absent. No provider login, account, startup file, or credential storage is changed.

## Build targets

The same source built successfully with `CGO_ENABLED=0` for:

- `darwin/arm64` (native journey binary)
- `linux/amd64`
- `linux/arm64`

The repository CI repeats the full suite on hosted Ubuntu and macOS runners and cross-builds amd64 and arm64 artifacts for each runner OS. Real-provider compatibility is recorded separately in [provider-compatibility.md](provider-compatibility.md), and the security pass is recorded in [threat-review.md](threat-review.md).

[GitHub Actions CI run 29243129560](https://github.com/taiwong148960/intent-sh/actions/runs/29243129560) passed on both hosted Ubuntu and macOS. It covered the full fake-provider/PTY suite with modern Bash and Zsh, explicit stock macOS Bash 3.2 rejection, and Linux/Darwin amd64/arm64 builds.
