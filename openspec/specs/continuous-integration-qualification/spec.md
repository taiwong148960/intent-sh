## Purpose

Define the repository's deterministic, hermetic, cross-platform CI qualification contract and its trust, evidence, and reproducibility boundaries.

## Requirements

### Requirement: Maintain deterministic required qualification gates
The repository SHALL run stable, explicitly named required checks for pull requests, merge-queue candidates, and changes pushed to the protected branch. Required suites MUST execute through dedicated targets, MUST fail when a mandatory prerequisite or test is unavailable, and MUST NOT treat an unexpected skip as successful qualification. A test case SHALL execute at most once in the required workflow unless a documented matrix dimension intentionally repeats it.

#### Scenario: Qualify a pull request
- **WHEN** a pull request changes Go code, shell adapters, test harnesses, CI configuration, documentation, or OpenSpec artifacts
- **THEN** every required static, unit, integration, and build gate runs under a stable check name and the change cannot qualify while any gate fails

#### Scenario: Detect an unavailable integration prerequisite
- **WHEN** a required PTY, shell, tmux, SSH, or artifact test cannot find its declared prerequisite
- **THEN** its dedicated job fails with the missing capability instead of reporting a skipped or passing suite

#### Scenario: Partition overlapping suites
- **WHEN** broad package tests and dedicated integration targets would select the same tmux, SSH, or provider test
- **THEN** the workflow excludes the overlap or uses an explicit test manifest so each required matrix case runs exactly once

### Requirement: Verify source, specification, and workflow integrity
Required validation SHALL check Go formatting, vet diagnostics, module checksums, a tidy module graph, Bash and Zsh syntax, shell lint, GitHub Actions syntax, strict OpenSpec validity, and supported target builds before qualification is considered complete. CI-only tools and third-party actions MUST be pinned reproducibly, and a generated or dependency-integrity difference MUST fail without rewriting the checkout.

#### Scenario: Detect repository drift
- **WHEN** source formatting, `go.mod`, `go.sum`, shell syntax, workflow syntax, or an OpenSpec artifact differs from its reproducible checked-in form
- **THEN** a required integrity gate fails and reports the file or validation command without modifying the working tree

#### Scenario: Validate all OpenSpec artifacts
- **WHEN** CI validates the repository's specifications and active changes
- **THEN** it runs strict validation over all applicable items and fails if any requirement, scenario, or planning artifact is invalid

#### Scenario: Resolve a CI dependency
- **WHEN** a workflow invokes an external action, linter, or downloaded test fixture
- **THEN** the invocation resolves from an immutable version or digest whose update is reviewable in the repository

### Requirement: Qualify native Bash and Zsh end to end
Required integration jobs SHALL execute the built `intent-sh` binary with fake Claude and Codex providers inside real pseudo-terminals on macOS and Linux. The matrix SHALL exercise supported native Bash and Zsh editors with default and custom chords, CR and LF acceptance, representative `TERM` and locale values, Emacs and Vi editing where supported, Unicode cursors, terminal resize, regeneration, undo, clarification, routing failures, cancellation, session isolation, and safe, review, and dangerous no-auto-execution behavior.

#### Scenario: Complete the native lifecycle on each operating system
- **WHEN** required CI runs on macOS or Linux with its declared Bash and Zsh executables
- **THEN** both shells complete the mandatory PTY lifecycle without accessing user startup files, provider credentials, terminal contents, or host configuration

#### Scenario: Cancel from a Linux Bash terminal
- **WHEN** Linux Bash receives terminal `Ctrl+C` or the bounded signal-path test interrupts a slow fake provider
- **THEN** the complete provider process tree stops, no fallback starts, the original buffer returns, terminal state is restored, and the shell remains usable

#### Scenario: Exercise editor and locale variants
- **WHEN** the scheduled compatibility matrix selects a supported editor mode, shell version, terminal description, or locale
- **THEN** the same buffer, cursor, confirmation, privacy, and no-auto-execution invariants hold for that declared variant

### Requirement: Qualify tmux hermetically
Required integration jobs SHALL run the isolated tmux lifecycle on macOS and Linux. tmux tests MUST use a private socket and empty configuration, MUST qualify supported Bash 4.0+ native Readline and Zsh sessions, and MUST NOT inspect or mutate user state. CI MUST NOT maintain a fixture, cache, manifest suite, or required job for an unsupported Bash generation or alternate Bash editor.

#### Scenario: Reattach an isolated tmux client
- **WHEN** the test client detaches and reattaches to a live Bash or Zsh pane
- **THEN** editable-buffer and shell-local rewrite state survive only in that pane and the test leaves no private tmux server running

#### Scenario: Exclude removed shell fixtures
- **WHEN** required CI constructs its shell and editor matrices
- **THEN** it selects supported native Bash/Zsh cases only and contains no below-minimum Bash or alternate-editor fixture dependency

### Requirement: Qualify the SSH transport without external credentials
A required Linux job SHALL create an ephemeral loopback OpenSSH server and run the remote Bash and Zsh conformance harness through the real SSH client and allocated PTY. The target, host keys, client key, known-host data, home, port, staged binaries, fake providers, and remote temporary directory MUST be isolated to the job and removed afterward. The job SHALL cover direct remote behavior and SSH-to-tmux detach and reattach without contacting an external host.

