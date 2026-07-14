## 1. Compatibility Contract and Feasibility

- [x] 1.1 Confirm `build-intent-sh-mvp` has been synced or archived so the modified base capabilities exist before this change is archived.
- [x] 1.2 Pin an official ble.sh release or commit and checksum, record its version and required edit/binding/widget APIs, and add a test-only path override without adding a production download.
- [x] 1.3 Build a disposable PTY compatibility probe for stock macOS Bash 3.2 that proves ble.sh can read and replace the complete buffer and cursor in Emacs and Vi insert modes, and narrow away Bash 3.0/3.1 when reproducible coverage is unavailable.
- [x] 1.4 Prove safe/review one-Enter acceptance, dangerous first-Enter blocking and second-unchanged-Enter delegation, edit-to-disarm, cancellation, detach/reattach, and conflict behavior through the selected ble.sh API.
- [x] 1.5 Record whether guarded acceptance uses documented or pinned internal ble.sh APIs, narrow the supported Bash/ble.sh range if the probe requires it, and keep the proposal, design, and specs consistent before enabling compatibility.

## 2. Adapter Protocol and Cursor Contract

- [x] 2.1 Introduce adapter protocol version 2 with bounded `editorBackend` and `editorVersion` fields and update NUL framing, JSON fixtures, round-trip, malformed-field-count, and version-mismatch tests.
- [x] 2.2 Add typed `zle`, `readline`, and `blesh` backend values and validate coherent shell, shell-version, backend, and editor-version combinations before provider invocation.
- [x] 2.3 Define protocol cursor positions as UTF-8 byte offsets, add conversion and boundary-validation helpers, and cover ASCII, Chinese, combining-character, and invalid-boundary cases.
- [x] 2.4 Update the Zsh and native Bash adapters, embedded assets, CLI fixtures, and fake adapter callers to emit protocol 2 while preserving protocol-1 fail-closed behavior.

## 3. Bash Editor Backend Architecture

- [x] 3.1 Refactor the Bash adapter into a shared rewrite/state machine plus editor-specific detection, binding, cursor, status, and acceptance functions, with the native Readline PTY suite unchanged.
- [x] 3.2 Implement capability-based selection that prefers compatible attached ble.sh, otherwise selects native Readline on Bash 4.0+, and installs no bindings when neither backend is usable.
- [x] 3.3 Add bounded exported adapter-status markers for protocol, backend, editor version, readiness, and stable initialization failures without exporting buffer content or binding commands.
- [x] 3.4 Implement ble.sh `Alt+G` rewrite/regenerate and `Alt+U` undo bindings using its edit-command API and the shared NUL-framed provider flow.
- [x] 3.5 Implement a namespaced ble.sh guarded-accept integration that preserves normal acceptance, blocks the first unchanged dangerous Enter, delegates the second, and disarms on every specified state transition.
- [x] 3.6 Preserve editor-native cursors for failure and undo while converting ble.sh logical cursor positions to protocol byte offsets and placing successful replacements at the editor-native end.
- [x] 3.7 Recheck ble.sh attachment and required APIs on every interactive action, preserve the current buffer after detach or incompatibility, and require explicit reinitialization instead of falling back.
- [x] 3.8 Verify synchronous provider execution, `Ctrl+C` process-tree cancellation, signal/trap restoration, and terminal redraw under ble.sh without invoking fallback after cancellation.

## 4. Setup and Diagnostics

- [x] 4.1 Extend Bash setup output and startup-file inspection with the optional ble.sh prerequisite, official installation reference, required load order, native alternatives, common `ble-bind` conflicts, and independent removal guidance.
- [x] 4.2 Extend doctor with stable editor-backend, ble.sh version, API, attachment, load-order, and backend-specific key-conflict checks derived from bounded adapter markers and read-only setup inspection.
- [x] 4.3 Change Bash compatibility reporting so ready Bash 3.x plus ble.sh passes, Bash 3.x without a usable backend fails actionably, and native Readline reported from Bash 3.x is rejected.
- [x] 4.4 Add adversarial tests proving adapter status and doctor output cannot expose intent text, generated commands, binding bodies, credentials, or unbounded environment values.
- [x] 4.5 Verify the provider environment allowlist excludes ble.sh and adapter diagnostic variables and that backend metadata is not added to the model request.

## 5. PTY and Integration Coverage

- [x] 5.1 Add a reproducible ble.sh test harness that verifies the pinned checksum, accepts an explicit local artifact path, caches CI setup, and never runs during production setup or ordinary unit tests.
- [x] 5.2 Run the full fake-provider workflow on stock macOS Bash 3.2 with ble.sh: initial rewrite, mixed input, regenerate, undo, manual editing, clarification, malformed response, timeout, cancellation, stale ID, review warning, and dangerous confirmation.
- [x] 5.3 Narrow the normative minimum to Bash 3.2 because reproducible Bash 3.0 and 3.1 PTY safety coverage is not practical.
- [x] 5.4 Run modern Bash PTY coverage both with native Readline and with ble.sh active, including Emacs and Vi insert keymaps plus supported `C-m`, `C-j`, and Return representations.
- [x] 5.5 Add Unicode PTY scenarios that verify protocol byte offsets and exact local cursor restoration around multibyte and combining characters in ZLE, native Readline, and ble.sh.
- [x] 5.6 Add negative PTY scenarios for missing, unsupported, detached, wrongly ordered, or API-incomplete ble.sh and assert that no partial binding, provider call, replacement, or automatic execution occurs.

## 6. Documentation and CI

- [x] 6.1 Update the README compatibility matrix and Bash setup guide with backend selection, pinned/tested ble.sh versions, activation order, doctor output, harmless first use, alternatives, and removal.
- [x] 6.2 Update privacy and threat documentation with ble.sh's interactive-shell trust boundary, Bash 3 performance limitations, prohibited capture fallbacks, protocol metadata, and unchanged no-auto-execution guarantee.
- [x] 6.3 Add pinned ble.sh jobs to macOS and Linux CI, including stock macOS Bash 3.2, modern Bash native/ble.sh modes, checksum verification, and artifact caching.
- [x] 6.4 Document protocol-2 upgrade and rollback behavior so users reinitialize existing shells and understand that mixed adapter/binary versions fail closed.

## 7. Final Verification

- [x] 7.1 Run formatting, Bash/Zsh syntax checks, `go vet ./...`, and `go test ./...`, and resolve every regression without weakening existing safety assertions.
- [x] 7.2 Execute end-to-end fake-Claude and fake-Codex flows through ZLE, native Readline, and Bash 3.2 ble.sh backends and prove generated targets never run before the required user acceptance.
- [x] 7.3 Run the documented source-build, setup, doctor, harmless rewrite, undo, cancellation, and removal journey on stock macOS Bash 3.2 with the pinned ble.sh version.
- [x] 7.4 Validate all OpenSpec artifacts and confirm the final supported Bash and ble.sh ranges, protocol version, diagnostics, README, and CI matrix agree before marking the change complete.
