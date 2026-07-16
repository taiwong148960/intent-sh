## MODIFIED Requirements

### Requirement: Maintain deterministic required qualification gates
The repository SHALL run stable, explicitly named macOS required checks for pull requests, merge-queue candidates, and changes pushed to the protected branch. Required suites MUST execute through dedicated targets, MUST fail when a mandatory local prerequisite or test is unavailable, and MUST NOT treat an unexpected skip as successful qualification. A test case SHALL execute at most once in the required workflow unless a documented shell, terminal, locale, or architecture dimension intentionally repeats it. External SSH and authenticated provider checks MUST remain outside the untrusted required graph.

#### Scenario: Qualify a pull request
- **WHEN** a pull request changes Go code, shell adapters, test harnesses, CI configuration, documentation, or OpenSpec artifacts
- **THEN** every required macOS static, unit, integration, and build gate runs under a stable check name and the change cannot qualify while any gate fails

#### Scenario: Detect an unavailable integration prerequisite
- **WHEN** a required PTY, shell, tmux, or artifact test cannot find its declared prerequisite on the macOS runner
- **THEN** its dedicated job fails with the missing capability instead of reporting a skipped or passing suite

#### Scenario: Partition overlapping suites
- **WHEN** broad package tests and dedicated integration targets would select the same tmux, remote-harness, or provider test
- **THEN** the workflow excludes the overlap or uses an explicit test manifest so each required matrix case runs exactly once

### Requirement: Qualify native Bash and Zsh end to end
Required integration jobs SHALL execute the built `intent-sh` binary with fake Claude and Codex providers inside real pseudo-terminals on macOS. The matrix SHALL exercise supported native Bash and Zsh editors with default and custom chords, CR and LF acceptance, representative `TERM` and explicit `C` and UTF-8 locale values, Emacs and Vi editing where supported, Unicode cursors, terminal resize, regeneration, undo, clarification, routing failures, cancellation, session isolation, and safe, review, and dangerous no-auto-execution behavior. Unicode journeys MUST NOT inherit locale categories from the workflow-launching shell.

#### Scenario: Complete the native lifecycle on macOS
- **WHEN** required CI runs on macOS with its declared Bash and Zsh executables
- **THEN** both shells complete the mandatory PTY lifecycle without accessing user startup files, provider credentials, terminal contents, or host configuration

#### Scenario: Cancel from a native Bash terminal
- **WHEN** macOS Bash receives terminal `Ctrl+C` or the bounded signal-path test interrupts a slow fake provider
- **THEN** the complete provider process tree stops, no fallback starts, the original buffer returns, terminal state is restored, and the shell remains usable

#### Scenario: Convert a Zsh Unicode cursor deterministically
- **WHEN** a Zsh PTY journey places the editor cursor among multibyte and combining characters
- **THEN** the harness supplies a verified UTF-8 locale and the adapter reports the corresponding protocol-2 UTF-8 byte offset independent of the parent process locale

#### Scenario: Exercise editor and locale variants
- **WHEN** the scheduled compatibility matrix selects a supported editor mode, shell version, terminal description, or explicit locale
- **THEN** the same buffer, cursor, confirmation, privacy, and no-auto-execution invariants hold for that declared variant

### Requirement: Qualify tmux hermetically
Required integration jobs SHALL run the isolated tmux lifecycle on macOS. tmux tests MUST use a private socket and empty configuration, MUST qualify supported Bash 4.0+ native Readline and Zsh sessions, and MUST NOT inspect or mutate user state. CI MUST NOT maintain a fixture, cache, manifest suite, or required job for an unsupported Bash generation or alternate Bash editor.

#### Scenario: Reattach an isolated tmux client
- **WHEN** the test client detaches and reattaches to a live Bash or Zsh pane on macOS
- **THEN** editable-buffer and shell-local rewrite state survive only in that pane and the test leaves no private tmux server running

#### Scenario: Exclude removed shell fixtures
- **WHEN** required CI constructs its shell and editor matrices
- **THEN** it selects supported native Bash/Zsh cases only and contains no below-minimum Bash or alternate-editor fixture dependency

### Requirement: Qualify the SSH transport without external credentials
Untrusted required CI SHALL prove that remote qualification is opt-in, preserves provider and marker privacy, validates cleanup paths, and contacts no SSH target. End-to-end SSH and SSH-to-tmux qualification SHALL run only in an explicitly authorized protected/manual environment against a caller-owned target with existing BatchMode authentication and known-host state. The harness MUST verify that the remote host reports Darwin before staging a candidate, MUST use remote Bash 4.0+ or Zsh, MUST create only one bounded disposable remote directory, and MUST remove that directory without modifying SSH configuration, credentials, provider login, or macOS Remote Login settings.

#### Scenario: Run required CI without an SSH target
- **WHEN** the ordinary required test graph evaluates the remote harness
- **THEN** the opt-in guard passes without resolving credentials, contacting a host, staging a binary, or creating remote state

