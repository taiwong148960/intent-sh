## Why

The repository has broad PTY, tmux, ble.sh, SSH, and provider-facing tests, but the current GitHub Actions baseline is red and ordinary test runs silently skip the most important opt-in integrations. CI should turn the existing behavioral contracts into explicit, reproducible qualification gates while reserving nondeterministic, credentialed, or hardware-dependent checks for scheduled and manual workflows.

## What Changes

- Restore a trustworthy green baseline by fixing the hermetic ble.sh fixture build and the Linux Bash cancellation regression exposed by hosted runners.
- Split CI into explicit static, unit/race, native PTY, tmux, ble.sh, loopback SSH, artifact, nightly stress, and manual qualification tiers instead of relying on one broad `go test ./...` invocation.
- Make required integration jobs fail when a prerequisite is missing or a mandatory test is skipped, and remove accidental duplicate execution of the tmux suite.
- Run the existing SSH Bash/Zsh conformance suite against an ephemeral loopback OpenSSH target with isolated keys, host state, and cleanup.
- Expand deterministic end-to-end coverage across supported shells, editor modes, locales, shell versions, provider failure paths, process teardown, installation journeys, and native executable smoke tests.
- Add race, shuffle, repeat, fuzz, coverage, module integrity, shell/workflow lint, strict OpenSpec validation, and security checks at tiers appropriate to their cost and stability.
- Produce bounded, sanitized test evidence on failure while keeping prompts, generated commands, credentials, terminal contents, and real-provider state out of untrusted pull-request workflows.
- Keep authenticated provider smoke, external SSH targets, and named GUI-terminal qualification explicitly manual or otherwise isolated from forked pull requests.

## Capabilities

### New Capabilities

- `continuous-integration-qualification`: Defines the tiered CI gates, mandatory integration matrix, hermetic dependency and skip policy, artifact/runtime qualification, stress coverage, evidence handling, and credential boundaries.

### Modified Capabilities

- None. Existing shell, terminal, installation, provider-routing, and command-safety behavior remains authoritative; this change strengthens how those contracts are verified and fixes implementation defects that already violate them.

## Impact

- Affects `.github/workflows`, CI helper scripts, `Makefile` verification targets, Go test organization and fixtures, shell PTY/tmux/SSH/ble.sh harnesses, and test documentation.
- May adjust Bash cancellation and process-group handling to satisfy the existing cross-platform cancellation contract.
- Adds pinned CI-only tools or external test fixtures, an ephemeral loopback OpenSSH setup, sanitized test-report artifacts, and integration-aware coverage collection.
- Does not change the adapter protocol, command syntax, configuration format, provider credentials, supported product platforms, or user-facing runtime behavior except to correct existing compatibility defects.
