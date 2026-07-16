## 1. Test Taxonomy and Strict Qualification Plumbing

- [x] 1.1 Inventory every top-level Go test and check in a manifest that assigns it to local-optional, required, scheduled, or manual qualification with its declared prerequisites and matrix dimensions.
- [x] 1.2 Add repository-owned strict capability helpers so dedicated CI targets fail, rather than skip, when their required Bash, Zsh, tmux, ble.sh, SSH, architecture, or prebuilt-binary prerequisite is missing.
- [x] 1.3 Implement a bounded Go `-json` result auditor that compares executed top-level cases with the manifest, rejects missing or unexpected skipped cases, and records pass/fail/skip/duration plus declared matrix metadata.
- [x] 1.4 Add non-overlapping Make targets for static checks, unit packages, native PTY, tmux, ble.sh, SSH, artifacts, race, coverage, and scheduled stress while preserving a convenient local `make check` path.
- [x] 1.5 Add tests for the result auditor covering renamed or missing cases, nested subtests, expected local skips, forbidden required skips, malformed or excessive input, and terminal-safe bounded output.
- [x] 1.6 Prove through the manifest that the required broad package target no longer duplicates tmux, ble.sh, SSH, or real-provider cases selected by dedicated targets.

## 2. Hermetic ble.sh Fixture and Cache Repair

- [x] 2.1 Resolve and document the pinned ble.sh root commit, required `contrib` gitlink revision, source archive URLs, checksums, expected `BLE_VERSION`, and applicable third-party license metadata.
- [x] 2.2 Rewrite the ble.sh installer to fetch each pinned archive independently, verify it before extraction, assemble a complete source tree in a fresh temporary directory, and publish the built fixture atomically.
- [x] 2.3 Add a fixture manifest and cache verifier that checks regular-file type, root and submodule revisions, checksums, installer revision, expected version, and built-script digest before exporting `INTENT_SH_TEST_BLESH`.
- [x] 2.4 Make partial extraction, missing submodule content, failed build, stale manifest, symlink, wrong digest, and unsupported version fail or rebuild without accepting a poisoned cache.
- [x] 2.5 Add network-free installer tests using local archive overrides for empty-cache, valid-cache, corrupted-cache, incomplete-source, and atomic-publication behavior on both GNU and macOS command variants.
- [x] 2.6 Run the repaired empty-cache and restored-cache build paths on hosted macOS and Linux and verify that every selected ble.sh case executes rather than skips.

## 3. Linux Bash Cancellation and Process Teardown

- [x] 3.1 Extend the deterministic slow fake providers with bounded phase and child-PID markers that can prove the full process tree starts, receives cancellation, exits, and never triggers fallback or a target command.
- [x] 3.2 Add Linux-focused PTY assertions that distinguish terminal-byte `Ctrl+C`, direct process `SIGINT`, timeout, PTY closure, and ordinary shell exit while checking TTY attributes and trap restoration after each path.
- [x] 3.3 Rework the Bash provider/monitor foreground-process-group, signal forwarding, wait, and reap sequence so cancellation is forwarded exactly once and no monitor or provider descendant survives.
- [x] 3.4 Preserve the pre-request buffer and cursor, suppress fallback, restore the caller's signal trap and terminal mode, and prove a new rewrite can run in the same shell after cancellation.
- [x] 3.5 Run the terminal-byte and direct-signal cancellation cases repeatedly on Linux without retries and retain the exact failing phase if any repetition fails.
- [x] 3.6 Re-run native macOS Bash, Zsh, tmux, SSH, and Bash 3.2 ble.sh cancellation matrices to prove the Linux fix does not regress another editor backend.

## 4. Required Workflow Integrity and Supply-Chain Gates

- [x] 4.1 Add a read-only static target covering formatting, `go vet`, `go mod verify`, `go mod tidy -diff`, Bash/Zsh syntax, shell lint, workflow lint, and `openspec validate --all --strict`.
- [x] 4.2 Pin shell/workflow linters and all third-party GitHub Actions to immutable versions or commit SHAs with reviewable version comments and checksum verification where applicable.
- [x] 4.3 Add dependency-update configuration for reviewed Go module and GitHub Actions pin updates without granting write permissions to ordinary CI jobs.
- [x] 4.4 Split the required workflow into stable static, unit/race, native PTY, tmux, ble.sh, loopback SSH, artifact, coverage, and aggregate jobs with explicit timeouts, `fail-fast: false`, and concurrency cancellation.
- [x] 4.5 Add `merge_group` and manual rerun triggers while retaining pull-request and protected-branch push coverage and avoiding path filters that could bypass documentation or specification validation.
- [x] 4.6 Keep required workflow permissions read-only, prohibit privileged fork execution, and add a final aggregate check whose stable name can be used by branch protection.

## 5. Native PTY, Editor, Locale, and Provider Failure Matrix

