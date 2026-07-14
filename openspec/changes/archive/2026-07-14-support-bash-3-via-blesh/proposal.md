## Why

Stock macOS still provides Bash 3.2, but `intent-sh` currently rejects every Bash version below 4.0 because native `bind -x` callbacks cannot access the editable line through `READLINE_LINE` and `READLINE_POINT`. Supporting a pinned ble.sh-backed editor path lets Bash 3.2 users keep the full review-before-execution workflow without weakening buffer ownership or dangerous-command confirmation.

## What Changes

- Add conditional Bash 3.2 support when the pinned compatible ble.sh line editor is loaded and can provide editable-buffer callbacks.
- Keep the native Readline adapter for Bash 4.0 and newer; select the editor backend during adapter initialization and fail closed when neither backend is usable.
- Provide feature parity for full-buffer rewrite, regenerate, undo, cancellation, manual-edit invalidation, and unchanged-command dangerous confirmation through ble.sh-specific bindings.
- Extend setup and diagnostics to report the selected Bash editor backend, ble.sh availability and compatibility, activation order, and actionable fallback guidance.
- Add stock macOS Bash 3.2 PTY coverage, including Emacs/Vi keymaps, Unicode cursor handling, provider cancellation, and first/second Enter behavior.
- Document ble.sh as an optional dependency for Bash 3.2, including its exact tested commit, trust, performance, installation, and removal implications.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `shell-rewrite-workflow`: Replace unconditional rejection of Bash below 4.0 with capability-based selection of a compatible ble.sh editor backend while preserving the complete rewrite and execution-guard contract.
- `installation-diagnostics`: Diagnose and document conditional Bash 3.2 support, ble.sh compatibility and load order, backend selection, key conflicts, and safe fallback behavior.

## Impact

The change affects the embedded Bash adapter, adapter protocol capability reporting, request validation, setup guidance, doctor checks, shell PTY fixtures, CI, and user documentation. It adds a pinned ble.sh commit as an optional third-party runtime dependency for Bash 3.2 sessions but does not vendor it, install it automatically, change the provider protocol, or relax local command validation. The change is sequenced after `build-intent-sh-mvp`, whose `shell-rewrite-workflow` and `installation-diagnostics` capabilities it modifies.
