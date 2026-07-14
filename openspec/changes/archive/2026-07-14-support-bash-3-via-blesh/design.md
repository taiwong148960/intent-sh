## Context

The MVP Bash adapter is built around Bash 4.0's `bind -x` contract: the shell exposes the active line and cursor as `READLINE_LINE` and `READLINE_POINT`, and reflects assignments back into Readline. Stock macOS Bash 3.2 runs `bind -x` callbacks but does not provide that contract, so the current shell loader, core request validation, doctor checks, tests, and documentation reject every Bash major version below 4.

ble.sh replaces the interactive Bash line editor and provides an edit-command binding that emulates `READLINE_LINE` and `READLINE_POINT` on Bash 3.x. A deterministic PTY probe on macOS Bash 3.2.57 confirmed that pinned upstream commit `d69e4d549a1881a37300fe6b4a05478bd9157dfc` (`BLE_VERSION=0.4.0-nightly+d69e4d5`) reads and replaces the complete buffer and logical cursor in Emacs and Vi insert modes. The older `0.4.0-devel3` release was rejected because its input loop stopped responding after attachment on the current stock-macOS runner. Bash 3.0 and 3.1 are therefore outside the advertised range because reproducible safety coverage is not practical.

This is a follow-up to `build-intent-sh-mvp`. Its delta specifications modify capabilities introduced by that change, so the MVP change must be synced or archived before this change is archived. The existing security boundary remains unchanged: generated text is data, the adapter never evaluates it, and only the user's shell may accept it after the required review behavior.

## Goals / Non-Goals

**Goals:**

- Support Bash 3.2 when the exact tested ble.sh commit is attached, including stock macOS Bash 3.2.57.
- Preserve full-buffer rewrite, regeneration from the original, undo, cancellation, manual-edit invalidation, stale-response rejection, and risk behavior across Bash editor backends.
- Preserve first-Enter blocking and second-unchanged-Enter delegation for dangerous generated commands.
- Select and report the active editor backend explicitly and fail before installing bindings when its capabilities are insufficient.
- Keep ble.sh optional, user-managed, and outside the provider subprocess environment.
- Make compatibility reproducible through a pinned ble.sh test matrix and stock Bash 3.2 PTY tests.

**Non-Goals:**

- Supporting Bash older than 3.2, Bash 3.0 or 3.1, or plain Bash 3.2 without the tested editable-buffer provider.
- Bundling, downloading, installing, updating, configuring, or removing ble.sh on the user's behalf.
- Reimplementing a Bash line editor, shipping a native Readline extension, wrapping the terminal, or changing the user's login shell.
- Capturing the input through shell history, clipboard state, terminal scraping, comment-and-reprompt tricks, or simulated keystrokes.
- Adding asynchronous generation, changing provider routing or model contracts, or weakening validation for old-shell syntax.
- Claiming compatibility with untested ble.sh versions or arbitrary third-party line editors.

## Decisions

### 1. Select an editor backend by capability, not Bash version alone

The Bash loader will select one of two backends:

1. If the tested ble.sh commit is attached and its required binding/widget functions are available, select `blesh` on Bash 3.2 or newer.
2. Otherwise, if Bash is 4.0 or newer and native `bind -x` exposes the required line variables, select `readline`.
3. Otherwise, fail before installing or replacing a keybinding.

Selecting ble.sh when it owns the line is necessary even on Bash 4+; native Readline state is no longer the active editor state in that session. Bash 3.2 setup will require ble.sh to load before `eval "$(intent-sh init bash)"`. The first implementation will report a wrong load order rather than registering a delayed hook, because deterministic initialization and rollback are easier to audit.

Alternatives considered: lowering the current version gate without checking the editor still leaves Bash 3 callbacks blind; automatically locating and sourcing ble.sh executes third-party code without explicit user activation; a history/macro fallback cannot safely preserve arbitrary multiline input or exact-command confirmation.

### 2. Bump the adapter protocol and identify the editor explicitly

Adapter protocol version 2 will add fixed-order `editorBackend` and `editorVersion` fields to the NUL-framed request. Supported backend values will be `zle`, `readline`, and `blesh`. The editor version is diagnostic metadata: Zsh and native Bash may reuse their shell version, while ble.sh reports its own version. Both fields are bounded, single-line values and are never sent to the model unless a later prompt requirement explicitly needs them.

