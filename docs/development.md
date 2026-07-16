# Development and qualification

The default developer loop is `make check`. It is intentionally local-friendly: ordinary Go tests may skip opt-in integrations that are not configured. A dedicated qualification target is different: it sets strict mode, selects only its manifest-owned cases, and fails if any selected case or prerequisite is missing. The checked-in manifest is `.github/ci/test-manifest.json`; the auditor records only case identity, bounded phase, result, duration, and allow-listed matrix metadata.

Use Go from `go.mod`, Node 20.19 or newer for the pinned OpenSpec package, and Bash plus Zsh. Individual integration targets list their extra prerequisites below. Do not add a skip to make required CI green: either supply the declared prerequisite, keep the test local-optional in the manifest, or make the job fail with a bounded diagnostic.

## Make targets

| Target | Purpose and prerequisite | Behavior, reproduction, and trust tier |
| --- | --- | --- |
| `build` | Build the native CLI; Go. | `make build`. Local, no integration prerequisite. |
| `fmt` | Rewrite repository Go files with `gofmt`; Go. | `make fmt`. Local mutation; not invoked by CI. |
| `fmt-check` | Verify formatting without rewriting. | `make fmt-check`. Required static. |
| `vet` | Run Go static analysis. | `make vet`. Required static. |
| `module-check` | Verify downloaded module content and a clean module graph. | `make module-check` runs `go mod verify` and `go mod tidy -diff`. Required static. |
| `shell-check` | Parse the Bash and Zsh adapters and repository shell helpers. | `make shell-check`; requires Bash and Zsh. Required static. |
| `shell-lint` | Run the checksum-pinned ShellCheck binary. | `make ci-tools shell-lint`. Required static. |
| `workflow-lint` | Run checksum-pinned actionlint with the declared trusted-runner label. | `make ci-tools workflow-lint`. Required static. |
| `openspec-check` | Run the lockfile-pinned OpenSpec CLI. | `make ci-tools openspec-check`. Required static. |
| `supported-builds` | Cross-build darwin/linux by amd64/arm64 with CGO disabled. | `make supported-builds`. Required static inspection, not PTY qualification. |
| `ci-tools` | Verify/download pinned actionlint and install the npm integrity lock. | `make ci-tools`; requires network only on an empty tool cache. Tooling bootstrap. |
| `static-check` | Aggregate formatting, vet, module, shell, workflow, OpenSpec, and supported-build checks. | `make static-check`. Required, read-only with respect to source; tool caches may be populated. |
| `test` | Run all ordinary Go tests. | `make test`. Local-optional integrations may skip with their documented reason. |
| `test-unit` | Audit the non-integration package inventory. | `make test-unit QUALIFICATION_DIR=/absolute/empty/results`. Required; no skip is allowed. |
| `shelltest-harness-test` | Test strict-mode, tmux isolation, SSH validation, and absent-target behavior without opening SSH. | `make shelltest-harness-test`. Required; no external service. |
| `native-pty-test` | Bash/Zsh Emacs/Vi, chord, CR/LF, locale, TERM, resize, Unicode, provider-failure, safety, cancellation, teardown, setup, and removal journeys. | `INTENT_SH_TEST_BASH=/path/to/bash make native-pty-test QUALIFICATION_DIR=/absolute/results`; requires Bash 4+, Zsh, and PTYs. Required; missing capabilities fail. |
| `tmux-test` | Full lifecycle, cancellation, resize, safety, detach/reattach, and pane/session isolation. | `INTENT_SH_TEST_TMUX=/path/to/tmux make tmux-test QUALIFICATION_DIR=/absolute/results`; uses only a private socket and empty config. Required. |
| `bash32-negative-test` | Prove stock macOS Bash 3.2 stays inert without ble.sh. | `INTENT_SH_TEST_BASH32=/bin/bash make bash32-negative-test QUALIFICATION_DIR=/absolute/results`. Required macOS boundary. |
| `blesh-fixture-test` | Exercise network-free empty, valid, corrupt, incomplete-runtime, symlink, and atomic-publication cache paths. | `make blesh-fixture-test`. Required on macOS and Linux; uses local synthetic archives. |
| `blesh-test` | Run the full ble.sh lifecycle. | `export INTENT_SH_TEST_BLESH="$(bash .github/scripts/install-blesh-test.sh)"`; then `make blesh-test BLESH_SUITE=blesh-modern QUALIFICATION_DIR=/absolute/results`. For Bash 3.2 also set `INTENT_SH_TEST_BASH32` and use `BLESH_SUITE=blesh-bash32`. Required; no selected skip. |
| `ssh-opt-in-test` | Prove ordinary execution does not contact SSH without explicit configuration. | `make ssh-opt-in-test`. Required safety guard. |
| `ssh-test` | Loopback remote Bash/Zsh, disconnect teardown, and SSH-to-tmux state isolation. | On Linux set `RUNNER_TEMP=/absolute/job-dir`, run `.github/scripts/setup-loopback-ssh.sh start`, export the printed target and generated config plus `INTENT_SH_TEST_SSH_LOOPBACK=1`, run `make ssh-test QUALIFICATION_DIR=/absolute/results`, and always run the helper with `stop`. Required; the CI job owns the daemon/account/keys. |
| `external-ssh-test` | The same remote journeys against a caller-owned host. | `INTENT_SH_TEST_SSH_TARGET=user@known-host make external-ssh-test`. Protected manual only; requires existing BatchMode authentication, known-host state, Bash 4+, Zsh, and tmux on the target. It never creates credentials. |
| `artifact-build` | Build every supported artifact twice and reject byte differences. | `ARTIFACT_DIR=/absolute/empty/artifacts make artifact-build`. Required supply-chain journey. |
| `artifact-inspect` | Verify regular executable type, Mach-O/ELF OS/arch, Go build info, CGO=0, trimpath, module, checksum, and adapter protocol. | `ARTIFACT_DIR=/absolute/artifacts make artifact-inspect`. Required; safe for foreign artifacts. |
| `artifact-test` | Run the complete native PTY/setup/safety/removal journey against exactly one prebuilt executable. | `INTENT_SH_TEST_BINARY=/absolute/artifacts/intent-sh-$(go env GOOS)-$(go env GOARCH) make artifact-test QUALIFICATION_DIR=/absolute/results`. Required; rebuilding or `go run` is prohibited. |
| `race-test` | Run the race detector over unit packages and integration-aware harness safety tests. | `make race-test`. Required on macOS and Linux. |
| `coverage-test` | Merge unit coverage with instrumented native, tmux, and loopback-SSH executable journeys. | With the loopback SSH fixture and tmux configured, run `COVERAGE_DIR=/absolute/empty/coverage make coverage-test`. Required Linux; floor 80.0%, tolerance 0.5%, exclusions are checked in at `.github/ci/coverage-policy.env`. |
| `scheduled-stress` | Shuffle and repeat native PTY and tmux with an exact seed. | `STRESS_COUNT=3 SHUFFLE_SEED=12345 make scheduled-stress QUALIFICATION_DIR=/absolute/results`. Scheduled/manual; accepted counts are 2, 3, or 5. |
| `shell-compatibility-test` | Run the applicable Emacs/Vi lifecycle with a pinned source-built shell. | Select `INTENT_SH_SHELL_FIXTURE=bash-4.0`, `bash-5.3`, `zsh-5.8.1`, or `zsh-5.9.1`; run `.github/scripts/install-shell-compat.sh`, export its name/path plus `INTENT_SH_TEST_COMPAT_FIXTURE`, then `make shell-compatibility-test`. Scheduled; a missing build fails. |
| `real-provider-test` | One harmless authenticated Claude, Codex, or combined smoke. | `INTENT_SH_REAL_PROVIDER_SMOKE=codex make real-provider-test`. Protected manual only; output is reduced to provider, bounded version, and pass/fail and is never uploaded. |
| `check` | Convenient format/vet/ordinary-test/shell-parse loop. | `make check`. Local-optional; it is not the merge gate. |

