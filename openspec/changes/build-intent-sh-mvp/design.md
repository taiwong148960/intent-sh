## Context

`intent-sh` starts from an empty implementation repository and a detailed product brief. The product is deliberately smaller than an agent: an interactive Zsh or Bash user asks for one command by pressing a key, the current editable buffer is replaced, and the shell remains the only component that can execute it. The MVP targets macOS and Linux on amd64 and arm64, supports Chinese and English intent, and reuses an existing Claude Code or Codex CLI login rather than handling model credentials.

The security boundary crosses four independently fallible layers: a shell editing API, the `intent-sh` process, an external provider CLI, and model output. Shell code is difficult to quote safely, provider CLIs can change, and destructive-command detection is necessarily heuristic. The design therefore keeps the adapter small, invokes providers without an intermediary shell, validates output locally, and never treats provider metadata as authorization to execute.

The project and executable use the name `intent-sh`; `ai-shell` in the source brief is treated as a descriptive placeholder. The initial Go module is `github.com/taiwong148960/intent-sh`.

## Goals / Non-Goals

**Goals:**

- Deliver one buildable Go binary and two thin interactive-shell adapters.
- Rewrite the complete current input buffer, regenerate from its preserved original, undo, and cancel without executing a generated command.
- Isolate and constrain Claude Code and Codex CLI subprocesses while preserving their official login access.
- Make malformed or multi-statement output fail closed and give locally derived, explainable risk warnings.
- Require two deliberate Enter presses for an unchanged dangerous generated command.
- Provide deterministic tests that do not call a real model, plus source-install, configuration, diagnostic, and removal paths.

**Non-Goals:**

- Fish, PowerShell, Windows, arbitrary terminal-screen selection, or partial-buffer rewriting.
- Explanations, chat/history, multi-step work, automatic command execution, project inspection, or terminal-output capture.
- Desktop software, Accessibility APIs, a daemon, a hosted service, direct model APIs, API keys, accounts, telemetry, or a database.
- Perfect semantic safety analysis. The engine reduces accidental execution risk but does not certify a command as harmless.
- Binary release automation or package-manager formulae in the first source-built MVP.

## Decisions

### 1. Use a layered Go core with shell-only editing responsibilities

The Go binary owns request construction, configuration, prompting, provider processes, response validation, risk analysis, and diagnostics. Adapters only read and write the active editor buffer, maintain per-session rewrite state, render status, and guard Enter for dangerous generated results.

The initial layout is:

```text
cmd/intent-sh/                 command dispatch
internal/app/                  rewrite orchestration and typed errors
internal/protocol/             versioned adapter and provider contracts
internal/context/              allowlisted environment snapshot
internal/prompt/               system prompt and schema assembly
internal/provider/             router, process runner, claude, codex
internal/safety/               structural validation and risk rules
internal/config/               XDG TOML loading and validation
internal/doctor/               compatibility and readiness checks
shell/zsh/intent-sh.zsh        ZLE widgets
shell/bash/intent-sh.bash      Readline bindings
schemas/                       provider-result JSON Schema
testdata/providers/            deterministic fake provider executables
```

The primary path is `adapter -> app.Rewrite -> provider.Router -> validator -> risk engine -> adapter`. Interfaces exist only at the provider runner and filesystem/process seams needed for tests; the rest stays as ordinary packages to avoid premature abstraction.

Alternatives considered: a shell-only implementation would duplicate quoting, JSON, provider, and safety logic; a daemon would improve repeated-call latency but adds lifecycle and IPC complexity before latency is measured; a Rust core provides similar distribution properties but slows the first implementation relative to the requested Go design.

### 2. Use a versioned, NUL-framed adapter transport and canonical Go/JSON types

Shell adapters call `intent-sh adapter rewrite --protocol 1` and exchange a fixed-order set of NUL-terminated fields over stdin/stdout. Fields include action, shell and version, current buffer and cursor, original buffer, previous generated command, generation index, and request ID. The reply includes status, replacement or question, provider, locally derived risk and reason, and the echoed request ID.

NUL framing allows arbitrary shell text without `eval`, JSON escaping in shell, command-line leakage, a `jq` dependency, or lossy command substitution. Both Bash 3.2 and Zsh can read fields from process substitution with `read -d`. The Go `Request` and `Result` structures remain the canonical versioned contract and have JSON fixtures; the model-facing request and result are strict JSON.

