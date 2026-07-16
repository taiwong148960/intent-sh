# Continuous Integration Qualification Specification

## Purpose

Define the repository's deterministic, hermetic macOS CI contract and its trust, evidence, architecture, and reproducibility boundaries.

## Requirements

### Requirement: Maintain deterministic required qualification gates
The repository SHALL run stable, explicitly named macOS required checks for pull requests, merge-queue candidates, and changes pushed to the protected branch. Required suites MUST execute through dedicated targets, MUST fail when a mandatory local prerequisite or test is unavailable, and MUST NOT treat an unexpected skip as successful qualification. A test case SHALL execute at most once unless a documented shell, terminal, locale, or architecture dimension intentionally repeats it. External SSH and authenticated provider checks MUST remain outside the untrusted required graph.

#### Scenario: Qualify a pull request
- **WHEN** a pull request changes source, adapters, tests, CI, documentation, or OpenSpec artifacts
- **THEN** every required macOS static, unit, integration, and build gate runs under a stable check name and the change cannot qualify while any gate fails

#### Scenario: Detect an unavailable prerequisite
- **WHEN** a required PTY, shell, tmux, or artifact test cannot find its declared prerequisite on the macOS runner
- **THEN** its dedicated job fails instead of reporting a skipped or passing suite

#### Scenario: Partition overlapping suites
- **WHEN** broad tests and dedicated integration targets could select the same tmux, remote-harness, or provider test
- **THEN** the workflow excludes the overlap or uses an explicit manifest so each required case runs once

### Requirement: Verify source, specification, and workflow integrity
Required validation SHALL check Go formatting, vet diagnostics, module checksums, a tidy module graph, Bash and Zsh syntax, shell lint, GitHub Actions syntax, strict OpenSpec validity, the first-party macOS platform-scope audit, and both supported Darwin builds. CI-only tools and actions MUST be pinned reproducibly, and a generated or dependency-integrity difference MUST fail without rewriting the checkout.

#### Scenario: Detect repository drift
- **WHEN** formatting, modules, shell syntax, workflow syntax, OpenSpec artifacts, or active platform scope differs from the checked-in contract
- **THEN** a required integrity gate fails and identifies the file or validation command without modifying source

#### Scenario: Validate all OpenSpec artifacts
- **WHEN** CI validates specifications and active changes
- **THEN** strict validation fails on any invalid requirement, scenario, or planning artifact

#### Scenario: Resolve a CI dependency
- **WHEN** a workflow invokes an external action, linter, or downloaded fixture
- **THEN** it resolves from an immutable version or digest whose update is reviewable

### Requirement: Qualify native Bash and Zsh end to end
Required jobs SHALL execute the built binary with fake Claude and Codex providers inside real macOS pseudo-terminals. The matrix SHALL exercise native Bash and Zsh editors with default/custom chords, CR/LF, representative `TERM`, explicit `C` and verified UTF-8 locales, Emacs and Vi where supported, Unicode cursors, resize, regeneration, undo, clarification, routing failures, cancellation, session isolation, and all no-auto-execution risk behavior. Unicode journeys MUST NOT inherit locale categories from the launching shell.

#### Scenario: Complete the native lifecycle on macOS
- **WHEN** required CI runs with its declared Bash and Zsh executables
- **THEN** both shells complete the mandatory lifecycle without accessing startup files, credentials, terminal contents, or host configuration

#### Scenario: Cancel from native Bash
- **WHEN** Bash receives terminal `Ctrl+C` or the bounded signal-path test interrupts a slow fake provider
- **THEN** the process tree stops, fallback does not start, the original buffer returns, terminal state is restored, and the shell remains usable

#### Scenario: Convert a Zsh Unicode cursor deterministically
- **WHEN** a Zsh journey places the editor cursor among multibyte and combining characters
- **THEN** the harness supplies a verified UTF-8 locale and reports the protocol-2 byte offset independently of the parent locale

#### Scenario: Exercise declared variants
- **WHEN** scheduled qualification selects an editor mode, shell version, terminal description, or explicit locale
- **THEN** the same buffer, cursor, confirmation, privacy, and no-auto-execution invariants hold

### Requirement: Qualify tmux hermetically
Required jobs SHALL run the isolated tmux lifecycle on macOS. Tests MUST use a private socket and empty configuration, MUST qualify Bash 4.0+ native Readline and Zsh, and MUST NOT inspect or mutate user state.

#### Scenario: Reattach an isolated client
- **WHEN** a client detaches and reattaches to a live Bash or Zsh pane
- **THEN** editable-buffer and rewrite state survive only in that pane and no private server remains

#### Scenario: Exclude unsupported shell fixtures
- **WHEN** CI constructs shell/editor matrices
- **THEN** it selects supported native Bash/Zsh cases only

### Requirement: Qualify the SSH transport without external credentials
Untrusted required CI SHALL prove that remote qualification is opt-in, preserves marker/provider privacy, validates cleanup paths, and contacts no target. End-to-end SSH and SSH-to-tmux SHALL run only in an authorized protected environment against a caller-owned target with existing BatchMode authentication and known-host state. The harness MUST verify Darwin and arm64/amd64 before staging, use supported remote shells, create one bounded disposable directory, and remove only that directory without changing SSH configuration, credentials, provider login, or macOS Remote Login settings.