- [x] 5.1 Parameterize the complete native Bash and Zsh conformance lifecycle by Emacs/Vi editor mode where supported, default/custom chords, CR/LF, `TERM`, locale, and terminal size without duplicating fixture logic.
- [x] 5.2 Add UTF-8 and `C`/`C.UTF-8` locale journeys for mixed English/Chinese buffers, combining characters, non-terminal cursors, resize, failure restoration, regeneration, and undo.
- [x] 5.3 Add shell-level fake-provider journeys for unavailable executable, login/capability failure, timeout fallback, malformed result fallback, excessive output, provider crash, explicit-provider no-fallback, and cancellation no-fallback.
- [x] 5.4 Add PTY teardown cases for shell exit, SIGHUP, terminal closure, and interrupted initialization, proving no provider, monitor, temporary workspace, or target-command side effect survives.
- [x] 5.5 Make the required macOS/Linux native jobs print exact Go, OS, architecture, Bash, Zsh, locale, and terminal fixture versions before running the strict manifest target.
- [x] 5.6 Run the Go race detector on both required host platforms over the appropriate unit and integration-aware packages and make any race a required failure.

## 6. tmux and Full ble.sh Integration Qualification

- [x] 6.1 Convert the isolated tmux target to strict mode on macOS and Linux and verify default/custom chords, CR/LF, resize, cancellation, dangerous confirmation, detach/reattach, intercepted keys, and pane/session isolation.
- [x] 6.2 Ensure every tmux test uses only a private socket, empty configuration, disposable inner home, bounded terminal responses, and cleanup that cannot address the user's default server.
- [x] 6.3 Parameterize the full ble.sh lifecycle so modern Bash on macOS and Linux receives rewrite, regeneration, undo, safety, failure, cancellation, Unicode, initialization, detach, and removal coverage rather than only acceptance-key smoke.
- [x] 6.4 Keep the stock macOS Bash 3.2 positive and fail-closed negative matrices explicit, including missing, incompatible, detached, wrong-load-order, conflicting-binding, and incomplete-API cases.
- [x] 6.5 Emit exact tmux, Bash, ble.sh root/submodule, manifest, and backend versions in bounded job metadata without emitting keymaps, buffer data, or pane contents.
- [x] 6.6 Remove the duplicate tmux execution from the broad package command and verify the strict manifest still contains every intended macOS/Linux tmux and ble.sh case exactly once.

## 7. Ephemeral Loopback SSH and SSH-to-tmux Qualification

- [x] 7.1 Add a Linux CI helper that creates a job-owned OpenSSH server on a high loopback port with temporary host/client keys, strict known-host data, isolated home/account state, disabled passwords and forwarding, and always-run cleanup.
- [x] 7.2 Configure a bounded SSH alias or validated test-only port/path options so the existing harness can reach the loopback target without accepting arbitrary caller-supplied SSH arguments.
- [x] 7.3 Run the existing remote Bash and Zsh conformance lifecycle in strict mode and prove staged binaries, fake providers, configuration, prompts, and target commands remain remote while prohibited SSH and credential markers remain absent.
- [x] 7.4 Add a direct-session disconnect case that starts a slow remote provider, closes the SSH client, and proves no staged provider descendant, fallback, or target-command side effect remains.
- [x] 7.5 Add an SSH-to-tmux journey that detaches the first client, reconnects with a fresh client, restores the same pane's visible buffer and rewrite/confirmation state, and proves a new pane is independent.
- [x] 7.6 Add adversarial tests for unsafe target, port, identity, known-host, staging, and cleanup paths plus lost-connection cleanup restricted to the job-owned `intent-sh-ssh.*` directory.
- [x] 7.7 Add the required loopback SSH job and verify its final cleanup rejects leftover sshd, tmux, provider, remote directory, key, or client-configuration state.

## 8. Distributed Artifact Journeys and Executable Coverage

- [x] 8.1 Extend test helpers to accept and validate a prebuilt `intent-sh` path so artifact journeys cannot silently invoke `go run` or rebuild a different executable.
- [x] 8.2 Build reproducible `darwin/linux` by `amd64/arm64` artifacts with `CGO_ENABLED=0`, record bounded target metadata and checksums, and verify the embedded adapter protocol in each file.
- [x] 8.3 Execute each runner's native artifact from a disposable installation prefix through version, init, setup, config, doctor, fake-provider rewrite, safe/review/dangerous acceptance, downgrade cleanup, and removal journeys.
- [x] 8.4 Inspect non-native artifacts for expected file format, architecture, executability, and bounded version/adapter metadata without treating emulation as PTY qualification.
- [x] 8.5 Add an instrumented native build using Go executable coverage and isolated `GOCOVERDIR` propagation through shell, tmux, and SSH fake-provider journeys.
- [x] 8.6 Merge unit and executable coverage with `go tool covdata`, exclude only documented test-only or foreign-target code, and produce a bounded source-level summary without third-party upload.
- [ ] 8.7 Measure repeated macOS/Linux baselines, check in the aggregate coverage floor and tolerance with its exclusions, and require an intentional policy change for any decrease.
- [x] 8.8 Add the required native artifact, cross-build inspection, and coverage jobs to the aggregate gate without uploading ordinary pull-request binaries unnecessarily.