The adapter protocol has an explicit version so incompatible binaries fail with an actionable message instead of misassigning fields. Output is fully buffered and only emitted after validation, so a partial provider response cannot become a buffer replacement.

Alternatives considered: raw stdin/stdout JSON matches the conceptual architecture but makes safe parsing in stock shells depend on `jq`; shell assignment output requires sourcing generated text; command-line flags expose intent in process listings and cannot safely carry all state.

### 3. Keep rewriting synchronous and state owned by each shell session

Each adapter maintains `originalBuffer`, `generatedCommand`, `provider`, `riskLevel`, `riskReason`, `requestID`, and `generationIndex` in namespaced shell variables.

- On the first `Alt+G`, a non-empty buffer becomes the original and generation index zero is requested.
- If `Alt+G` is pressed while the buffer exactly equals the last generated command, the adapter sends the preserved original plus the previous command and increments the index.
- If the user changed the generated command, its fingerprint no longer matches; the old state is cleared and that edited buffer becomes a new original.
- `Alt+U` restores the original only while the active buffer still matches the generated command, preventing an undo widget from overwriting manual edits.
- A clarification or error leaves the buffer untouched. A successful response replaces the full buffer and puts the cursor at its end.

The adapter shows a transient “generating” status and waits synchronously. `Ctrl+C` reaches the foreground Go process, which cancels its context and terminates the complete provider process group. Synchronous operation is simpler and eliminates most stale-response races; request IDs are still compared before applying a reply and make a later asynchronous implementation compatible.

Alternatives considered: background jobs keep editing responsive but require a reliable ZLE/Readline notification channel and stronger concurrent buffer ownership. That complexity is deferred until real latency data justifies it.

### 4. Implement native ZLE/Readline widgets and a shell-specific Enter guard

The Zsh adapter uses ZLE widgets to access `BUFFER` and `CURSOR`, `zle -M` for messages, and a wrapped `accept-line` widget for dangerous confirmation. The wrapper calls the original `accept-line` only when the generated-command fingerprint is not dangerous or is already armed.

The Bash adapter uses `bind -x`, `READLINE_LINE`, and `READLINE_POINT`. To preserve normal Readline execution semantics, Enter is a macro composed of a private guard key followed by a private continuation key. The guard callback dynamically maps the continuation to either `accept-line` or a no-op for that keypress. On the first Enter for an unchanged dangerous generated command it selects the no-op, renders the warning, and arms the fingerprint; on the second it selects `accept-line`. Every guard invocation compares the current buffer fingerprint, so editing disarms the confirmation. This behavior must be proven on the oldest supported Bash 3.2 before the remaining Bash adapter work proceeds.

Adapters never call `eval` on generated output and never run the generated command themselves. Review-risk commands receive a yellow message but retain ordinary Enter behavior. Dangerous commands receive a red reason; a rewrite, undo, cancellation, or buffer change clears the armed state. The adapters expose fixed MVP defaults (`Alt+G`, `Alt+U`, and normal `Ctrl+C` cancellation) and detect conflicting bindings during setup/doctor rather than silently replacing unknown custom behavior.

Alternatives considered: replacing dangerous text with a confirmation wrapper changes the command the user reviews; manually evaluating safe commands from a Readline callback changes history, jobs, and shell semantics; OS-level keystroke injection is unreliable and violates the product boundary.

### 5. Constrain providers behind one process runner and strict routing rules

`provider.Provider` accepts a typed request and returns raw structured output plus provider metadata. Claude and Codex implementations only build argument lists; a shared runner uses direct `exec.CommandContext`-style process creation, bounded stdout/stderr capture, a new Unix process group, and a temporary empty working directory removed after each call. It never invokes `/bin/sh -c`.

The Claude adapter uses non-interactive bare mode, disables tools and session persistence, and requests JSON matching the embedded schema. The Codex adapter uses an ephemeral, read-only non-interactive execution, ignores project configuration, skips the Git-repository requirement, writes the schema and final result only inside the temporary directory, and receives the prompt on stdin. Exact flag sets and minimum supported CLI versions live in provider-specific code and contract tests rather than being scattered through the application.

