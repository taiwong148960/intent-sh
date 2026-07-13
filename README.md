# intent-sh

`intent-sh` turns the editable text at your shell prompt into one validated command. It works inside Zsh or modern Bash, reuses an already authenticated Claude Code or Codex CLI, and leaves the generated command in the buffer for you to inspect. It never presses Enter or executes the command for you.

This repository is an MVP. Its local safety checks reduce accidental risk; they are not a sandbox or a guarantee that a command is harmless.

## Compatibility

| Component | MVP support |
| --- | --- |
| Operating system | macOS or Linux |
| Architecture | amd64/x86_64 or arm64/AArch64 |
| Zsh | Supported; the stock macOS Zsh is suitable |
| Bash | 4.0 or newer |
| Stock macOS Bash 3.2 | Not supported; it lacks the editable-buffer interface needed by the adapter |
| Provider | At least one compatible, logged-in official Claude Code or Codex CLI |

Windows, WSL, Fish, PowerShell, and other shells are outside the MVP compatibility contract.

## Quick start

### 1. Build from source

Install Go 1.24 or newer, then build the single executable:

```sh
git clone https://github.com/taiwong148960/intent-sh.git
cd intent-sh
mkdir -p ./bin
go build -trimpath -o ./bin/intent-sh ./cmd/intent-sh
install -d "$HOME/.local/bin"
install -m 0755 ./bin/intent-sh "$HOME/.local/bin/intent-sh"
```

Make sure `$HOME/.local/bin` is on `PATH`, then verify both versions:

```sh
intent-sh version
```

Go is used because the product needs one source-buildable binary with embedded shell adapters, strict typed contracts, and reliable Unix subprocess/process-group control. There is no daemon, database, desktop app, or hosted `intent-sh` service.

### 2. Install and sign in to one official provider

Use an authentication method supported and stored by the official CLI:

