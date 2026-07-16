# intent-sh

`intent-sh` turns the editable text at your shell prompt into one validated command. It works inside Zsh, modern Bash, and stock macOS Bash 3.2 when the exact tested ble.sh editor is attached. It reuses an already authenticated Claude Code or Codex CLI and leaves the generated command in the buffer for you to inspect. It never presses Enter or executes the command for you.

This repository is an MVP. Its local safety checks reduce accidental risk; they are not a sandbox or a guarantee that a command is harmless. Terminal support is behavioral: a terminal is **contract-compatible** when it satisfies the PTY contract below, and a named environment is **qualified** only when it has a dated result in [the terminal qualification record](docs/terminal-qualification-results.md).

## Compatibility

| Component | MVP support |
| --- | --- |
| Operating system | macOS or Linux |
| Architecture | amd64/x86_64 or arm64/AArch64 |
| Zsh | Supported; the stock macOS Zsh is suitable |
| Bash 4.0 or newer | Supported through native Readline, or through the tested ble.sh backend when it is attached |
| Stock macOS Bash 3.2 | Conditionally supported only with ble.sh `0.4.0-nightly+d69e4d5` (commit `d69e4d549a1881a37300fe6b4a05478bd9157dfc`) attached before `intent-sh` initializes |
| Bash 3.0, 3.1, or plain Bash 3.2 | Not supported; no reproducible safe editable-buffer contract is available |
| Provider | At least one compatible, logged-in official Claude Code or Codex CLI |

Windows, WSL, Fish, PowerShell, and other shells are outside the MVP compatibility contract.

For the native ZLE and Bash 4.0+ Readline paths, a contract-compatible terminal environment must provide an interactive controlling PTY, deliver the configured rewrite and undo sequences plus CR or LF and `Ctrl+C`, support ordinary shell-editor repaint and resize, and keep the shell process alive for any claimed tmux reattach journey. `intent-sh` never selects code by terminal brand, `TERM`, `TERM_PROGRAM`, tmux, or SSH client identity. The existing Bash 3.2 ble.sh path remains separately qualified with fixed `Alt+G`/`Alt+U` bindings; configurable chords and `doctor --keys` qualification apply to native ZLE/Readline only.

