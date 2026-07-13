## 1. Foundation and Contracts

- [ ] 1.1 Initialize `github.com/taiwong148960/intent-sh` as a Go module, add the `cmd/intent-sh` entry point, and establish `internal`, `shell`, `schemas`, and `testdata` package boundaries from the design.
- [ ] 1.2 Define shared command exit codes and typed error categories for invalid input, configuration, provider availability, timeout, cancellation, provider output, safety rejection, and protocol incompatibility, with unit tests for user-facing mappings.
- [ ] 1.3 Implement the version-1 adapter request/result types and bounded NUL-framed codec, including round-trip, truncation, malformed-field-count, embedded-newline, and protocol-mismatch tests.
- [ ] 1.4 Define and embed the strict provider-result JSON Schema and matching Go union types for `ok` and `clarify`, with golden valid and invalid fixtures.
- [ ] 1.5 Pin the minimal Go dependencies for TOML, shell AST parsing, and PTY testing, and add reproducible formatting, vet, unit-test, and shell-syntax developer commands.

## 2. Shell Confirmation Feasibility Spikes

- [ ] 2.1 Prototype the Bash Readline guard-macro/private-continuation technique without model integration and prove that an unchanged test-danger command is blocked on first Enter and accepted through native Readline on second Enter.
- [ ] 2.2 Add PTY coverage for the Bash confirmation prototype on macOS Bash 3.2 and a current Linux Bash, including edit-to-disarm and ordinary-command acceptance; record the supported behavior before continuing the Bash adapter.
- [ ] 2.3 Prototype the Zsh wrapped `accept-line` widget and add PTY coverage for first-Enter blocking, second-Enter native acceptance, and edit-to-disarm.

## 3. Configuration, Context, and Prompting

- [ ] 3.1 Implement strict XDG TOML configuration with documented defaults, provider/priority/timeout/model validation, no-file behavior, and table-driven invalid-config tests.
- [ ] 3.2 Implement `config path`, `config show`, and atomic `config set` operations without credential fields, and test permissions, replacement failure, and effective-value output.
- [ ] 3.3 Implement the environment context builder for OS, architecture, shell/version, current path, SSH boolean, locale, and fixed tool allowlist, with tests proving prohibited environment values and files are never read or serialized.
- [ ] 3.4 Implement the safety-biased rewrite prompt for initial and regenerated requests, including original intent, previous command, generation index, Chinese/English clarification guidance, target platform, and destructive-preview rules.
- [ ] 3.5 Add golden prompt/request tests that assert no history, terminal output, arbitrary environment map, project content, credential data, or implicit `sudo` instruction can enter provider input.

## 4. Provider Process Layer and Routing

- [ ] 4.1 Implement a shared direct-exec provider runner with an empty temporary working directory, allowlisted child environment, bounded stdout/stderr, configured deadline, Unix process groups, cancellation, and guaranteed cleanup.
- [ ] 4.2 Create fake provider executables and integration tests that prove argument boundaries, stdin delivery, isolated working directory, environment filtering, timeout, output bounds, descendant-process termination, and temporary-file removal.
- [ ] 4.3 Implement the Claude Code adapter with centralized non-interactive bare/tool-disabled/sessionless/schema arguments, supported-version and login-readiness probing, and contract tests for success, clarify, auth failure, and malformed output.
- [ ] 4.4 Implement the Codex CLI adapter with centralized ephemeral/read-only/ignored-user-config/schema/final-message arguments, temporary schema/result files, supported-version and login-readiness probing, and equivalent contract tests.
- [ ] 4.5 Implement strict provider-result decoding with single-value, unknown-field, branch-shape, trailing-content, and bounded-output enforcement before results reach command validation.
- [ ] 4.6 Implement deterministic provider routing for auto priority and explicit modes, including typed fallback eligibility, valid-clarify termination, invalid-output fallback, timeout fallback, no fallback after cancellation, and aggregated bounded diagnostics.

## 5. Command Validation and Local Risk

- [ ] 5.1 Implement command boundary checks for empty text, 8 KiB maximum, NUL, CR/LF, Markdown fences, and surrounding chatter, with tests for every rejection path.
- [ ] 5.2 Implement shell AST validation that accepts one simple command or pipeline with redirections and rejects lists, background jobs, compound statements, functions, heredocs, and multi-statement nested substitutions.
- [ ] 5.3 Implement no-execution Bash and Zsh syntax checks with startup files disabled, and prove with marker fixtures that substitutions and target commands are never executed during validation.
- [ ] 5.4 Implement normalized wrapper and pipeline analysis for `sudo`, `env`, `command`, `builtin`, executable stages, arguments, and redirections, returning stable risk reason codes.
- [ ] 5.5 Implement safe baseline rules for documented read-only commands and review rules for state-changing or unknown commands, with table tests covering options, quoting, paths, pipelines, and conservative defaults.
- [ ] 5.6 Implement dangerous rules for recursive/privileged deletion, disk tools, download-to-shell pipelines, destructive Git operations, destructive database statements, and shutdown/reboot, with bypass-oriented table and fuzz tests.
- [ ] 5.7 Combine validation and classification so local risk can only preserve or raise the provider hint, and test that every rejected command fails closed while every review/dangerous result contains a concise reason.

