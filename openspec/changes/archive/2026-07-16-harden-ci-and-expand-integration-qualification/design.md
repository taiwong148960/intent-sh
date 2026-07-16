## Context

The repository currently has one GitHub Actions workflow with a macOS/Linux verification matrix and a separate macOS/Linux ble.sh matrix. The underlying test suite is unusually strong for an MVP: it builds the real binary, launches Bash and Zsh behind pseudo-terminals, uses deterministic fake Claude and Codex executables, exercises tmux through a private socket, and contains an opt-in SSH harness and real-provider smoke test. The normal package command nevertheless treats external capabilities as optional and currently skips nineteen top-level integration tests when ble.sh, an SSH target, and authenticated providers are absent.

The latest protected-branch run is not a usable baseline. Both ble.sh jobs build a checksum-verified GitHub source archive that omits the required `contrib` submodule, and the Linux native job repeatedly fails Bash cancellation while the equivalent macOS tests pass. tmux also runs through the broad package command and then runs again through its explicit target. The result is a red pipeline with duplicated work and important suites that can silently disappear.

The product supports macOS and Linux on amd64 and arm64, Zsh, Bash 4.0+ native Readline, and stock macOS Bash 3.2 only through one pinned ble.sh version. Its security boundary forbids automatic command execution, credential handling, terminal-screen capture, and arbitrary environment collection. CI must preserve those boundaries, remain safe for forked pull requests, and distinguish deterministic fake-provider qualification from checks that inherently require external credentials or a real terminal application.

## Goals / Non-Goals

**Goals:**

- Restore a green, repeatable hosted-runner baseline before adding matrix breadth.
- Give each required integration family an explicit target, prerequisites, stable check name, timeout, and unexpected-skip policy.
- Exercise the real compiled binary through native PTYs, isolated tmux, pinned ble.sh, and an ephemeral loopback SSH server.
- Cover both supported operating systems, all four build targets, supported shell/editor paths, process teardown, provider failure paths, and installation journeys at an appropriate CI tier.
- Detect races, ordering dependencies, fuzz failures, and meaningful coverage regressions without making pull-request latency unbounded.
- Retain enough sanitized evidence to reproduce failures without retaining prompts, commands, terminal contents, arbitrary environment values, or credentials.
- Keep local developer commands useful even when optional external fixtures are not installed.

**Non-Goals:**

- Add Fish, Nushell, PowerShell, Windows, WSL, BSD, unsupported Bash versions, or new runtime terminal integrations.
- Change adapter protocol 2, provider request semantics, command validation, risk policy, configuration format, or the user-visible workflow except to fix existing cross-platform defects.
- Drive GUI terminal applications from hosted CI or treat pseudo-terminal coverage as proof of a named terminal qualification.
- Expose provider or SSH credentials to forked pull requests, use `pull_request_target` to execute an untrusted checkout, or make real-provider generation a required merge gate.
- Make transient retries, larger timeouts, or ignored failures the primary response to a reproducible integration defect.
- Manage repository branch-protection settings automatically; the repository will provide stable check names and maintainers can update protection after the new gates are green.

## Decisions

### 1. Use three workflow trust and cost tiers

The repository will use three explicit tiers:

1. A required workflow for `pull_request`, protected-branch `push`, merge queue, and manual rerun. It contains static integrity, unit/race, native PTY, tmux, ble.sh, loopback SSH, artifact, and final aggregate jobs. Job names remain stable for branch protection, matrix failures do not cancel their peers, and every job has a bounded timeout.
2. A scheduled workflow, also manually dispatchable, for repeated and shuffled PTY/tmux runs, bounded fuzzing, minimum/current shell versions, representative Linux distributions, available native architectures, provider CLI capability probes without authentication, and vulnerability checks whose external databases can change independently of a pull request.
3. A manually dispatched protected qualification path for authenticated real providers and user-supplied external SSH targets. Named GUI and integrated terminal results continue to use the checked-in manual qualification record.

The required workflow will target a short wall-clock time through independent parallel jobs rather than by omitting invariants. The nightly workflow can spend a larger bounded budget and will never convert a final failure into success merely because a retry passed.