## 9. Scheduled Stress, Compatibility, Fuzz, and Security Matrix

- [x] 9.1 Add a scheduled and manually dispatchable workflow with bounded concurrency, explicit timeouts, stable matrix metadata, and no unconditional retry-to-success behavior.
- [x] 9.2 Run native PTY and tmux suites with logged shuffle seeds and repeat counts, retaining the exact seed, repetition, platform, shell, and phase for any failure.
- [x] 9.3 Give every registered Go fuzz target an independent bounded budget, verify corpus minimization, and retain only privacy-safe failing corpus artifacts.
- [x] 9.4 Bootstrap pinned minimum/current Bash and selected Zsh versions and run the applicable native or ble.sh lifecycle with exact source revisions and checksums.
- [x] 9.5 Add justified Linux distribution/libc and available native arm64 dimensions, marking unavailable capacity explicitly rather than silently skipping or claiming qualification.
- [x] 9.6 Add pinned, unauthenticated Claude/Codex CLI capability probes that verify required flags and expected login-not-ready handling without generating a model request or reading credentials.
- [x] 9.7 Add pinned Go vulnerability and other security checks whose external databases are recorded with the run, keeping their scheduled results visible and actionable.

## 10. Manual Trusted Qualification Boundaries

- [x] 10.1 Add a manually dispatched protected workflow or documented local entry point for selecting real Claude/Codex smoke and an explicitly supplied external SSH target without exposing either to pull requests.
- [x] 10.2 Keep real-provider output limited to provider name, bounded compatible version, and pass/fail, and disable general log-artifact upload for authenticated smoke.
- [x] 10.3 Validate all external SSH inputs, require pre-existing BatchMode authentication and known-host state, disable forwarding, and retain the existing bounded remote staging and cleanup contract.
- [x] 10.4 Add workflow-policy tests or lint rules that reject writable default permissions, secret-bearing fork jobs, mutable privileged checkout paths, and `pull_request_target` execution of repository code.
- [x] 10.5 Preserve named GUI and integrated terminal qualification as a manual dated record and document that hosted PTY success does not automatically refresh those records.

## 11. Sanitized Evidence, Documentation, and Threat Review

- [x] 11.1 Upload seven-day sanitized structured results for failed deterministic fake-provider jobs, including manifest case, phase, duration, matrix, tool versions, and shuffle seed where applicable.
- [x] 11.2 Add adversarial evidence tests proving prompts, generated commands from real services, raw PTY or pane streams, arbitrary environment values, history, SSH private material, and provider credentials are omitted or cause evidence generation to fail closed.
- [x] 11.3 Document every Make/CI target, prerequisite, strict-versus-local behavior, exact reproduction command, expected skip policy, job timeout, and trust tier in the development guide.
- [x] 11.4 Update the README with the required CI summary, loopback versus external SSH distinction, complete pinned ble.sh fixture process, scheduled matrix, and manual provider/terminal boundaries.
- [x] 11.5 Extend the threat review for action pinning, downloaded fixture integrity, cache validation, fork permissions, ephemeral sshd privilege, evidence redaction, coverage paths, and cleanup ownership.
- [x] 11.6 Update provider and terminal compatibility documentation with the new automated evidence scope while retaining dated real-provider and named-terminal results separately.

## 12. End-to-End Verification and Rollout

- [x] 12.1 Run formatting, module integrity, vet, shell/workflow lint, strict OpenSpec validation, unit, race, native PTY, tmux, ble.sh, loopback SSH, artifact, and coverage targets from clean disposable state.
- [x] 12.2 Verify the required manifest reports no missing, duplicated, or unexpectedly skipped cases and that ordinary local tests still skip unavailable opt-in fixtures cleanly.
- [x] 12.3 Exercise empty and restored caches, interrupted jobs, failure evidence, and always-run cleanup paths for ble.sh, tmux, SSH, coverage, and temporary provider workspaces.
- [x] 12.4 Obtain consecutive green required workflow runs on macOS and Linux, then remove the superseded monolithic or duplicate paths and retain the stable aggregate check name.
- [ ] 12.5 Manually dispatch a bounded scheduled run and verify shuffle, repeat, fuzz, shell-version, distribution, architecture, provider-capability, and vulnerability results are reproducible from recorded metadata.
- [x] 12.6 Document the branch-protection check name and maintainer update procedure without mutating repository protection settings from CI.
- [x] 12.7 Run `openspec validate --all --strict`, review the final diff for secret or generated-test artifacts, and confirm every new CI requirement has a deterministic automated case or an explicit protected/manual qualification step.