## 6. Rewrite Orchestration and CLI Surface

- [ ] 6.1 Implement the rewrite application flow from protocol decode through config, context, prompt, provider routing, strict validation, local risk, and fully buffered protocol response.
- [ ] 6.2 Implement empty-input rejection, clarification, cancellation, timeout, fallback, invalid-command, and successful-command flows with fake providers and assertions that failures never emit a replacement.
- [ ] 6.3 Implement `adapter rewrite --protocol 1`, `init`, `setup`, `config`, `doctor`, and `version` command dispatch with stable exit behavior and command-level tests.
- [ ] 6.4 Embed version-matched Zsh and Bash scripts in the binary and reject adapter initialization or rewrite when protocol versions differ.

## 7. Zsh Adapter

- [ ] 7.1 Implement the ZLE adapter loader, namespaced per-session state, protocol negotiation, `Alt+G` rewrite widget, NUL-framed I/O, generating status, and validated full-buffer replacement.
- [ ] 7.2 Implement Zsh regeneration from the preserved original, manual-edit chain invalidation, request-ID checking, and `Alt+U` restoration without overwriting edited buffers.
- [ ] 7.3 Implement clarification/error status, foreground `Ctrl+C` cancellation, cursor restoration, and state cleanup for failed or cancelled Zsh requests.
- [ ] 7.4 Integrate the proven Zsh `accept-line` guard with safe/review/dangerous display, exact-command fingerprinting, second-Enter acceptance, and disarming on every specified state change.
- [ ] 7.5 Add clean-`zsh -f` PTY scenarios for initial rewrite, mixed input, regenerate, undo, manual editing, clarification, malformed output, timeout, cancellation, stale ID, review warning, and dangerous confirmation.

## 8. Bash Adapter

- [ ] 8.1 Implement the Readline adapter loader, namespaced per-session state, protocol negotiation, `Alt+G` rewrite binding, NUL-framed I/O, status rendering, and validated `READLINE_LINE`/`READLINE_POINT` replacement.
- [ ] 8.2 Implement Bash regeneration from the preserved original, manual-edit chain invalidation, request-ID checking, and `Alt+U` restoration without overwriting edited buffers.
- [ ] 8.3 Implement clarification/error status, foreground `Ctrl+C` cancellation, cursor restoration, and state cleanup for failed or cancelled Bash requests.
- [ ] 8.4 Integrate the proven Bash guard-macro continuation with normal Readline acceptance, exact-command fingerprinting, safe/review/dangerous behavior, second-Enter acceptance, and edit/rewrite/undo/cancel disarming.
- [ ] 8.5 Add clean-`bash --noprofile --norc` PTY scenarios matching the Zsh acceptance matrix on both Bash 3.2 and current Linux Bash.

## 9. Setup, Doctor, and Documentation

- [ ] 9.1 Implement `setup zsh|bash` guidance that detects the likely startup file, prints an idempotent activation line, default bindings, conflict warnings, and exact removal steps without modifying dotfiles.
- [ ] 9.2 Implement doctor checks with stable IDs for platform/architecture, config, binary/adapter protocol, default-key conflicts, provider executable/version/features, and official login readiness, plus success and failure exit-code tests.
- [ ] 9.3 Verify doctor and all ordinary errors redact credential material, prompt bodies, raw model output, and unbounded provider stderr using adversarial fixtures containing sentinel secrets.
- [ ] 9.4 Write the README quick start for source build, provider prerequisites, activation, doctor, harmless first rewrite, regenerate, undo, cancel, risk levels, dangerous confirmation, and removal.
- [ ] 9.5 Document the exact provider context allowlist and prohibited data, trust boundaries, no-auto-execution guarantee, heuristic safety limitations, MVP compatibility matrix, and explicit non-goals.

## 10. End-to-End MVP Verification

- [ ] 10.1 Add Ubuntu and macOS CI for formatting, `go vet`, `go test ./...`, Bash/Zsh syntax checks, fake-provider integration tests, PTY suites, and amd64/arm64 build checks where runners support them.
- [ ] 10.2 Run end-to-end fake-Claude and fake-Codex flows in Zsh and Bash and verify rewrite, fallback, regenerate-from-original, undo, clarification, cancellation, privacy bounds, and no automatic command execution.
- [ ] 10.3 Run opt-in smoke tests against compatible logged-in Claude Code and Codex CLIs from a disposable directory using harmless prompts, recording CLI version compatibility without capturing prompts or credentials.
- [ ] 10.4 Complete a threat-focused review of shell framing, subprocess isolation, output parsing, AST policy, risk-rule bypasses, Enter confirmation, config writes, and diagnostics; add regression tests for every issue found.
- [ ] 10.5 Validate all OpenSpec artifacts, build the source-installable MVP on macOS and Linux, and confirm the documented install-to-uninstall journey before marking the change complete.
