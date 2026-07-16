## Context

The Bash adapter currently contains native Readline support for Bash 4.0+ plus a pinned ble.sh integration originally introduced to make Bash 3.2 usable. That second editor stack now spans shell functions, protocol constants, core validation, setup inspection, doctor checks, PTY helpers, downloaded fixtures, CI jobs, active specifications, and documentation. With Bash 4.0 established as the minimum, native Readline already exposes the editable line and cursor required by `intent-sh`, so the third-party backend is no longer part of the desired support contract.

The cleanup is cross-cutting: the embedded adapter, binary validation, diagnostics, test selection, and documentation must agree that supported interactive editors are ZLE and native Bash Readline only. Archived OpenSpec changes remain historical records.

## Goals / Non-Goals

**Goals:**

- Require Bash 4.0 or newer and use native Readline as the only Bash editor backend.
- Remove all ble.sh production code, protocol identifiers/constants, setup and doctor behavior, fixtures, tests, CI suites, and active documentation.
- Reject Bash older than 4.0 and every non-native Bash backend before provider invocation.
- Preserve ZLE/Readline rewrite, regeneration, undo, cancellation, Unicode, safety, key-configuration, and no-auto-execution behavior.
- Remove obsolete files and references outside archived OpenSpec history.

**Non-Goals:**

- Supporting alternate Bash line editors.
- Changing protocol-2 framing, provider routing, command validation, risk rules, or the no-auto-execution guarantee.
- Rewriting archived OpenSpec changes or Git history.
- Refactoring native cancellation architecture beyond deleting code used only by the removed backend.
- Expanding support below Bash 4.0.

## Decisions

### Gate Bash and initialize only native Readline

The embedded Bash adapter will check `BASH_VERSINFO[0] < 4` before installing any binding. Once the gate passes, it will initialize native Readline directly and report `editorBackend=readline`; it will contain no ble.sh detection, version checks, widget bindings, advice hooks, cursor conversion, or detach handling.

The adapter will not attempt to recognize or interoperate with third-party line editors. Bash users must run `intent-sh` in a native Readline session. Silently maintaining another editor-specific branch would recreate the removed support obligation.

### Remove the backend semantically without changing protocol framing

Protocol 2 already carries generic editor-backend and editor-version fields. The fields remain because ZLE and Readline validation still uses them, but the `blesh` value and pinned-version constant will be removed. Core validation will accept only ZLE for Zsh and Readline for Bash 4.0+, rejecting all other backend values before routing to a provider.

This is a breaking semantic support change, not a wire-format change. A stale adapter reporting the removed backend fails closed through existing compatibility handling, so a protocol bump would add churn without improving safety.

### Delete setup and diagnostic branches rather than leave tombstones

Setup inspection will examine only native ZLE/Readline binding conflicts. Its plan model and CLI output will lose pinned-editor fields, alternate-backend conflicts, load-order detection, and third-party removal guidance. Doctor will remove all `shell.blesh.*` checks and backend-specific failure codes; it will report only shell compatibility, the native editor backend, native key conflicts, configuration, and provider readiness.

Keeping deprecated fields or stable check IDs as permanent skips would continue to advertise a backend that no longer exists and would complicate repository-wide audits.

### Delete external fixtures and qualification suites

The Bash 3.2 and ble.sh fixture metadata, installers, cache tests, environment variables, shell harness helpers, PTY cases, Make targets, manifest suites, workflow jobs, cache steps, artifacts, and aggregate dependencies will be removed. Native Bash/Zsh PTY, tmux, SSH, artifact, coverage, scheduled shell-compatibility, and provider suites remain.

Unit tests will retain explicit Bash-below-4 rejection for the native request path and add rejection coverage for unknown or removed editor-backend values. Required CI should qualify supported environments, not download or maintain removed editor stacks.

### Preserve native cancellation behavior

The terminal monitor and process-group cancellation behavior used by native Readline remains. Functions and hooks that exist only to integrate with the removed editor will be deleted. Any broader simplification requires separate native Bash/Zsh cancellation evidence.

### Update active truth while preserving history

Main OpenSpec capabilities, README, supporting documentation, diagnostics, and CI descriptions will describe ZLE and Bash 4.0+ native Readline only. `docs/blesh-compatibility.md` and repository-owned ble.sh fixture files will be deleted. Archived OpenSpec artifacts remain unchanged and are excluded from the active-reference audit.

## Risks / Trade-offs

- **Existing Bash users who load ble.sh lose support** → Document the breaking change and require a native Readline Bash 4.0+ session or Zsh.
- **Stale adapters still report the removed backend** → Keep protocol-2 semantic validation and fail before provider routing; instruct users to open a new native shell and re-evaluate initialization.
- **Deleting editor-specific code accidentally removes shared behavior** → Identify functions by call graph, retain native monitor/cancellation paths, and run the complete native Bash/Zsh PTY suite.
- **Removing external suites reduces coverage count** → Keep focused unit rejection tests and all native lifecycle, Unicode, safety, tmux, SSH, and artifact qualification.
- **Text searches find historical references** → Audit active code/specs/docs while excluding `openspec/changes/archive/**`, this active change's removal rationale, dependency lock versions, and Git history.

## Migration Plan

1. Remove the backend from adapter/core behavior, setup/doctor models, tests, fixtures, CI, active specifications, and documentation in one change.
2. Bash users must use Bash 4.0+ without a third-party line editor in the session where `intent-sh` is initialized; Zsh remains an alternative.
3. After upgrading, open a new native shell or re-evaluate the adapter emitted by the new binary.
4. Rollback consists of reinstalling the prior binary and reloading its matching adapter; there is no persistent data migration.

## Open Questions

None.