#### Scenario: Run required CI without a target
- **WHEN** the required graph evaluates the remote harness
- **THEN** the opt-in guard passes without credential lookup, host contact, staging, or remote state

#### Scenario: Reject a target outside the boundary
- **WHEN** a protected target does not report Darwin and a supported architecture
- **THEN** qualification stops before staging or running the candidate and emits only a bounded incompatibility result

#### Scenario: Run protected remote conformance
- **WHEN** an authorized maintainer supplies a prepared macOS target with supported shells, tmux, PTY, authentication, and known-host state
- **THEN** remote Bash/Zsh complete rewrite, regenerate, undo, cancellation, review, dangerous confirmation, locality, privacy, and no-auto-execution checks

#### Scenario: Disconnect and clean a protected session
- **WHEN** a protected journey completes or loses its connection
- **THEN** cleanup attempts only the validated remote directory and reports no address, username, key, prompt, command, or terminal content

#### Scenario: Reattach through SSH and tmux
- **WHEN** a fresh client reconnects to a live remote tmux shell
- **THEN** the same pane retains its shell-local state while new panes remain independent

### Requirement: Qualify supported build artifacts
CI SHALL reproducibly build macOS amd64 and arm64 artifacts. A runner SHALL execute only its native artifact through version, initialization, setup, doctor, configuration, and fake-provider PTY journeys. The other artifact SHALL be inspected as Mach-O for CPU architecture, Go metadata, checksum, executable shape, embedded adapters, and bounded version metadata, with evidence labeled inspection rather than native qualification.

#### Scenario: Smoke-test a native artifact
- **WHEN** a runner selects the artifact for its own macOS architecture
- **THEN** it completes command and interactive journeys from disposable state without `go run` or another development binary

#### Scenario: Inspect the other macOS artifact
- **WHEN** CI cannot execute the other architecture natively
- **THEN** it verifies Mach-O construction, CPU type, adapters, and metadata without recording a native execution pass

#### Scenario: Exercise activation and removal
- **WHEN** the native artifact runs from a disposable prefix and home
- **THEN** initialization, setup, custom configuration, downgrade cleanup, and removal complete without modifying real user state

### Requirement: Detect races, order dependencies, fuzz failures, and coverage regressions
Required CI SHALL run the race detector on macOS and enforce a documented coverage floor using unit, native PTY, and tmux executable paths. Scheduled CI SHALL run logged-seed stress, bounded independent fuzz targets, minimum/current Bash and Zsh source builds on macOS, and macOS architecture evidence. Scheduled failures MUST remain visible rather than being converted into retry success.

#### Scenario: Detect a race or coverage regression
- **WHEN** a change introduces a race or lowers measured macOS coverage below policy
- **THEN** the corresponding required gate fails with bounded evidence

#### Scenario: Reproduce a shuffled failure
- **WHEN** stress fails
- **THEN** CI records seed, repetition, macOS version, architecture, and relevant tool versions

#### Scenario: Exercise fuzz targets
- **WHEN** scheduled fuzzing runs on macOS
- **THEN** every registered target receives an independent bounded budget and retained metadata contains no prompts, credentials, or terminal contents

### Requirement: Isolate trusted and credentialed qualification
Pull-request workflows MUST run without provider credentials, external SSH credentials, writable tokens, or a `pull_request_target` path executing untrusted checkout code. Authenticated providers, caller-supplied macOS SSH, and named GUI-terminal qualification SHALL run only in an authorized protected/manual environment.

#### Scenario: Run an untrusted pull request
- **WHEN** CI evaluates untrusted source
- **THEN** only fake providers and local disposable state are available, permissions remain read-only, and no external authenticated service or SSH target is contacted

#### Scenario: Run authenticated provider smoke
- **WHEN** a maintainer starts a protected macOS provider run
- **THEN** the bounded harmless smoke uploads no prompt, command, model output, environment value, token, or credential file

#### Scenario: Qualify a protected macOS remote
- **WHEN** a maintainer starts external SSH qualification
- **THEN** the workflow validates bounded input and Darwin identity before staging and uploads no host or credential material

#### Scenario: Qualify a named terminal manually
- **WHEN** a maintainer validates an actual macOS GUI or integrated terminal
- **THEN** the dated record receives bounded metadata without claiming hosted automation drove that application

### Requirement: Preserve bounded and privacy-safe CI evidence
CI SHALL retain machine-readable pass, fail, skip, duration, matrix, and tool-version evidence needed for deterministic tests. Required jobs MUST reject unexpected skips. Uploaded artifacts MUST be bounded and terminal-safe and MUST NOT contain prompts, generated target commands from real services, raw terminal/pane contents, history, arbitrary environment values, SSH credentials, or provider credentials.

#### Scenario: Upload failed fake-provider evidence
- **WHEN** a deterministic required test fails
- **THEN** CI retains only its sanitized result, declared matrix values, and bounded phase for the documented retention period

#### Scenario: Encounter prohibited evidence
- **WHEN** an evidence producer receives prohibited material
- **THEN** it omits that value and fails closed if safe evidence cannot be produced

#### Scenario: Audit skipped tests
- **WHEN** a required command reports a skipped mandatory case
- **THEN** the evidence checker names the case and fails the job
