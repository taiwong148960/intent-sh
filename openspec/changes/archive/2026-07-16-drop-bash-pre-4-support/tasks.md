## 1. Bash Runtime and Protocol Boundary

- [x] 1.1 Enforce `BASH_VERSINFO[0] >= 4` before the Bash adapter installs bindings, publish bounded `unsupported_bash` status, and reject old Bash requests before provider routing.
- [x] 1.2 Remove all ble.sh detection, version/API checks, widgets, advice hooks, conflicts, detach handling, cursor conversion, and initialization branches from the Bash adapter so native Readline is the only Bash path.
- [x] 1.3 Remove the `blesh` backend and pinned-version constants from protocol/core validation, accepting only ZLE for Zsh and Readline for Bash 4.0+ while retaining protocol-2 framing.
- [x] 1.4 Update application and protocol tests to prove supported native backends remain accepted and old Bash, unknown backends, and the removed backend are rejected before provider invocation.
- [x] 1.5 Preserve and verify native Bash provider cancellation, monitor, buffer restoration, and no-auto-execution behavior after editor-specific code removal.

## 2. Setup, Doctor, and CLI Diagnostics

- [x] 2.1 Remove ble.sh fields, backend conflict types, load-order inspection, and pinned dependency metadata from setup plans while preserving bounded read-only native startup-file conflict inspection.
- [x] 2.2 Rewrite Bash setup output to state Bash 4.0+ native Readline only, deleting third-party editor installation, load-order, conflict, trust, and removal guidance.
- [x] 2.3 Remove all `shell.blesh.*` doctor checks, ble.sh failure codes, attachment/load-order/API logic, and alternate-backend readiness paths.
- [x] 2.4 Update doctor, setup, CLI, protocol, and privacy tests for the native-only backend contract and uniform actionable rejection of unsupported shells/backends.

## 3. Shell Test Harness Cleanup

- [x] 3.1 Delete the dedicated ble.sh contract test file and remove ble.sh lifecycle, safety, cancellation, detach, Unicode, setup/doctor, and helper code from the shared PTY workflow harness.
- [x] 3.2 Remove ble.sh test environment variables, executable selectors, fixture helpers, expected test names, and any retained Bash-3-only shell cases.
- [x] 3.3 Verify the remaining native Bash/Zsh harness still covers Emacs, Vi, Unicode, cancellation, conflicts, safety, acceptance, tmux, SSH, and no-auto-execution behavior.

## 4. Fixtures, Make Targets, Manifests, and Workflows

- [x] 4.1 Delete the Bash 3.2 fixture metadata/installer and remove its environment selection, Make target, manifest suites, workflow job, cache, artifacts, and aggregate dependency.
- [x] 4.2 Delete the ble.sh fixture metadata, installer, installer regression script, and all syntax/lint/metadata inventory entries.
- [x] 4.3 Remove ble.sh Make variables and targets, leaving native Bash/Zsh, tmux, SSH, artifact, coverage, and shell-compatibility targets intact.
- [x] 4.4 Remove the `blesh-modern` manifest suite and update repository manifest expectations so no missing or duplicate test is selected.
- [x] 4.5 Remove the ble.sh fixture-installer and qualification jobs, caches, artifacts, and aggregate dependencies from required CI.
- [x] 4.6 Audit scheduled, manual, artifact, SSH, tmux, coverage, and shell-compatibility workflows for hidden Bash-below-4 or ble.sh dependencies.

## 5. Active Specifications and Documentation

- [x] 5.1 Sync the revised delta requirements into main OpenSpec capabilities, removing the ble.sh workflow requirement and describing only native Bash/Zsh qualification.
- [x] 5.2 Rewrite the README compatibility, activation, doctor, workflow, configuration, trust, non-goals, removal, rollback, and CI sections for Bash 4.0+ native Readline only.
- [x] 5.3 Delete `docs/blesh-compatibility.md` and update development, MVP validation, terminal qualification, threat-review, and linked documentation to remove editor-specific fixtures, evidence, support claims, and rationale.
- [x] 5.4 Leave `openspec/changes/archive/**` unchanged and audit all active code, specs, tests, CI, tooling, and docs so old Bash mentions are explicit rejection boundaries only and no ble.sh support surface remains.

## 6. Verification

- [x] 6.1 Run formatting, Go unit tests, adapter embedding tests, Bash/Zsh syntax checks, shell lint, and CI manifest repository tests after the cleanup.
- [x] 6.2 Run the complete native Bash/Zsh PTY suite, confirming rewrite, regenerate, undo, cancellation, Unicode, safety, acceptance, and no-auto-execution behavior.
- [x] 6.3 Run workflow lint and `openspec validate --all --strict`, and verify supported builds and module integrity.
- [x] 6.4 Search outside archived OpenSpec history and this change's removal rationale for `ble.sh`, `blesh`, `bash32`, `BASH32`, Bash-3 compatibility language, removed fixture names, and stale files.
- [x] 6.5 Inspect the final diff for accidental archive, generated dependency, or unrelated changes and confirm the active change is ready for archive.
