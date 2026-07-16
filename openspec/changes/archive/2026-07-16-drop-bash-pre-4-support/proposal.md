## Why

The repository still carries a second Bash editor stack through ble.sh even though Bash 4.0 or newer provides the native Readline buffer API required by `intent-sh`. Keeping ble.sh and Bash-3 compatibility machinery duplicates runtime, diagnostic, test, CI, fixture, and documentation maintenance for paths the project no longer wants to support.

## What Changes

- **BREAKING** Require Bash 4.0 or newer and support Bash only through native Readline; Zsh continues to use ZLE.
- **BREAKING** Remove the `blesh` editor backend, its pinned version and fixture, capability checks, bindings, lifecycle hooks, diagnostics, setup guidance, tests, and CI qualification.
- Reject Bash older than 4.0 before installing bindings or invoking a provider, and reject any non-native editor backend reported for Bash before provider routing.
- Remove Bash-3-specific fixture metadata, installers, PTY tests, Make targets, CI manifest suites, workflow jobs, cache entries, and aggregate dependencies.
- Remove ble.sh-specific files and references across active code, tooling, OpenSpec specifications, and user/developer/security documentation.
- Leave archived OpenSpec changes unchanged as historical records while ensuring every active support surface describes only ZLE and Bash 4.0+ native Readline.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `shell-rewrite-workflow`: Require Bash 4.0+ native Readline, remove ble.sh backend selection and workflow behavior, and retain ZLE/Readline safety semantics.
- `installation-diagnostics`: Remove ble.sh setup, inspection, diagnostics, protocol acceptance, documentation, and removal guidance while uniformly rejecting unsupported Bash versions and backends.
- `continuous-integration-qualification`: Remove all Bash-3 and ble.sh fixtures and qualification suites while retaining required native Bash/Zsh coverage.

## Impact

The change affects the embedded Bash adapter, protocol/backend constants, request validation, setup inspection and output, doctor checks, shell and application tests, fixture scripts, Make targets, CI manifests and workflows, active OpenSpec specifications, and user/developer/security documentation. Adapter protocol framing remains at version 2, but `editorBackend=blesh` becomes unsupported. Existing Bash users must use Bash 4.0+ native Readline or switch to Zsh; users who independently load ble.sh must not use that editor session with `intent-sh`.
