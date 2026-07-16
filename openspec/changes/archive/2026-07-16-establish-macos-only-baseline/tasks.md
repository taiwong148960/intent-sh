## 1. Enforce the macOS Runtime Boundary

- [x] 1.1 Add a Darwin build constraint to the shipped command and verify supported builds produce no executable target outside macOS.
- [x] 1.2 Restrict doctor platform readiness to macOS, keep arm64/amd64 architecture checks, and replace named foreign-platform test fixtures with an abstract invalid platform case.
- [x] 1.3 Remove the setup package's multi-OS option and branch, implement only the documented macOS Zsh and Bash startup-file ordering, and update setup/doctor tests.
- [x] 1.4 Replace application, context, prompt, protocol fixture, and service-test platform values with Darwin/macOS data while preserving the remote boolean and privacy boundary.

## 2. Narrow Builds and Artifact Qualification

- [x] 2.1 Reduce `artifactqual.SupportedTargets` to `darwin/arm64` and `darwin/amd64`, remove ELF parsing, and cover Mach-O CPU, metadata, checksum, executable, and adapter-marker validation.
- [x] 2.2 Change `supported-builds`, artifact qualification helpers, and release filenames to build only the two Darwin artifacts with reproducible flags.
- [x] 2.3 Simplify shell-test native-binary validation to Mach-O and update invalid-target tests without naming another operating system or foreign executable format.
- [x] 2.4 Add a checked-in first-party platform-reference audit with explicit exclusions for Git history, archived OpenSpec changes, generated dependency locks, `darwin`, and required macOS Unix/POSIX identifiers; run it from the static gate.

## 3. Convert Test Manifests and Harnesses

- [x] 3.1 Change CI manifest and evidence metadata validation to use Darwin as the only operating-system value and remove one-value Ubuntu/macOS matrices and Linux-specific test phases.
- [x] 3.2 Replace remaining named non-macOS fixtures, comments, assumptions, home paths, and provider outputs in active Go and JSON tests with macOS or platform-neutral data.
- [x] 3.3 Make every Unicode PTY journey select and inject a verified UTF-8 locale, keep `C`-locale cases explicit, and add a regression proving the Zsh cursor test passes even when the parent process has `LC_ALL=C`.
- [x] 3.4 Remove the Linux loopback SSH script, required loopback suite, manifest entry, Make targets, and coverage dependency while preserving no-target, cleanup-path, marker-privacy, and harness-safety unit tests.
- [x] 3.5 Restrict the external SSH harness to a Darwin arm64/amd64 target before staging, retain BatchMode/known-host/disposable-directory cleanup rules, and update remote Bash/Zsh/tmux tests.

## 4. Rebuild Required CI on macOS

- [x] 4.1 Move static, unit, race, native-PTY, tmux, artifact, coverage, and aggregate jobs to explicit macOS runners and remove package-manager and `RUNNER_OS` branches.
- [x] 4.2 Collapse single-value operating-system matrices while retaining shell, editor, locale, terminal, and architecture evidence dimensions and stable required check names.
- [x] 4.3 Build both Darwin artifacts on macOS, execute only the runner-native artifact journey, inspect the other Mach-O artifact, and label native versus inspected evidence distinctly.
- [x] 4.4 Rework executable coverage to include macOS unit, native PTY, and tmux journeys without external SSH; measure the result and update coverage policy only with a checked-in rationale if necessary.
- [x] 4.5 Update the aggregate dependency list after removing loopback SSH and prove every remaining required job must succeed without unexpected skips.
- [x] 4.6 Reduce `install-ci-tools.sh` to pinned Darwin arm64/amd64 archives and verify shellcheck, actionlint, OpenSpec, workflow-policy, and immutable-action checks on macOS.

## 5. Rebuild Scheduled and Protected Qualification

- [x] 5.1 Run scheduled stress and independent fuzz jobs on macOS and preserve bounded seed, repetition, corpus, and privacy-safe failure evidence.
- [x] 5.2 Port pinned Bash/Zsh source compatibility installation to macOS prerequisites and run the minimum/current editor lifecycle without a foreign-host cache branch.
- [x] 5.3 Remove distribution, native non-macOS architecture, and foreign-capacity jobs; replace any remaining capacity record with macOS architecture evidence only.
- [x] 5.4 Run provider capability probes on macOS, select the official Darwin package matching the runner architecture, and leave generated upstream lock metadata integrity-controlled rather than manually pruning it.
- [x] 5.5 Run module-integrity and vulnerability checks on macOS with portable checksum/evidence commands and unchanged read-only permissions.
- [x] 5.6 Update protected manual provider and external-SSH workflows to require an approved macOS environment and Darwin remote identity without exposing credentials or host metadata.

## 6. Rewrite the Current macOS-Only Product Surface

- [x] 6.1 Rewrite README compatibility, installation, non-goals, development, build, tmux, SSH, and qualification sections so every current support statement is macOS-only and native-versus-inspected evidence is explicit.
- [x] 6.2 Rewrite the development and MVP validation guides for the macOS Make targets, CI graph, shell prerequisites, artifact set, coverage boundary, and protected/manual SSH path.
- [x] 6.3 Replace the terminal qualification guide and current result matrix with macOS-only categories and evidence, removing non-macOS rows and claims without inventing a PASS for an unrun environment.
- [x] 6.4 Update threat-review and provider-compatibility wording where platform scope or CI boundaries changed while preserving all existing security and credential-handling guarantees.
- [x] 6.5 Update active OpenSpec purpose text and cross-references that delta requirement syntax cannot express, migrate links to `macos-terminal-compatibility`, and leave archived changes and Git history untouched.

## 7. Verify and Record the Baseline

- [x] 7.1 Run formatting, vet, module, shell syntax/lint, workflow lint, strict OpenSpec validation, and the first-party platform-reference audit on macOS.
- [x] 7.2 Run all unit and race tests with a clean macOS environment and explicitly rerun the Unicode cursor regression under both inherited `LC_ALL=C` and the harness-selected UTF-8 locale.
- [x] 7.3 Run strict native Bash/Zsh PTY and private-socket tmux qualification from disposable homes, verify no mandatory skip, and retain only sanitized evidence.
- [x] 7.4 Build and inspect both Darwin artifacts, execute the native artifact install/setup/doctor/rewrite/removal journey, and confirm the other architecture is recorded only as inspected unless a native runner executes it.
- [x] 7.5 Validate required, scheduled, and protected workflow policy plus aggregate-job dependencies, and document any protected macOS SSH or named-terminal journey as NOT RUN until dated evidence exists.
- [x] 7.6 Run the final scoped repository audit and confirm all remaining platform terms are either macOS/Darwin identifiers, required Unix/POSIX implementation names, generated dependency metadata, or preserved historical records.