The child environment is an allowlist sufficient for the official CLI to find its executable, locale, TLS roots, and its normal user configuration/login. `intent-sh` never locates, parses, copies, or stores provider tokens. The model receives no environment-variable map and cannot access the real current directory because the provider process runs elsewhere with model tools disabled.

Routing follows the TOML configuration:

```toml
provider = "auto"
priority = ["claude", "codex"]
timeout_seconds = 30
model = ""
```

In `auto`, providers are tried sequentially. A valid `ok` or `clarify` result stops routing. Unavailable executables, missing login, supported transient/provider-limit failures, timeouts, and invalid structured output can advance to the next configured provider; user cancellation never does. Selecting an explicit provider disables fallback so configuration mistakes are visible. Typed errors are mapped to short actionable messages; raw prompts, model output, and unbounded provider stderr are not logged.

Alternatives considered: parallel requests waste subscription quota and make provider choice nondeterministic; direct APIs require new credentials and a cloud-facing product surface; invoking provider commands through the user's shell permits aliases and injection.

### 6. Build the smallest useful context and a safety-biased prompt

The core derives OS and architecture itself. The adapter supplies the shell name/version, cursor, and buffer. The core adds the current path as text, a boolean derived from the presence of SSH markers, an allowlisted locale, and installed status for a fixed small command list using `exec.LookPath`. It does not enumerate arbitrary commands or read history, terminal output, project files, Git state, clipboard data, `.env` files, SSH configuration, or environment values beyond the explicit locale/remote signals.

For regeneration, the model receives the preserved original, the immediately previous command, and the generation index and is told to return a materially different valid alternative. The prompt requires one command, no execution or inspection, no implicit `sudo`, preference for available tools and target-platform syntax, preservation of existing shell fragments, and a preview command for ambiguous destructive intent.

Provider output is a strict JSON union:

- `ok`: command, short explanation, assumptions, and an untrusted risk hint.
- `clarify`: one concise question and no command.

Chinese input does not change the command-language contract; clarification may follow the input language. There is no conversation state beyond the current shell-session rewrite chain.

### 7. Validate structure and syntax before applying deterministic local risk rules

Validation is fail-closed and ordered:

1. Decode exactly one schema-valid JSON value with bounded input and no unknown fields.
2. Require a non-empty command no longer than 8 KiB and reject NUL, CR/LF, Markdown fences, and surrounding model chatter.
3. Parse with `mvdan.cc/sh/v3/syntax` in Bash/POSIX-compatible mode. Require one top-level simple command or pipeline with redirections; reject command lists, background jobs, compound statements, functions, heredocs, and nested multi-statement substitutions.
4. Ask the target shell to parse without execution (`bash --noprofile --norc -n -c` or `zsh -f -n -c`) to catch shell-specific syntax errors.
5. Walk normalized command names, arguments, pipelines, and redirections through ordered local risk rules.

The local classification is authoritative; the model's `riskHint` is never used to lower it. Read-only known commands can be safe. State-changing commands such as `mv`, `cp`, `kill`, permission changes, `git reset`, and `docker rm` are review. Known high-impact patterns such as recursive forced deletion, privileged deletion, raw-disk tools, download-to-shell pipelines, destructive Git cleanup/reset, shutdown/reboot, and destructive database statements are dangerous. Unknown or dynamically obscured commands default to review rather than safe. Every non-safe result carries a stable reason code and human-readable target/pattern when it can be derived without filesystem inspection.

Rules are data-driven and covered by table tests, but they are not a sandbox. A user can edit and execute any buffer, and documentation must say that “safe” means no known risky pattern matched, not proof of harmlessness.

Alternatives considered: trusting the model is not an independent safety boundary; regex alone misses quoting and wrappers; executing a dry run is not generally possible and could itself have effects; a complete shell semantic analyzer is beyond MVP scope.

### 8. Embed adapters and make setup explicit, reversible, and diagnostic

The Go binary embeds the two version-matched adapter scripts. `intent-sh init zsh|bash` prints the selected script for the conventional startup line `eval "$(intent-sh init <shell>)"`. `intent-sh setup <shell>` prints the exact idempotent line and target startup file; it does not modify dotfiles unless a future explicit mutation option is added. This keeps initial installation transparent and rollback to removing one line.

