# Development and qualification

Development and qualification run on macOS. Use the Go version declared in `go.mod`, Node 20.19 or newer for the pinned OpenSpec package, Zsh, and Bash 4.0 or newer. The default loop is `make check`; it is intentionally local-friendly, so opt-in integrations may skip when they are not configured. Dedicated qualification targets enable strict mode, select only their manifest-owned cases, and fail when a selected test or prerequisite is unavailable.

The checked-in manifest is `.github/ci/test-manifest.json`. Its auditor retains only case identity, bounded phase, result, duration, and allow-listed Darwin, architecture, shell, tool, fixture, and seed metadata. Never add a skip merely to make a required job green.

## Make targets

| Target | Purpose and reproduction | Qualification tier |
| --- | --- | --- |
| `build` | Build the native macOS CLI with `make build`. | Local |
| `fmt`, `fmt-check`, `vet`, `module-check` | Format Go or verify formatting, static analysis, module checksums, and a tidy module graph. | Required static, except mutating `fmt` |
| `shell-check`, `shell-lint` | Parse the adapters/helpers and run the checksum-pinned ShellCheck binary. | Required static |
| `workflow-lint` | Validate GitHub Actions with pinned actionlint. | Required static |
| `openspec-check` | Strictly validate active and baseline OpenSpec artifacts. | Required static |
| `platform-scope-audit` | Reject first-party active references outside the macOS support boundary; preserved change records and generated dependency locks are excluded explicitly. | Required static |
| `supported-builds` | Build `darwin/arm64` and `darwin/amd64` with CGO disabled. | Required static inspection |
| `ci-tools` | Install checksum-pinned Darwin ShellCheck/actionlint archives and the npm integrity lock. | Tool bootstrap |
| `static-check` | Aggregate every read-only static gate and both supported builds. | Required |
| `test`, `test-unit` | Run ordinary tests, or audit the strict non-integration inventory with `make test-unit QUALIFICATION_DIR=/absolute/results`. | Local / required |
| `shelltest-harness-test` | Verify strict-mode, private tmux state, SSH target/config/cleanup validation, Darwin remote parsing, marker privacy, and absent-target behavior without opening SSH. | Required |
| `native-pty-test` | Run Bash/Zsh editor, chord, CR/LF, explicit locale, `TERM`, resize, Unicode, provider failure, safety, cancellation, setup, downgrade, and removal journeys. Set `INTENT_SH_TEST_BASH=/path/to/bash` when needed. | Required |
| `tmux-test` | Run the lifecycle and detach/reattach matrix on a private socket and empty config. Set `INTENT_SH_TEST_TMUX=/path/to/tmux`. | Required |
| `ssh-opt-in-test` | Prove ordinary execution contacts no SSH target. | Required |
| `external-ssh-test` | Run protected Bash/Zsh, disconnect, and SSH-to-tmux journeys with `INTENT_SH_TEST_SSH_TARGET=user@known-macos-host make external-ssh-test`. The target must be macOS on arm64 or amd64 with existing BatchMode authentication, known-host state, Bash 4+, Zsh, and tmux. | Protected manual |
| `artifact-build` | Build both Darwin artifacts twice and reject byte differences. Set `ARTIFACT_DIR=/absolute/empty/artifacts`. | Required |
| `artifact-inspect` | Verify regular executable shape, Mach-O CPU, Go metadata, CGO setting, trimpath, checksum, and embedded protocol. | Required inspection |
| `artifact-test` | Execute the full journey against exactly one runner-native prebuilt artifact. Set `INTENT_SH_TEST_BINARY=/absolute/artifacts/intent-sh-darwin-$(go env GOARCH)`. | Required native |
| `race-test` | Run the race detector over unit packages and integration-harness safety tests. | Required |
| `coverage-test` | Merge unit coverage with instrumented native PTY and tmux journeys: `COVERAGE_DIR=/absolute/empty/coverage make coverage-test`. | Required |
| `scheduled-stress` | Shuffle and repeat PTY/tmux with an exact seed: `STRESS_COUNT=3 SHUFFLE_SEED=12345 make scheduled-stress QUALIFICATION_DIR=/absolute/results`. | Scheduled |
| `shell-compatibility-test` | Exercise a checksum-pinned source build selected from `bash-4.0`, `bash-5.3`, `zsh-5.8.1`, or `zsh-5.9.1`. | Scheduled |
| `real-provider-test` | Run one harmless authenticated smoke, for example `INTENT_SH_REAL_PROVIDER_SMOKE=codex make real-provider-test`. | Protected manual |
| `check` | Run the convenient formatting, vet, ordinary-test, and shell-parse loop. | Local |

`QUALIFICATION_DIR`, `ARTIFACT_DIR`, and `COVERAGE_DIR` must be disposable absolute locations. Evidence files never retain Go test output, prompts, commands, PTY/pane bytes, environment values, history, SSH material, or provider credentials. Symlinked and non-regular destinations fail closed. Failure summaries expire after seven days; the workflow-internal artifact bundle expires after one day.

## GitHub Actions tiers

The required workflow runs for `main`, pull requests, merge groups, and manual reruns with read-only contents permission, immutable action references, explicit timeouts, and concurrency cancellation. Every job uses the explicit `macos-15` hosted runner. Its stable branch-protection check is `required / aggregate`.

| Tier | Jobs and timeout in minutes |
| --- | --- |
| Required | static 15; unit 15; race 20; native PTY 35; tmux 30; reproducible artifacts 20; Mach-O inspection 10; native artifact 40; executable coverage 70; aggregate 5 |
| Scheduled/manual | PTY/tmux stress 90; independent fuzz 15; pinned shell compatibility 50; unauthenticated provider capability 15; module/vulnerability security 20; macOS architecture evidence 5 |
| Protected manual | authenticated provider smoke 10; prepared macOS SSH 45; both require `[self-hosted, macOS, intent-sh-trusted]` and `trusted-qualification` |

Required suites reject missing tests, duplicate or unexpected execution, mandatory skips, and missing prerequisites. Required CI proves only that SSH is opt-in and that the harness preserves its safety/privacy boundary; it never contacts a target. Scheduled jobs record bounded seed, repetition, architecture, fixture, provider version, and vulnerability-database evidence. The current hosted runner executes its native artifact; the other supported Mach-O artifact receives inspection evidence only.

## Failure reproduction and cleanup

Copy the failed job's bounded metadata and invoke the matching target. For stress failures retain `SHUFFLE_SEED` and `STRESS_COUNT`; for shell compatibility retain the fixture name. Do not paste raw workflow logs into an issue if they may contain runner or account data.

Every integration owns a narrow cleanup namespace:

- tmux kills only its random private server and removes its mode-0700 socket directory;
- shell source builders atomically replace only their configured fixture cache;
- external SSH removes only the validated remote `intent-sh-ssh.*` directory it created; after a lost connection, verify ownership before removing that one namespace manually;
- provider workspaces and coverage directories are disposable and must never point at user data.

## Branch protection

Require the exact status check `required / aggregate`. CI never edits repository protection settings. If a required check name changes, first land the new macOS job, obtain consecutive green runs, update the repository rule, and only then remove the superseded check. Record the transition so a temporary protection gap remains reviewable.
