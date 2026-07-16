## Context

The current repository treats macOS and Linux as co-equal supported platforms. That choice is reflected in user documentation, five baseline capabilities, `runtime.GOOS` branches, Bash startup-file selection, Mach-O/ELF artifact inspection, test fixtures, manifest metadata, Linux-only SSH infrastructure, and required and scheduled GitHub Actions jobs. The maintainer's reproducible environment is macOS on Apple silicon, and the existing macOS test harness also inherits ambient locale state in at least one Unicode cursor journey.

This change is a support-contract migration, not a new shell workflow. The protocol-2 adapters, provider boundary, safety pipeline, Zsh support, Bash 4.0+ native Readline support, tmux behavior, and review-before-execution model remain intact. Go and GitHub Actions use `darwin` as the macOS platform identifier, and macOS terminal code legitimately imports `golang.org/x/sys/unix`; those identifiers do not imply another supported operating system. Official provider package locks can also contain upstream optional packages for other platforms even when the repository executes only the macOS variant.

## Goals / Non-Goals

**Goals:**

- Make every current first-party product, build, runtime, test, CI, documentation, and baseline-spec support claim macOS-only.
- Produce and inspect only Mach-O artifacts for macOS `arm64` and `amd64`, with evidence distinguishing native execution from cross-build inspection.
- Run required and scheduled automation on macOS and remove Linux-only infrastructure and matrix dimensions.
- Preserve local PTY, Zsh, Bash 4.0+, tmux, and remote-shell semantics within the supported macOS boundary.
- Make integration-test locale selection explicit so macOS results do not depend on the invoking shell's locale.
- Leave the repository with a documented, reproducible audit that catches new first-party non-macOS assumptions.

**Non-Goals:**

- Rewriting Git commits, force-pushing a replacement root, deleting merged pull-request references, or rewriting archived OpenSpec records.
- Manually pruning generated third-party lockfiles solely because upstream dependency metadata names other platforms.
- Removing Go's `darwin` identifier, macOS/POSIX terminology required to explain implementation behavior, or the `golang.org/x/sys/unix` import path.
- Changing the adapter protocol, provider result schema, risk classifier, provider authentication model, supported shells, or default key chords.
- Automatically enabling macOS Remote Login, modifying system SSH configuration, or contacting an external SSH target from untrusted CI.
- Claiming a named terminal, remote environment, or architecture was natively qualified when it received only generic PTY coverage or static artifact inspection.

## Decisions

### 1. Enforce a Darwin-only executable and simplify OS-dependent behavior

The shipped command will be buildable as a supported executable only for Go's `darwin` target. Supported release targets remain `darwin/arm64` and `darwin/amd64`; no unsupported-platform stub executable will be added. Doctor will accept only the macOS platform boundary, setup will use the macOS Zsh and Bash startup-file rules, and test seams that previously modeled multiple operating systems will use macOS or an unnamed invalid value.

This is preferred to retaining portable branches with documentation-only disclaimers because dead branches would continue to require review and invite accidental support claims. It is also preferred to hard-coding user-facing architecture success: native runtime architecture remains detected normally, while artifact qualification has an explicit two-target allowlist.

### 2. Keep both macOS architectures but label evidence precisely

The build contract will retain Apple silicon and Intel macOS artifacts. A runner executes all install, doctor, adapter, PTY, and removal journeys only for its native architecture. The other Mach-O artifact may be cross-built and inspected for CPU type, Go build settings, checksum, executable shape, and embedded protocol markers, but that result is recorded as inspection rather than native qualification.

This preserves useful macOS distribution breadth without pretending that cross-compilation proves runtime behavior. Workflow runner labels may be updated as hosted macOS capacity changes, but the evidence schema must always record the actual architecture.

### 3. Collapse CI onto macOS rather than retaining an OS matrix of size one

Required jobs will use macOS runners directly for static validation, unit and race tests, native PTYs, private tmux, artifact construction/inspection/native smoke, coverage, and the aggregate gate. Scheduled stress, fuzzing, pinned-shell compatibility, provider capability, and vulnerability jobs will likewise run on macOS. Linux distribution, Linux arm64, Linux cancellation-repeat, Linux capacity-boundary, package-manager, checksum-tool, and tool-download branches will be removed.

The test manifest and evidence validator will use `darwin` as the only operating-system matrix value. Where a job previously branched on `RUNNER_OS`, it will select macOS tools and Darwin provider binaries directly. Generated provider lock metadata remains integrity-controlled; the workflow chooses only the artifact matching the macOS runner architecture.