Configuration lives at `${XDG_CONFIG_HOME:-$HOME/.config}/intent-sh/config.toml`, contains no secrets, and is parsed strictly. CLI defaults work with no file. `intent-sh config path|show|set` provides safe manipulation through atomic replacement and rejects unknown providers, invalid priority entries, timeouts outside 1–120 seconds, or unknown keys.

`intent-sh doctor` reports machine-readable check IDs plus readable guidance for platform/architecture, config validity, adapter/binary protocol match, keybinding conflicts, provider executable/version, and official CLI login readiness. It never prints credential material. A nonzero exit means no configured provider can currently serve a rewrite or the active adapter is incompatible.

Alternatives considered: automatically rewriting shell startup files is convenient but risky in an MVP; shipping loose scripts can drift from the binary protocol; storing provider auth would duplicate and weaken official credential handling.

### 9. Test from pure rules outward without live model calls

Unit tests cover config, framing, context allowlists, prompt construction, strict result decoding, AST policy, every risk rule, routing, cancellation, and fingerprint state transitions. Fake `claude` and `codex` executables assert arguments, working directory, bounded environment, stdin, timeout, and process-tree cleanup while returning golden valid, clarify, malformed, slow, and failing results.

PTY integration tests launch clean `zsh -f` and `bash --noprofile --norc` sessions with the fake provider first on `PATH`. They exercise rewrite, regenerate from the original, undo, clarification, cancellation, manual-edit invalidation, and first/second Enter behavior without ever executing a destructive target; a harmless marker command stands in for danger after the classifier is injected with a test rule. CI runs `go test ./...`, `go vet ./...`, formatting checks, shell syntax checks, and PTY tests on Ubuntu and macOS.

No telemetry or persistent logs are added. Errors are observable through status text, exit codes, and `doctor`; an opt-in future debug mode would need explicit redaction rules before it can exist.

## Risks / Trade-offs

- [Provider CLI flags or output formats drift] -> Centralize each invocation, record supported version ranges, make `doctor` probe compatibility, and exercise argument/output contracts with fixtures before release.
- [Bash Readline cannot reliably suppress the continuation key on all supported versions] -> Make a Bash 3.2/5.x PTY spike the first adapter task and block Bash completion until first/second Enter tests pass; do not fall back to automatic `eval`.
- [Risk rules produce false negatives or false positives] -> Default unknown constructs to review, make rules structural and reasoned, require double confirmation for known dangerous patterns, and state the limits prominently.
- [Provider latency freezes line editing] -> Show status, enforce a 30-second default deadline, make `Ctrl+C` kill the process tree, and defer asynchronous jobs until the synchronous workflow has usage data.
- [Shell quoting or protocol confusion becomes command injection] -> Use NUL framing, fixed field order, version negotiation, bounded fields, and no sourcing/evaluation of provider-controlled output.
- [Portable parsing rejects valid Zsh syntax] -> Ask for portable commands in the prompt, validate with both an AST policy and the target shell, and return a clear regeneration message rather than weakening validation.
- [Adapter Enter wrapping conflicts with existing custom bindings] -> Detect and report conflicts during setup/doctor, keep startup changes explicit, and document removal; configurable shortcuts remain post-MVP.
- [The external CLI can still access its user configuration] -> Use bare/ignore-user-config modes where supported, an empty working directory and no model tools, while retaining only the configuration needed for official login; document that provider CLI behavior is part of the trust boundary.

## Migration Plan

There is no existing runtime or user data to migrate. Build the core and fake-provider tests first, add Zsh and Bash adapters behind the embedded `init` command, then publish the source-install instructions only after both PTY suites and provider compatibility checks pass. The initial release is opt-in per shell session through one startup-file line.

Rollback is removal of that startup line and the `intent-sh` binary. The optional TOML file contains no credentials and can be retained or deleted. Protocol version mismatches fail closed, allowing a user to restore the previous binary without changing persistent state.

## Open Questions

- Which exact minimum Claude Code and Codex CLI versions satisfy all isolation and structured-output flags must be recorded from compatibility tests during implementation; the architecture does not depend on a particular version string.
- Whether the Bash continuation-macro technique is reliable on every supported Readline build is intentionally resolved by the first Bash PTY spike. If it cannot meet the specified behavior, Bash support must remain experimental rather than weakening the confirmation contract.