Core validation will accept only coherent combinations: Zsh with `zle`; Bash 4.0 or newer with `readline`; and Bash 3.2 or newer with the exact tested `blesh` editor version. It will reject native Readline on Bash 3.2, unknown backends, missing versions, and protocol mismatches before provider invocation. This is capability reporting rather than remote attestation—the adapter is local trusted code—but it prevents accidental execution through an incompatible old adapter.

Both embedded adapters and the core will move to protocol 2 in one change. Existing protocol-1 adapters fail closed with the already documented mismatch behavior, so there is no ambiguous field parsing during upgrade or rollback.

Alternatives considered: accepting all Bash versions without recording a backend makes doctor output and compatibility failures ambiguous; overloading `shellVersion` with editor details breaks parsing; an optional trailing field weakens the fixed-field protocol contract.

### 3. Share rewrite state while isolating editor-specific binding and acceptance

The Bash script will retain one shared rewrite/state machine for request framing, provider lifecycle, response parsing, regeneration, undo, and risk metadata. A small editor layer will own:

- backend detection and capability probing;
- registration and removal of rewrite, undo, and accept bindings;
- editor-native cursor capture and restoration;
- status redraw behavior; and
- delegation to the editor's normal accept-line path.

The ble.sh rewrite and undo actions use its documented edit-command API so the shared code receives emulated `READLINE_LINE` and `READLINE_POINT` values. Guarded Enter uses a namespaced wrapper installed through the pinned internal `ble/function#advice` interface around `ble/widget/default/accept-line`; it reads the pinned `_ble_edit_str` buffer, warns and arms on the first unchanged dangerous Enter, and calls `ble/function#advice/do` only for ordinary acceptance or the second unchanged Enter. Initialization refuses an existing around-advice conflict. Application code does not directly evaluate the generated command or call Bash `eval` with provider-controlled text.

The native Readline guard remains unchanged except for the editor abstraction and protocol field. Key registration will preserve existing bindings when possible and refuse unsupported conflicts rather than silently overwriting them.

Alternatives considered: maintaining two complete Bash adapters would duplicate cancellation and buffer-ownership logic; trying to reuse the native private-continuation macro without a ble.sh contract risks different key-dispatch semantics; replacing Enter with a command evaluator would alter history, jobs, traps, and shell execution semantics.

### 4. Normalize protocol cursors while retaining editor-native restoration state

Protocol 2 will define `cursor` as a zero-based UTF-8 byte offset into `buffer`, matching Go bounds and native Bash Readline. ZLE and ble.sh adapters will convert their logical character position to a byte offset before framing the request. Each adapter will separately retain its editor-native cursor value for local undo and failure restoration. A successful replacement still places the cursor at the editor-native end of the validated command.

Conversion helpers will run locally, reject a position that does not land on a UTF-8 boundary, and receive table and PTY coverage for ASCII, Chinese text, combining characters, and cursor positions before and after multibyte characters. No cursor conversion is delegated to the provider.

Alternatives considered: leaving the unit backend-dependent preserves an existing ambiguity and makes ble.sh behavior impossible to validate consistently; changing to code-point offsets would require converting native Readline's byte position on every Bash request and would not represent grapheme clusters either.

### 5. Treat ble.sh as an optional, pinned compatibility boundary

The project will not vendor or fetch ble.sh. Test setup builds the checksum-verified source archive for commit `d69e4d549a1881a37300fe6b4a05478bd9157dfc`; production code accepts only `BLE_VERSION=0.4.0-nightly+d69e4d5`. The compatibility matrix verifies:

- stock macOS Bash 3.2 initialization and the same ble.sh backend on modern Bash;
- edit-command line and cursor reads/writes;
- Emacs and Vi insert-mode bindings;
- safe/review one-Enter behavior and dangerous two-Enter behavior;
- manual-edit disarming, regenerate, and undo;
- synchronous provider cancellation and terminal restoration; and
- load, detach, reattach, and conflicting-binding behavior.

The resulting minimum is Bash 3.2 and the ble.sh range is the one exact tested commit. Required functions become constants used by adapter initialization, doctor, documentation, and CI. CI obtains the pinned official source archive through an explicit test setup, verifies its SHA-256 checksum, builds it, and passes its path through `INTENT_SH_TEST_BLESH`. Production setup only links to official installation guidance.

ble.sh remains part of the interactive-shell trust boundary: it executes in the user's shell and controls line editing. It is not added to the provider runner's allowlisted environment, and intent text is still sent only through the existing NUL-framed adapter request.