Named terminal applications are not inferred to be qualified merely because they allocate a PTY. Follow [the terminal qualification guide](docs/terminal-qualification.md) and consult the dated record before making a named support claim.

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
# or:
intent-sh setup bash
```

The command identifies the likely startup file, reports default-key conflicts, and prints this activation line without editing anything:

```sh
eval "$(intent-sh init zsh)"
```

Use `bash` instead of `zsh` for Bash 4.0 or newer. Add the printed line to the reported startup file, then open a new shell session. Loading is idempotent within a session, and an adapter/binary protocol mismatch fails before installing bindings.

For Bash 3.2, ble.sh is an optional dependency that you install and manage yourself. `intent-sh` does not download, source, update, configure, or remove it. Use the [official ble.sh project](https://github.com/akinomyoga/ble.sh) to install the exact tested commit, verify that it reports `BLE_VERSION=0.4.0-nightly+d69e4d5`, and place the two activation steps in this order:

```bash
# Use the actual path chosen for the exact tested ble.sh build.
source "/path/to/pinned/ble.sh"
eval "$(intent-sh init bash)"
```

ble.sh must be attached, not merely loaded, before the second line runs. Existing `ble-bind` customizations for `M-g`/`M-u` or an `accept-line` advice must be resolved first; initialization refuses conflicts without partially replacing them. If you do not want this external editor dependency, use stock Zsh on macOS or install Bash 4.0+ separately without replacing the system shell. The detailed version/API record is in [docs/blesh-compatibility.md](docs/blesh-compatibility.md).

### 4. Diagnose readiness

```sh
intent-sh doctor
```

Doctor prints stable `PASS`, `WARN`, `FAIL`, and `SKIP` check IDs for the platform, architecture, shell version, active editor backend, adapter protocol, config, static and runtime key conflicts, provider executable, compatible features, and official login readiness. When ble.sh is selected it separately reports its exact version, required APIs, attachment, and load order. Ready Bash 3.2 passes only when the adapter reports the tested `blesh` backend; missing ble.sh and impossible native Readline reports fail actionably. Doctor never sources startup files and never prints buffers, generated commands, binding bodies, tokens, or credential-file contents.

To test actual key delivery from the controlling terminal without invoking a provider, run the explicit interactive mode and press the four requested keys:

```sh
intent-sh doctor --keys
```

The probe temporarily opens `/dev/tty` in bounded raw mode, checks the configured rewrite and undo chords, Enter (CR or LF), and `Ctrl+C`, then restores the original mode on success, mismatch, timeout, cancellation, or read failure. It does not consume piped stdin. A failure means a terminal, tmux layer, or shell mapping intercepted or transformed a sequence; it does not change that mapping for you.

### 5. Try a harmless rewrite

At a fresh shell prompt, type a plain-language intent but do not press Enter:

```text
show the current directory
```

Press the configured rewrite chord (`Alt+G` by default). A successful request replaces the full editable line with one command, such as `pwd`, and moves the cursor to the end. Read the command first. It has not run. Press Enter only if you want your shell to accept it.

## Editing workflow

- The configured rewrite chord (`Alt+G` by default) rewrites the complete current buffer.
- The rewrite chord again, while the generated command is unchanged, requests a different alternative from the preserved original intent.
- The configured undo chord (`Alt+U` by default) restores the original buffer only while the generated command is unchanged. It will not overwrite manual edits.
- `Ctrl+C` during “generating” cancels the provider process tree, stops fallback, and keeps the original buffer.
- Editing a generated command makes it ordinary user-owned shell input, starts a new rewrite chain at the next adapter action, and clears any detected dangerous-command confirmation state.

A clarification, timeout, cancellation, malformed provider response, invalid shell command, or stale response leaves the editable buffer unchanged.

On Bash 3.2, ble.sh owns interactive editing and the same provider call remains synchronous. Older Bash can redraw and process large buffers more slowly than modern Bash or Zsh; this does not permit history capture, clipboard access, terminal-screen scraping, comment-and-reprompt tricks, or simulated keystrokes as a fallback.

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

No file is created until `config set` is used. Defaults are auto routing, Claude then Codex priority, a 30-second timeout, no forced model, rewrite `alt+g`, and undo `alt+u`.

```sh
intent-sh config path
intent-sh config show
intent-sh config set provider codex
intent-sh config set provider auto
intent-sh config set priority codex,claude
intent-sh config set timeout_seconds 45
intent-sh config set model '<provider-supported-model>'
intent-sh config set rewrite_key ctrl+x
intent-sh config set undo_key alt+u
```

Supported config keys are `provider`, `priority`, `timeout_seconds`, `model`, `rewrite_key`, and `undo_key`. Credential fields and unknown keys are rejected. In `auto` mode, providers are attempted sequentially in priority order; cancellation never falls back. Selecting `claude` or `codex` explicitly disables fallback.

Binding values are canonical lowercase `alt+<key>` or `ctrl+<letter>` strings. `alt+` accepts exactly one non-whitespace printable ASCII character. `ctrl+` accepts one ASCII letter except `c`, `d`, `j`, `m`, `q`, `s`, `y`, and `z`, which are reserved for cancellation, EOF, Enter, flow control, or suspension. Shift, Command/Super, function keys, non-ASCII keys, raw escapes, whitespace, and multi-key sequences are not supported. Rewrite and undo must be distinct.

After changing a chord, run `intent-sh setup bash|zsh`, open a new shell, and run `intent-sh doctor --keys`. An already initialized adapter refuses a different pair so it cannot leave stale bindings active. Restore the defaults with:

```sh
intent-sh config set rewrite_key alt+g
intent-sh config set undo_key alt+u
```

If a terminal or tmux binding intercepts an Alt chord, either change that external binding manually or choose an allowed Ctrl alternative. `intent-sh` never edits terminal or tmux settings. Custom chords require native ZLE or Bash 4.0+ Readline; the optional ble.sh compatibility path retains fixed `Alt+G`/`Alt+U` behavior.

## tmux and SSH

tmux needs no `intent-sh` plugin. When the outer terminal and tmux deliver the configured bytes, the adapter behaves as it does on a direct PTY. Rewrite, undo, and dangerous-confirmation state live only in the shell process, so they survive detach/reattach to that same live pane and are not shared with another pane or session. `intent-sh` never runs `capture-pane`, reads scrollback, or modifies tmux configuration.

If `doctor --keys` fails only inside tmux, inspect root-table bindings such as `tmux list-keys -T root` yourself. Remove or remap the conflicting tmux binding in your own configuration, or set a different allowed `intent-sh` chord and start a new shell. The repository's automated tmux suite uses a private `-L` server and an empty config; it never contacts the user's normal server. Detailed checks are in [the terminal qualification guide](docs/terminal-qualification.md).

Inside SSH, everything that serves the rewrite is remote: the shell adapter, `intent-sh` binary, provider CLI, provider login, current directory, provider process, and generated target command. A provider installed or authenticated only on the client cannot serve the remote shell, and `intent-sh` does not forward provider credentials or requests to a client service. Remote model context contains only `remote: true`; SSH marker values and client terminal identity are excluded.

Losing a plain SSH connection normally terminates or hangs up that remote shell, so no recovery is promised. If the remote shell remains alive in remote tmux, reconnect and reattach to that same pane; surviving state belongs to that remote shell and requires no client-local state. The client and remote host need no shared `intent-sh` files beyond the ordinary terminal byte stream.

## Exactly what providers receive

The structured prompt contains only:

- the complete active buffer and cursor position (protocol 2 represents the cursor as a zero-based UTF-8 byte offset);
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
- terminal application identity, version, selections, or tmux pane contents;
- arbitrary environment variables or SSH marker values;
- project/repository file contents, `.env` files, or Git diffs;
- clipboard data;
- SSH configuration;
- provider tokens, credential files, or other authentication material.

The official provider subprocess receives a small environment allowlist needed to locate its executable, official login storage, locale, certificates, and proxy configuration. Those environment values are not serialized into the model prompt. The CLI runs directly—not through a shell string—in a new empty temporary directory with bounded output and a deadline. `intent-sh` removes that directory after success, failure, timeout, or cancellation.

The adapter protocol also carries bounded local `editorBackend` and `editorVersion` compatibility metadata. Those fields, ble.sh variables, and exported adapter diagnostic markers are excluded from both the model prompt and the provider subprocess environment.

## Trust boundaries

You are trusting:

- the local `intent-sh` binary and the embedded Bash/Zsh adapter you explicitly load;
- when selected, the separately maintained ble.sh code running inside and controlling editing in your interactive Bash process;
- the selected official provider executable, its installed version, user-level configuration, account, and login storage;
- the provider service to process the documented prompt payload;
- your shell and every executable named by a command you choose to accept.

You are not trusting `intent-sh` with provider credentials, automatic command execution, project-file access, persistent rewrite logs, or telemetry. Rewrite state exists only as variables in the current shell session. ble.sh expands the interactive-shell trust boundary but does not enter the provider boundary. A model result is untrusted until it passes the local decoder and safety pipeline, and even a passing command still requires your Enter key.

## Non-goals for the MVP

- automatic command execution or multi-step agents;
- API-key management, OAuth, provider accounts, or a hosted backend;
- Fish, PowerShell, Windows/WSL, Bash 3.0/3.1, plain Bash 3.2, untested ble.sh versions, or shells outside the documented Zsh/Bash backends;
- selected-text rewriting, terminal-screen scraping, clipboard access, or global keystroke simulation;
- reading project files, history, terminal output, or Git state to enrich prompts;
- a desktop UI, daemon, database, telemetry, persistent logs, or command-history store;
- arbitrary raw-byte or multi-stroke shortcuts, command explanations as a separate product surface, or a general shell sandbox.

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

If you use ble.sh, removing the `intent-sh` activation line disables only this integration. Keep or remove ble.sh independently using its own installation guidance; `intent-sh` never deletes it. Likewise, removing ble.sh does not remove the `intent-sh` binary or its optional secret-free config.

## Protocol upgrades and rollback

The current embedded adapters and binary use adapter protocol 2. It adds explicit editor-backend/version fields and standardizes cursor positions as UTF-8 byte offsets. After upgrading, open a new shell or re-evaluate the `intent-sh init` line so the running adapter comes from the same binary. An old protocol-1 adapter paired with a protocol-2 binary—or the reverse—fails closed before a provider call or buffer replacement.

To roll back, first remove `rewrite_key` and `undo_key` from the strict TOML file if the older binary predates those keys; otherwise it will correctly reject the now-unknown fields. Then install the earlier binary and re-evaluate the adapter emitted by that binary in a new shell. Removing the activation line is the complete integration rollback. Neither path changes the user's login shell, terminal/tmux settings, provider login, or independently managed ble.sh installation.

## Development

Run the convenient local checks with:

```sh
make check
```

The merge gate is substantially broader. Required GitHub CI has stable read-only jobs for static integrity, unit and race tests, native Bash/Zsh PTYs, private-socket tmux, stock Bash 3.2 fail-closed behavior, complete ble.sh qualification, ephemeral loopback SSH and SSH-to-tmux, reproducible/cross/native artifacts, executable coverage, and the final `required / aggregate` status. Pull-request matrices fail on a missing prerequisite or unexpected skip; ordinary `make check` remains local-friendly.

The pinned ble.sh fixture is not a shallow checkout. The installer downloads the exact root and `contrib` source archives independently, verifies both checked-in SHA-256 digests, assembles the required gitlink content in a fresh tree, builds the expected version, records source commits/digests/licenses plus the built-script digest, and atomically publishes only a fully validated cache. Network-free adversarial tests cover empty, restored, corrupt, incomplete, symlink, and interrupted publication paths on GNU and macOS tool variants. See [the compatibility contract](docs/blesh-compatibility.md).

For an explicit local ble.sh run:

```sh
export INTENT_SH_TEST_BLESH="$(bash .github/scripts/install-blesh-test.sh)"
make blesh-test BLESH_SUITE=blesh-modern QUALIFICATION_DIR=/absolute/disposable/results
```

The required loopback SSH job owns a temporary localhost daemon, high port, account/home, host/client keys, strict known-host file, and client config; forwarding, passwords, agent/X11, tunnels, user environment, and user rc files are disabled, and an always-run step removes the whole job-owned namespace. A caller-owned external target is never used by pull-request CI. It is available only through protected manual dispatch or the local `external-ssh-test`, requires pre-existing BatchMode authentication and known-host state, creates no credential, and restricts cleanup to its validated remote `intent-sh-ssh.*` directory.

Weekly and manual scheduled qualification adds shuffled/repeated PTY and tmux runs, independent fuzz budgets, checksum-pinned Bash 4.0/5.3 and Zsh 5.8.1/5.9.1 source builds, Ubuntu glibc versions, native Linux arm64, unauthenticated pinned Claude/Codex capability checks without generation, and recorded vulnerability-database checks. Authenticated provider smoke and named GUI/integrated terminal qualification remain protected manual evidence; hosted PTY success never updates a dated named-terminal record.

Common strict targets include:

```sh
make static-check
make native-pty-test QUALIFICATION_DIR=/absolute/disposable/results
make tmux-test
INTENT_SH_TEST_SSH_TARGET=user@known-host make external-ssh-test
```

The complete target/prerequisite/skip/timeout/trust-tier matrix, exact reproduction commands, cleanup ownership, and branch-protection update procedure are in [the development guide](docs/development.md). Terminal and provider manual boundaries remain in the [terminal qualification guide](docs/terminal-qualification.md) and [provider compatibility record](docs/provider-compatibility.md).

## License

Licensed under the [Apache License 2.0](LICENSE) (`Apache-2.0`).