- [Codex CLI installation](https://developers.openai.com/codex/cli/): install Codex, run `codex login`, and verify with `codex login status`.
- [Claude Code setup](https://code.claude.com/docs/en/getting-started): install Claude Code, run `claude`, and follow its browser login prompts.

`intent-sh` does not ask for an API key, implement login, read provider credential files, or copy credentials into its own config. It invokes the selected official CLI, which remains responsible for its account, authentication, and credential storage.

### 3. Inspect and activate the shell adapter

Ask for read-only setup guidance for your shell:

```sh
intent-sh setup zsh
# or, only with Bash 4.0+:
intent-sh setup bash
```

The command identifies the likely startup file, reports default-key conflicts, and prints this activation line without editing anything:

```sh
eval "$(intent-sh init zsh)"
```

Use `bash` instead of `zsh` for modern Bash. Add the printed line to the reported startup file, then open a new shell session. Loading is idempotent within a session, and an adapter/binary protocol mismatch fails before installing bindings.

### 4. Diagnose readiness

```sh
intent-sh doctor
```

Doctor prints stable `PASS`, `WARN`, `FAIL`, and `SKIP` check IDs for the platform, architecture, shell version, adapter protocol, config, key conflicts, provider executable, compatible features, and official login readiness. It succeeds when the core checks pass and at least one configured provider is ready. It never prints tokens or credential-file contents.

### 5. Try a harmless rewrite

At a fresh shell prompt, type a plain-language intent but do not press Enter:

```text
show the current directory
```

Press `Alt+G`. A successful request replaces the full editable line with one command, such as `pwd`, and moves the cursor to the end. Read the command first. It has not run. Press Enter only if you want your shell to accept it.

## Editing workflow

- `Alt+G` rewrites the complete current buffer.
- `Alt+G` again, while the generated command is unchanged, requests a different alternative from the preserved original intent.
- `Alt+U` restores the original buffer only while the generated command is unchanged. It will not overwrite manual edits.
- `Ctrl+C` during “generating” cancels the provider process tree, stops fallback, and keeps the original buffer.
- Editing a generated command makes it ordinary user-owned shell input, starts a new rewrite chain at the next adapter action, and clears any detected dangerous-command confirmation state.

A clarification, timeout, cancellation, malformed provider response, invalid shell command, or stale response leaves the editable buffer unchanged.

## Risk levels and Enter behavior

Every provider result must decode as one bounded structured object and then pass local one-command parsing, target-shell syntax validation without execution, and deterministic risk classification. The provider’s own risk hint can never lower the local result.

- `safe`: matches a known read-only baseline and keeps normal one-Enter acceptance. “Safe” is not proof of harmlessness.
- `review`: is unknown, dynamic, state-changing, privileged, environment-altering, or uses an explicit executable path. A warning is shown, but one deliberate Enter still uses normal shell acceptance.
- `dangerous`: matches a destructive pattern such as recursive deletion, privileged deletion, raw-disk tools, opaque shell evaluation, download-to-shell, destructive Git/database operations, or shutdown. The first Enter only warns and arms that exact unchanged command. A second consecutive Enter delegates to the shell’s native acceptance.

Any buffer difference observed by the guard, rewrite, regeneration, undo, cancellation, accepted command, or request change disarms the dangerous confirmation. Bash Readline and ZLE do not expose a durable edit counter, so an edit that is restored byte-for-byte between guard callbacks is indistinguishable from unchanged text; the guard still never accepts a different command under the old fingerprint. The adapter never evaluates generated text and never runs a target command itself.

These checks are heuristic. Aliases, functions, shell options, environment state, command-version differences, filesystem state, remote systems, and programs outside the rules can change what a command does. Always review the visible command; use a real sandbox or least-privilege environment when consequences matter.

The completed MVP review and closed findings are recorded in [docs/threat-review.md](docs/threat-review.md).

## Configuration

The optional secret-free config lives at:

```text
${XDG_CONFIG_HOME:-$HOME/.config}/intent-sh/config.toml
```

No file is created until `config set` is used. Defaults are auto routing, Claude then Codex priority, a 30-second timeout, and no forced model.

```sh
intent-sh config path
intent-sh config show
intent-sh config set provider codex
intent-sh config set provider auto
intent-sh config set priority codex,claude
intent-sh config set timeout_seconds 45
intent-sh config set model '<provider-supported-model>'
```

Supported keys are `provider`, `priority`, `timeout_seconds`, and `model`. Credential fields and unknown keys are rejected. In `auto` mode, providers are attempted sequentially in priority order; cancellation never falls back. Selecting `claude` or `codex` explicitly disables fallback.

## Exactly what providers receive

The structured prompt contains only:

- the complete active buffer and cursor position;
- on regeneration only, the preserved original buffer, previous generated command, and generation index;
- operating system and architecture;
- shell name and shell version;
- current working-directory path;
- a boolean indicating whether SSH markers are present, never their values;
- locale selected from `LC_ALL`, `LC_MESSAGES`, or `LANG`;
- names, without paths or versions, of commands found from this fixed allowlist: `awk`, `curl`, `docker`, `fd`, `find`, `git`, `grep`, `jq`, `kubectl`, `lsof`, `ps`, `rg`, `sed`, `ss`, and `wget`.

Because the active buffer is intentionally sent, do not request a rewrite while the buffer itself contains a password, token, private key, or other secret.

`intent-sh` does not read or send:

- shell history;
- terminal output, scrollback, or screen contents;
- arbitrary environment variables or SSH marker values;
- project/repository file contents, `.env` files, or Git diffs;
- clipboard data;
- SSH configuration;
- provider tokens, credential files, or other authentication material.

The official provider subprocess receives a small environment allowlist needed to locate its executable, official login storage, locale, certificates, and proxy configuration. Those environment values are not serialized into the model prompt. The CLI runs directly—not through a shell string—in a new empty temporary directory with bounded output and a deadline. `intent-sh` removes that directory after success, failure, timeout, or cancellation.

## Trust boundaries

You are trusting:

- the local `intent-sh` binary and the embedded Bash/Zsh adapter you explicitly load;
- the selected official provider executable, its installed version, user-level configuration, account, and login storage;
- the provider service to process the documented prompt payload;
- your shell and every executable named by a command you choose to accept.

You are not trusting `intent-sh` with provider credentials, automatic command execution, project-file access, persistent rewrite logs, or telemetry. Rewrite state exists only as variables in the current shell session. A model result is untrusted until it passes the local decoder and safety pipeline, and even a passing command still requires your Enter key.

## Non-goals for the MVP

- automatic command execution or multi-step agents;
- API-key management, OAuth, provider accounts, or a hosted backend;
- Fish, PowerShell, Windows/WSL, or shells other than Zsh and Bash 4.0+;
- selected-text rewriting, terminal-screen scraping, clipboard access, or global keystroke simulation;
- reading project files, history, terminal output, or Git state to enrich prompts;
- a desktop UI, daemon, database, telemetry, persistent logs, or command-history store;
- configurable shortcuts, command explanations as a separate product surface, or a general shell sandbox.

## Removal

Run `intent-sh setup zsh` or `intent-sh setup bash` again to print the startup file and exact activation line. Delete only that line:

```sh
eval "$(intent-sh init zsh)"
```

Open a new shell, then remove the binary from wherever you installed it. For the quick-start path:

```sh
rm -f "$HOME/.local/bin/intent-sh"
```

The optional config contains no credentials and can be removed independently:

```sh
rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/intent-sh/config.toml"
rmdir "${XDG_CONFIG_HOME:-$HOME/.config}/intent-sh" 2>/dev/null || true
```

Removing `intent-sh` does not uninstall Claude Code or Codex CLI and does not alter their accounts or credentials.

## Development

Run the reproducible checks with:

```sh
make check
```

On macOS, point `INTENT_SH_TEST_BASH` at a separately installed Bash 4.0+ to run the modern-Bash PTY matrix. The suite still verifies that `/bin/bash` 3.2 is rejected before bindings are installed.

Real provider checks are deliberately opt-in; see the [provider compatibility record](docs/provider-compatibility.md).