### 4. Move SSH from Linux-required automation to protected macOS qualification

The Linux loopback daemon fixture depends on `/proc`, Linux account-management tools, Linux package installation, and privileged cleanup, so it will be removed instead of ported by emulation. Required CI will retain unit tests for remote-context privacy, harness path validation, and the explicit no-target skip, but it will not claim end-to-end SSH qualification.

The protected manual SSH workflow remains available only for a caller-owned target that reports macOS before staging or running the candidate. Its existing BatchMode, known-host, bounded-target, disposable-directory, fake-provider, and cleanup rules remain. Documentation will describe SSH as qualified only when a dated macOS-to-macOS run exists. This is preferred to changing system Remote Login settings on hosted or maintainer machines.

### 5. Replace the broad terminal capability rather than leaving a misleading name

The `macos-terminal-compatibility` capability will receive the PTY, editor-state, tmux, key-probe, remote-locality, and evidence requirements narrowed to macOS. The corresponding requirements in `unix-terminal-compatibility` will be removed with migration pointers. Main-spec purpose text and documentation links will be updated when the deltas are synchronized so the active specification set has no broad platform claim.

Other affected capabilities receive full modified requirement blocks only where normative behavior changes. `command-safety` is unchanged because its validation and acceptance contract is platform-independent within the now-supported environment.

### 6. Audit authored support surfaces with explicit exclusions

Implementation verification will scan first-party active code, tests, docs, specs, CI configuration, and helper scripts for named unsupported systems, their package managers, foreign artifact formats, and broad support phrases. Historical Git objects, archived OpenSpec changes, generated dependency locks, `darwin`, and required macOS Unix/POSIX implementation identifiers are outside that lexical gate. Negative platform tests will use an abstract invalid value rather than naming another system.

This scoped audit is preferred to demanding zero matches across the repository, which would encourage corrupting lockfiles and obscuring legitimate macOS implementation details.

### 7. Pin locale inside tests that assert Unicode editor units

Every integration journey that relies on character-aware ZLE behavior will set a verified UTF-8 locale explicitly. Tests for the `C` locale will continue to set it explicitly. No Unicode test may inherit `LANG`, `LC_ALL`, or `LC_CTYPE` from the parent shell. This makes the observed Zsh cursor unit deterministic and preserves protocol-2's UTF-8 byte-offset conversion contract.

## Risks / Trade-offs

- [macOS runner cost or limited concurrency increases CI latency] → Collapse one-value matrices, keep expensive repetition scheduled, and preserve stable aggregate checks.
- [Intel macOS receives only cross-build inspection when no native runner is available] → Label the evidence as inspected, never qualified, and require a native run before publishing a native-runtime claim.
- [Removing required loopback SSH reduces deterministic remote coverage] → Preserve privacy and harness-safety unit coverage and require protected macOS target evidence for any SSH qualification claim.
- [Coverage falls when Linux-only SSH execution leaves the required graph] → Measure the macOS executable suite first and update exclusions or thresholds only with a reviewed explanation; never silently lower the gate to make it pass.
- [First-party platform references survive in an overlooked fixture or document] → Add a bounded repository audit with reviewed exclusions and run it in the static gate.
- [Generated locks continue to contain other platform package names] → Document why they are dependency metadata, validate their integrity, and ensure workflows execute only Darwin variants.
- [A build tag makes developer commands fail on unsupported hosts] → Treat that failure as intentional enforcement of the published support boundary and document macOS as a prerequisite.

## Migration Plan

1. Establish the new and modified OpenSpec requirements, including removal mappings from the old terminal capability.
2. Simplify runtime setup, diagnostics, context fixtures, artifacts, and build targets to Darwin-only behavior.
3. Convert test harnesses and manifests to macOS, make Unicode locale setup deterministic, and remove Linux-only SSH infrastructure.
4. Replace required and scheduled workflows with the macOS graph and verify sanitized evidence, architecture labeling, and coverage.
5. Rewrite current user/developer documentation and qualification records, then run the scoped platform-reference audit and the full macOS qualification gate.

Rollback is a normal source revert before the change is released. Reintroducing another supported operating system after release requires a new proposal, new native qualification evidence, and explicit restoration of its build, runtime, CI, and documentation contract; it is not achieved by removing a build constraint alone.

## Open Questions

- Which hosted macOS runner labels provide native `arm64` and `amd64` execution at implementation time? The workflow will use available explicit labels and will record cross-built-only evidence for any architecture without a native runner.