#### Scenario: Run loopback remote conformance
- **WHEN** the loopback SSH target is ready
- **THEN** remote Bash and Zsh complete rewrite, regenerate, undo, cancellation, review, dangerous confirmation, provider locality, privacy, and no-auto-execution checks with no client-side provider fallback

#### Scenario: Disconnect a direct SSH session
- **WHEN** the client disconnects while a non-tmux remote fake provider is active
- **THEN** the bounded cleanup assertion detects no surviving staged provider process or target-command side effect and removes the remote test directory

#### Scenario: Reattach through SSH and tmux
- **WHEN** the first SSH client detaches from a live remote tmux shell and a fresh client reconnects
- **THEN** the same remote pane retains its shell-local state while a new pane has independent state and no client-local `intent-sh` state is required

### Requirement: Qualify supported build artifacts
CI SHALL build `intent-sh` for macOS and Linux on amd64 and arm64 with reproducible build flags. Each runner SHALL execute the artifact native to that runner through version, adapter initialization, setup, doctor, configuration, and fake-provider PTY smoke journeys; non-native artifacts SHALL at least be inspected for the expected target format and architecture. Scheduled CI SHALL execute on additional native architectures when an appropriate runner is available.

#### Scenario: Smoke-test a native artifact
- **WHEN** a runner builds an executable for its own operating system and architecture
- **THEN** the produced file runs the declared command and interactive smoke journeys from a disposable home without relying on `go run` or a separately built development binary

#### Scenario: Inspect a cross-built artifact
- **WHEN** CI cannot execute a supported cross-compiled target natively
- **THEN** it verifies successful static construction, expected file format and architecture, embedded adapters, and bounded version metadata

#### Scenario: Exercise activation and removal guidance
- **WHEN** the native artifact is tested from a disposable installation prefix and home
- **THEN** initialization, setup, custom configuration, downgrade cleanup, and removal guidance complete without modifying a real startup file or system shell

### Requirement: Detect races, order dependencies, fuzz failures, and coverage regressions
Required CI SHALL run the Go race detector on the supported host platforms and SHALL enforce a documented coverage floor that includes the executable paths exercised by integration tests. Scheduled CI SHALL run logged-seed shuffle and repeat stress suites, bounded fuzz targets, minimum/current shell versions, representative Linux distributions, and available native architectures. A scheduled failure MUST remain visible and actionable rather than being converted into an unconditional retry success.

#### Scenario: Detect a race or coverage regression
- **WHEN** a change introduces a data race or lowers measured coverage below the checked-in policy
- **THEN** the corresponding required gate fails with bounded diagnostic evidence

#### Scenario: Reproduce a shuffled failure
- **WHEN** a shuffled or repeated scheduled run fails
- **THEN** CI records the seed, repetition, platform, architecture, and relevant tool versions needed to rerun the same case

#### Scenario: Exercise fuzz targets
- **WHEN** the scheduled fuzz workflow runs
- **THEN** every registered fuzz target receives a bounded independent budget and any minimizing corpus artifact is retained without containing prompts, credentials, or terminal contents

### Requirement: Isolate trusted and credentialed qualification
Forked and ordinary pull-request workflows MUST run without provider credentials, external SSH credentials, writable repository tokens, or a `pull_request_target` path that executes untrusted checkout code. Authenticated real-provider smoke, user-supplied remote SSH qualification, and named GUI-terminal qualification SHALL run only through an explicitly authorized manual or protected environment and SHALL remain outside required untrusted pull-request gates.

#### Scenario: Run an untrusted pull request
- **WHEN** CI evaluates code from a fork or otherwise untrusted source
- **THEN** only fake providers and ephemeral local credentials are available, permissions remain read-only, and no external authenticated service is contacted

#### Scenario: Run authenticated provider smoke
- **WHEN** an authorized maintainer explicitly starts a real-provider qualification in a protected environment
- **THEN** the existing bounded harmless smoke runs without uploading prompts, commands, model output, environment values, tokens, or credential files

#### Scenario: Qualify a named terminal manually
- **WHEN** a maintainer validates an actual GUI or integrated terminal that hosted CI cannot drive hermetically
- **THEN** the dated qualification record is updated with bounded environment metadata while CI does not claim to have automated that terminal application

### Requirement: Preserve bounded and privacy-safe CI evidence
CI SHALL retain machine-readable pass, fail, skip, duration, matrix, and tool-version evidence needed to diagnose deterministic fake-provider tests. Required jobs MUST reject unexpected skips. Uploaded logs and artifacts MUST be bounded and terminal-safe and MUST NOT contain provider prompts, generated target commands from real services, raw terminal or pane contents, shell history, arbitrary environment values, SSH credentials, or provider credentials.

#### Scenario: Upload evidence for a failed fake-provider test
- **WHEN** a deterministic required test fails
- **THEN** CI retains its sanitized structured test result, declared matrix values, and bounded failure message for a documented retention period

#### Scenario: Encounter prohibited evidence
- **WHEN** an evidence producer receives a prompt, credential marker, unbounded PTY stream, or other prohibited value
- **THEN** it redacts or omits that value and fails closed if safe evidence cannot be produced

#### Scenario: Audit skipped tests
- **WHEN** a required test command finishes with one or more skipped mandatory cases
- **THEN** the evidence checker names those cases and fails the job even if the Go test process exited successfully