#### Scenario: Reject a target outside the supported platform boundary
- **WHEN** a protected remote target does not report Darwin and a supported architecture
- **THEN** qualification stops before staging or running `intent-sh` and records only a bounded incompatibility result

#### Scenario: Run protected macOS remote conformance
- **WHEN** an authorized maintainer supplies a prepared macOS target with BatchMode authentication, a known host key, supported shells, tmux, and an allocated PTY
- **THEN** remote Bash and Zsh complete rewrite, regenerate, undo, cancellation, review, dangerous confirmation, provider locality, privacy, and no-auto-execution checks with no client-side provider fallback

#### Scenario: Disconnect and clean a protected remote session
- **WHEN** a protected SSH journey completes or loses its client connection
- **THEN** the harness attempts bounded cleanup of only its validated remote temporary directory and reports any retained path without exposing an address, username, key, prompt, command, or terminal content

#### Scenario: Reattach through SSH and tmux
- **WHEN** the first protected SSH client detaches from a live remote tmux shell and a fresh client reconnects
- **THEN** the same remote pane retains its shell-local state while a new pane has independent state and no client-local `intent-sh` state is required

### Requirement: Qualify supported build artifacts
CI SHALL build `intent-sh` for macOS on amd64 and arm64 with reproducible build flags. A macOS runner SHALL execute the artifact native to its architecture through version, adapter initialization, setup, doctor, configuration, and fake-provider PTY smoke journeys. A supported artifact that cannot be executed natively in the current graph SHALL be inspected as Mach-O for the expected CPU architecture, Go build settings, checksum, executable shape, embedded adapters, and bounded version metadata, and its evidence MUST be labeled as inspection rather than native qualification.

#### Scenario: Smoke-test a native macOS artifact
- **WHEN** a runner builds an executable for its own macOS architecture
- **THEN** the produced file runs the declared command and interactive smoke journeys from a disposable home without relying on `go run` or a separately built development binary

#### Scenario: Inspect a cross-built macOS artifact
- **WHEN** CI cannot execute the other supported macOS architecture natively
- **THEN** it verifies successful static construction, Mach-O CPU type, embedded adapters, and bounded build metadata without recording a native execution pass

#### Scenario: Exercise activation and removal guidance
- **WHEN** the native artifact is tested from a disposable installation prefix and home
- **THEN** initialization, setup, custom configuration, downgrade cleanup, and removal guidance complete without modifying a real startup file or system shell

### Requirement: Detect races, order dependencies, fuzz failures, and coverage regressions
Required CI SHALL run the Go race detector on macOS and SHALL enforce a documented coverage floor that includes the executable paths exercised by required macOS integration tests. Scheduled CI SHALL run logged-seed shuffle and repeat stress suites, bounded independent fuzz targets, minimum/current Bash and Zsh versions built or installed on macOS, and available native macOS architectures. A scheduled failure MUST remain visible and actionable rather than being converted into an unconditional retry success.

#### Scenario: Detect a race or coverage regression
- **WHEN** a change introduces a data race or lowers measured macOS coverage below the checked-in policy
- **THEN** the corresponding required gate fails with bounded diagnostic evidence

#### Scenario: Reproduce a shuffled failure
- **WHEN** a shuffled or repeated scheduled run fails
- **THEN** CI records the seed, repetition, macOS version, architecture, and relevant tool versions needed to rerun the same case

#### Scenario: Exercise fuzz targets
- **WHEN** the scheduled fuzz workflow runs on macOS
- **THEN** every registered fuzz target receives a bounded independent budget and any minimizing corpus artifact is retained without containing prompts, credentials, or terminal contents

### Requirement: Isolate trusted and credentialed qualification
Forked and ordinary pull-request workflows MUST run without provider credentials, external SSH credentials, writable repository tokens, or a `pull_request_target` path that executes untrusted checkout code. Authenticated real-provider smoke, caller-supplied macOS remote SSH qualification, and named GUI-terminal qualification SHALL run only through an explicitly authorized manual or protected environment and SHALL remain outside required untrusted pull-request gates.

#### Scenario: Run an untrusted pull request
- **WHEN** CI evaluates code from a fork or otherwise untrusted source
- **THEN** only fake providers and local disposable state are available, permissions remain read-only, and no external authenticated service or SSH target is contacted

#### Scenario: Run authenticated provider smoke
- **WHEN** an authorized maintainer explicitly starts a real-provider qualification in a protected macOS environment
- **THEN** the existing bounded harmless smoke runs without uploading prompts, commands, model output, environment values, tokens, or credential files

#### Scenario: Qualify a protected macOS remote
- **WHEN** an authorized maintainer starts the external SSH workflow
- **THEN** the workflow validates the bounded target input and Darwin remote identity before staging test files and uploads no SSH evidence containing host or credential material

#### Scenario: Qualify a named terminal manually
- **WHEN** a maintainer validates an actual macOS GUI or integrated terminal that hosted CI cannot drive hermetically
- **THEN** the dated qualification record is updated with bounded environment metadata while CI does not claim to have automated that terminal application