When `QUALIFICATION_DIR` is supplied it must be a disposable location. The emitted JSON never contains Go test output, prompts, commands, PTY/pane bytes, environment values, history, SSH material, or provider credentials. Existing symlink/non-regular destinations fail closed. Required and scheduled workflows upload these summaries only after failure, retain them for seven days, and do not upload normal pull-request binaries except the one-day workflow-internal artifact bundle needed by native/cross inspection jobs.

## GitHub Actions tiers and timeouts

The required workflow runs on pushes to `main`, pull requests, merge queue groups, and manual reruns. It has read-only contents permission, immutable action refs, `fail-fast: false` matrices, explicit timeouts, and concurrency cancellation. Its stable branch-protection check is `required / aggregate`.

| Workflow tier | Jobs and timeout in minutes |
| --- | --- |
| Required | static 15; fixture installer 10; unit 15; race 20; native PTY 35; tmux 30; stock Bash 3.2 negative 15; ble.sh 40; loopback SSH 35; reproducible artifact build 20; cross-artifact inspection 10; native artifact 40; executable coverage 70; aggregate 5. |
| Scheduled/manual dispatch | PTY/tmux stress 90; independent fuzz 15; pinned shell compatibility 50; Ubuntu glibc distributions 45; native Linux arm64 60; unauthenticated provider capability 15; vulnerability/module security 20; capacity boundary 5. |
| Protected manual dispatch | authenticated provider smoke 10; caller-owned external SSH 45. Both require `[self-hosted, intent-sh-trusted]` and the `trusted-qualification` environment. |