Alternatives considered: one monolithic workflow makes trust boundaries and required status names unclear; putting every stress dimension on every pull request wastes hosted capacity and encourages disabling valuable tests; keeping all external integrations manual permits silent regressions in code that already has deterministic fake-provider harnesses.

### 2. Put suite selection and strictness in repository-owned targets

The `Makefile` and small repository-owned CI helpers will define non-overlapping targets such as static checks, unit tests, native PTY, tmux, ble.sh, SSH loopback, artifact smoke, race, coverage, and nightly stress. Broad unit/package targets will exclude dedicated external integration cases so tmux and other suites do not run twice.

Tests will retain friendly local behavior: a developer without tmux, ble.sh, or SSH configuration can still run the ordinary local suite and see an intentional skip. Dedicated CI targets will set explicit `INTENT_SH_REQUIRE_*` capability flags or equivalent typed test options. A missing prerequisite then becomes `t.Fatal`, not `t.Skip`. Commands will emit `go test -json`; a small Go-based auditor will compare top-level results with a checked-in manifest, fail on missing or unexpectedly skipped mandatory cases, and emit a bounded summary. This source-level strict mode plus result audit prevents both accidental selection drift and a helper silently skipping an entire matrix.

Alternatives considered: parsing human `go test -v` output is fragile; relying only on test-name regular expressions makes renames silently remove coverage; making every optional fixture mandatory for local `go test ./...` harms normal development.

### 3. Treat Linux cancellation as a product defect, not a flaky test

Before the matrix expands, the Bash cancellation implementation and tests will be reconciled with Linux foreground-process-group and job-control behavior. Terminal-byte `Ctrl+C` remains the primary user-contract test. The direct `os.Interrupt` case remains a separate bounded signal-path regression rather than pretending it is identical to terminal input.

The implementation must leave the provider and monitor processes in known groups, forward cancellation exactly once, reap every child, restore the original TTY attributes and signal trap, stop routing before fallback, restore the original editable buffer, and leave the interactive shell usable. Fake providers will record bounded child PIDs so the test can prove the complete process tree is gone. The fix will be stressed repeatedly on Linux; retries and suppressed output are not accepted fixes.

Alternatives considered: dropping the Linux signal case would hide a real platform difference; increasing the seven-second read timeout does not address a provider or monitor that never receives cancellation; weakening the assertion would contradict the existing shell workflow specification.

### 4. Build ble.sh from independently pinned complete source inputs

The current root GitHub archive is checksum-verified but incomplete because GitHub archives do not contain submodule contents. The fixture installer will instead resolve the pinned root commit and every required gitlink to separately downloaded source archives with committed checksums, assemble them under a fresh temporary source tree, and run the upstream build without asking `git submodule` to repair an archive. The implementation will record the exact root commit, submodule commit, expected `BLE_VERSION`, archive checksums, and installer revision in a small manifest.

The cache key will include the operating system, all pinned revisions/checksums, and installer hash. Restoration is not trust: every hit must contain a regular built script, matching manifest, expected version, and optional committed artifact digest before `INTENT_SH_TEST_BLESH` is exported. A partial extraction or failed build is assembled outside the cache destination and cannot masquerade as a valid hit. The ordinary test path remains network-free.

Alternatives considered: a recursive shallow clone is simpler but depends on live git/submodule behavior and does not preserve the existing explicit archive checksum boundary; vendoring the large third-party editor in the product repository expands review and licensing surface; downloading an unpinned prebuilt script weakens reproducibility.

### 5. Use an ephemeral loopback OpenSSH server as the required transport target

The Linux SSH job will install or use OpenSSH and tmux, create temporary server and client host keys, an isolated account/home or otherwise isolated runner identity, a high local port, a strict known-host entry, and a private client configuration alias. Forwarding, agents, X11, local commands, password prompts, host-key updates, and multiplexed control connections remain disabled. The existing harness will continue to validate bounded target and remote temporary paths and will stage only the cross-built binary, fake providers, and secret-free configuration.