Alternatives considered: vendoring improves test availability but transfers update and redistribution responsibility to this project; accepting any detected `ble-bind` function permits incompatible or spoofed interfaces; building a native extension adds per-OS and per-architecture ABI artifacts and weakens the single-binary distribution property.

### 6. Make runtime adapter state visible to diagnostics without sourcing startup files

After successful initialization, the adapter will export bounded, non-sensitive status markers containing protocol version, backend name, editor version, and readiness. On a failed Bash 3.2 initialization it will leave a stable failure marker such as missing editor, incompatible version, missing API, wrong load order, or binding conflict. `intent-sh doctor` will combine those markers with its existing read-only shell, config, and provider checks; it will not source `.bashrc`, execute ble.sh itself, or trust arbitrary diagnostic text from the environment.

Initialization will detect active ble.sh key conflicts before replacement and export only stable key identifiers, not the bound shell command. Static setup inspection will learn common `ble.sh` and `ble-bind` activation forms and warn when an obvious `intent-sh`-before-ble.sh ordering appears, while runtime detection remains authoritative for framework-managed configurations.

Stable checks will distinguish `shell.editor_backend`, `shell.blesh.version`, `shell.blesh.api`, `shell.blesh.load_order`, and backend-specific key conflicts. Provider subprocess environment filtering remains unchanged, so these markers do not reach Claude Code or Codex.

Alternatives considered: having doctor source user startup files would execute arbitrary code; defining a shell function that shadows the `intent-sh` executable would be invasive; reporting only the Bash major version cannot distinguish a ready Bash 3.2 ble.sh session from an incompatible one.

## Risks / Trade-offs

- [ble.sh changes a widget or binding API] → Pin a tested version range, probe required functions at initialization, run contract PTY tests, and fail closed outside the range.
- [Bash 3.2 line editing is slower] → Keep generation synchronous as today, document ble.sh's Bash 3 performance limitations, and avoid enabling unrelated editor features automatically.
- [A third-party editor expands the interactive-shell trust boundary] → Never install or source it automatically, link to its official distribution, disclose the boundary, and keep provider credentials and environment filtered.
- [Enter behavior differs across ble.sh keymaps or terminal protocols] → Use a backend-native acceptance wrapper, cover Emacs and Vi insert modes plus `C-m`, `C-j`, and supported Return representations, and refuse unknown conflicts.
- [ble.sh detaches or is replaced after initialization] → Recheck a cheap capability/attachment marker on every interactive action and preserve the buffer on failure.
- [Unicode cursor units diverge across editors] → Standardize protocol 2 on UTF-8 byte offsets, retain editor-native restoration state, and add multibyte PTY tests.
- [Protocol 2 temporarily mismatches an older sourced adapter] → Embed both scripts with the binary, fail closed on mismatch, and tell users to reopen or reinitialize the shell after upgrade.
- [The follow-up delta is archived before its base capability] → Complete and sync/archive `build-intent-sh-mvp` first; validate the ordering before archive.
- [Pinned external test downloads make CI unavailable offline] → Verify checksums, cache the artifact in CI, allow an explicit local test path, and keep unit tests independent of the download.

## Migration Plan

1. Complete and sync or archive `build-intent-sh-mvp` so the modified capabilities exist as main specifications.
2. Land the ble.sh compatibility spike and record the tested version/API contract before changing advertised compatibility.
3. Introduce adapter protocol 2 across the core, Zsh adapter, native Bash adapter, fixtures, and diagnostics in one commit-sized unit that continues to reject Bash 3.
4. Add the ble.sh backend behind capability detection, then enable Bash 3.2 requests only after the complete PTY safety matrix passes.
5. Update setup, doctor, README compatibility tables, trust documentation, and CI before declaring Bash 3 support.

Rollback is installation of the previous binary and re-evaluation of its embedded adapter, or removal of the `intent-sh` activation line. Protocol mismatch prevents an old/new mixed pair from rewriting. Rollback never changes the user's ble.sh installation or shell selection.

## Resolved Compatibility Questions

- The tested editor is upstream commit `d69e4d549a1881a37300fe6b4a05478bd9157dfc`, reported as `0.4.0-nightly+d69e4d5`; other versions fail closed.
- Guarded acceptance requires the narrowly pinned internal advice interface documented above; the public edit-command interface alone is insufficient to intercept every ordinary accept-line path.
- Bash 3.0 and 3.1 are not practical to keep in the safety matrix, so the normative conditional minimum is Bash 3.2.