Required jobs treat every manifest-selected skip, missing top-level test, duplicate/unexpected execution, or absent prerequisite as a failure. `make test` remains the only broad local path where opt-in SSH, ble.sh, tmux, or real-provider tests may report an expected skip.

The scheduled workflow records the shuffle seed/repeat, runs each registered fuzz target with its own budget, source-builds checksum-pinned minimum/current shells, qualifies Ubuntu 22.04/24.04 and native hosted arm64, probes exact unauthenticated provider CLIs without a model request, and records the pinned vulnerability scanner/database revision. musl PTY capacity is explicitly `UNAVAILABLE`; foreign artifacts are inspected, never reported as emulated PTY qualification.

## Failure reproduction and cleanup

Copy the failed job's exact matrix metadata and invoke the corresponding target above. For stress failures, also copy `SHUFFLE_SEED` and `STRESS_COUNT`; for shell compatibility copy the fixture name; for cancellation use the safe nested phase recorded by the auditor. Do not copy raw Actions logs into an issue if they may contain runner or account data.

Every integration owns a narrow cleanup namespace:

- tmux kills only its random private server and removes its mode-0700 socket directory;
- ble.sh and shell source builders atomically replace only their configured fixture cache;
- loopback SSH removes its generated account, home, daemon, keys, config, tmux/provider processes, and `$RUNNER_TEMP/intent-sh-loopback-ssh` state in an `always()` step;
- external SSH removes only the validated remote `intent-sh-ssh.*` directory it created; after a lost connection, verify ownership before removing that one namespace manually;
- provider workspaces and coverage directories are disposable and must not be pointed at user data.

## Branch protection

Configure the protected branch to require the exact status check `required / aggregate`. CI never edits repository protection settings. When changing the check graph or name, maintainers should first land the new job, obtain consecutive green macOS/Linux required runs, update the branch rule in repository settings, and only then remove the superseded check. Record the rename in this guide and in the change that updates the workflow so a temporary protection gap is reviewable.