The first required journey runs the existing remote Bash/Zsh lifecycle. Additional cases disconnect during a slow direct session and assert bounded process cleanup, then create a remote tmux shell, detach the first SSH client, reconnect, and prove state remains in the live pane while a new pane is independent. Workflow cleanup terminates only the job-owned sshd, removes keys/configuration, and checks for leftover `intent-sh-ssh.*` directories owned by the test account.

The test can use an isolated SSH config alias so the production harness does not need arbitrary command-line option injection. If hosted-runner constraints require explicit port or identity fields, the harness may accept only validated numeric ports and regular paths rooted in a declared test directory.

Alternatives considered: an external SSH host adds credentials, availability, and cleanup uncertainty to every pull request; a mocked transport does not test PTY allocation, signal delivery, quoting, staging, or remote process locality; enabling the runner's default sshd and user configuration risks state leakage.

### 6. Expand behavior through a deliberate required and scheduled matrix

Required native jobs will use the supported current shell available on each macOS/Linux runner and cover both Bash native Readline and Zsh, default and custom chords, CR/LF, representative `TERM` values, Unicode, resize, provider routing, risk behavior, cancellation, and concurrent sessions. Emacs and Vi editor modes will share the full lifecycle where the adapter claims support. The macOS ble.sh job will cover stock Bash 3.2; macOS and Linux jobs will exercise the full applicable modern-Bash ble.sh lifecycle rather than only acceptance keys.

Scheduled jobs add the compatibility breadth that is expensive to bootstrap: the Bash 4.0 boundary and a current Bash, selected Zsh versions, representative glibc and musl or other justified Linux environments, repeat/shuffle counts, and native arm64 when an appropriate runner is available. Every matrix entry prints exact versions and declares whether it is required or exploratory. Adding a new dimension requires a matching owner, timeout, and reproduction command.

Alternatives considered: only rolling `-latest` runners miss the minimum supported shell boundary; pinning every distribution and shell combination in required CI creates a Cartesian explosion without strengthening independent invariants; QEMU is useful for artifact inspection but is not a reliable substitute for native PTY and signal qualification.

### 7. Test the distributed binary and collect integration-aware coverage

The artifact job will produce the four supported `GOOS`/`GOARCH` combinations with `CGO_ENABLED=0` and reproducible flags. Each host will execute its native artifact from a disposable installation prefix through `version`, `init`, `setup`, configuration, doctor, and fake-provider PTY journeys. Cross artifacts will be checked with the host's format/architecture inspection tool and by verifying embedded adapter/version metadata without executing the foreign binary.

Test helpers will accept a validated prebuilt binary path so an artifact journey does not silently rebuild a different executable. For coverage, a dedicated native build will use Go's executable coverage support and an isolated `GOCOVERDIR`; unit and executable counters will be merged with `go tool covdata`. Test-only packages and unreachable foreign-architecture binaries will be excluded explicitly. Implementation will record the measured baseline and adopt a stable aggregate floor with a small documented tolerance, then require an intentional policy edit to lower it.

Alternatives considered: package-only coverage leaves `cmd/intent-sh` at zero and misses spawned executable paths; uploading every built artifact from every pull request adds cost without diagnostic value; executing foreign binaries under emulation conflates artifact construction with PTY correctness.

### 8. Keep workflow logic reviewable and supply-chain inputs immutable

Third-party actions will be referenced by full commit SHA with a version comment, and dependency automation may propose reviewed updates. CI-only tools will be installed from pinned versions and verified checksums or invoked through the pinned Go module graph. `go mod verify`, `go mod tidy -diff`, shell syntax/lint, workflow lint, and `openspec validate --all --strict` run without rewriting the checkout. External vulnerability databases and provider CLI releases are checked on the scheduled tier so an upstream change cannot make an unrelated pull request nondeterministically unmergeable; actionable results remain visible and can be promoted to a required pin update.

Alternatives considered: mutable major action tags are convenient but expand the unreviewed execution boundary; installing `latest` linters makes old commits irreproducible; omitting workflow lint allows expression and matrix mistakes to fail only after push.

### 9. Retain structured evidence only for deterministic, non-secret tests

Every required test command will retain a sanitized JSON summary containing package/test name, pass/fail/skip status, duration, declared matrix values, shuffle seed when relevant, and bounded failure text. Failed deterministic jobs upload that summary and selected fixture-only diagnostics for seven days. Raw PTY streams, pane capture, prompts, generated commands from real services, arbitrary environment dumps, SSH private material, provider output, and credential files are never artifacts.

The manual real-provider workflow will report only provider name, bounded compatible version, and pass/fail, matching the existing smoke contract; it will not upload a general test log artifact. All ordinary workflow permissions remain read-only, ephemeral SSH credentials exist only inside the loopback job, and no workflow executes fork code with privileged event credentials.

Alternatives considered: uploading complete terminal transcripts conflicts with the product privacy boundary; retaining nothing makes hosted-only failures difficult to reproduce; third-party coverage/reporting services introduce an unnecessary data-sharing dependency for this repository.

## Risks / Trade-offs

- [More required jobs consume additional hosted-runner minutes] → Partition suites without duplication, cache only verified dependencies, parallelize independent jobs, and keep version/distro repetition in the scheduled tier.
- [PTY, signal, or tmux tests become flaky under load] → Use deterministic fake providers and phase markers, assert process ownership, record exact seeds/versions, and fix root causes instead of adding unconditional retries.
- [The loopback sshd setup gains excessive privilege] → Confine privileged operations to one Linux job, use a temporary account/configuration and high port, disable forwarding and passwords, and prove cleanup in an always-run step.
- [A cached external fixture is incomplete or poisoned] → Key by every pinned input, assemble atomically, and verify manifest, type, version, and digest after every restoration.
- [The minimum shell matrix becomes expensive or unavailable] → Bootstrap it only in scheduled CI and keep the current supported runner paths required; mark unavailable runner dimensions explicitly rather than claiming qualification.
- [Coverage from subprocesses is unstable] → Use isolated coverage directories, merge only completed profiles, exclude declared test-only code, and establish the floor from repeated baseline measurements.
- [Sanitized evidence omits the clue needed for a failure] → Preserve structured phase and process-state markers in fixtures while continuing to exclude user-like content and credentials.
- [Stable check names change during workflow splitting] → Introduce a final required aggregate job, keep names stable across matrix changes, and update branch protection only after parallel green runs.

## Migration Plan

1. Repair the current ble.sh source assembly and Linux Bash cancellation behavior in the existing workflow, and obtain consecutive green macOS/Linux baselines.
2. Add non-overlapping Make/CI targets, capability-required test modes, and the structured skip auditor while retaining the current workflow as a comparison path.
3. Split required static, unit/race, native PTY, tmux, ble.sh, and artifact jobs; run old and new selections in parallel until their intended case manifests agree, then remove duplicate broad execution.
4. Add and harden the isolated loopback OpenSSH job, followed by direct disconnect and SSH-to-tmux reattach cases.
5. Add prebuilt-artifact journeys, executable coverage, the documented floor, and bounded failure evidence.
6. Add scheduled shell/distro/architecture, shuffle/repeat, fuzz, vulnerability, and unauthenticated provider-capability jobs.
7. Add the protected manual provider/external-SSH entry point, document trust boundaries and reproduction commands, and update branch protection to the stable aggregate check.

Rollback is workflow-local: disable or revert a newly introduced job while retaining its repository-owned local target and failure evidence, then restore it after correction. The current required path is not removed until its replacement has passed in parallel. Product code rollback follows the existing binary/adapter rollback documentation; this change introduces no user data migration or configuration schema change.

## Open Questions

- Which explicit hosted-runner labels and native arm64 capacity will be available when implementation lands; the matrix must record actual labels rather than assume `latest` architecture.
- Which minimum Zsh version is worth source-building in scheduled CI, since the product contract names supported ZLE behavior more strongly than a numeric Zsh floor.
- Whether authenticated provider smoke will run only from a maintainer workstation or from a protected self-hosted environment; it remains manual and non-required either way.
- The exact aggregate coverage floor will be chosen after unit and instrumented executable counters are measured repeatedly on both required host platforms.
