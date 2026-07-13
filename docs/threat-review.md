# MVP threat review

Review date: 2026-07-13

This review covers the source-installable MVP described by the `build-intent-sh-mvp` OpenSpec change. The security objective is narrow: provider output must remain inert text until it has crossed the local framing, decoding, command-policy, syntax, and risk checks, and a shell must never accept a generated command without a deliberate user Enter.

## Reviewed boundaries

| Area | Controls and review result |
| --- | --- |
| Shell framing | Requests and responses use fixed-order NUL framing with exact field counts, per-field and whole-frame bounds, protocol negotiation, fully buffered responses, and request-ID matching. Adapters assign the validated replacement as editor text; they do not `eval`, source, or execute it. |
| Provider subprocess | Official CLIs are invoked directly with argument arrays and prompt stdin, never through a shell. Each attempt uses a new private empty directory, an allowlisted environment, bounded output, a deadline, a separate process group, descendant cleanup, and removal on every exit path. Result files are opened nonblocking with no symlink following and must be regular files. |
| Output parsing | Provider JSON must be bounded, valid UTF-8, contain no duplicate object keys, contain exactly one value, and match one strict local `ok` or `clarify` branch with no unknown result fields. Claude's outer envelope is independently checked for duplicate keys before its structured result is decoded. |
| Command policy | Input is limited to one line and 8 KiB. The AST policy accepts one simple command or pipeline and recursively checks command/process substitutions; it rejects lists, background jobs, compounds, functions, comments, heredocs, and here-strings. Bash or Zsh then parses the text in no-execution mode with startup files disabled. |
| Risk rules | Every top-level and nested stage is considered. `sudo`, `env`, `command`, and `builtin` normalization has no fixed wrapper-depth cutoff. Environment changes, privilege, and explicit executable paths raise a result to at least review. Opaque `env -S`, download-to-shell through pipelines or process substitution, recursive/privileged deletion, raw disks, destructive Git/database operations, and shutdown patterns are dangerous. Provider hints can only raise local risk. |
| Enter confirmation | Generation never executes the target. Safe/review results retain native one-Enter acceptance. A dangerous generated result requires a warning Enter followed by a second Enter while the exact generated text is still present. Rewrites, undo, cancellation, stale IDs, accepted commands, and a buffer that differs at a guard check clear the armed state. Bash resets its private macro continuation before every rewrite so a prior acceptance cannot bypass a new danger guard. |
| Config and setup | Config is strict, secret-free TOML at an absolute XDG-derived path. Reads are bounded, nonblocking, and regular-file-only. Writes use a private temporary file, `0600`, sync, and atomic rename. Setup only performs a bounded nonblocking read of a regular startup file and prints reversible guidance; it never executes or modifies the file. |
| Diagnostics | User-visible errors use curated messages passed through terminal-control removal and UTF-8-safe byte bounds. Raw prompts, model output, provider stderr, credentials, and arbitrary subprocess errors are not rendered. Doctor uses stable check IDs and the same bounded rendering. |

## Findings closed during review

| ID | Finding | Resolution and regression evidence |
| --- | --- | --- |
| TR-01 | Go's default JSON behavior accepts duplicate keys and invalid UTF-8 replacement. | Reject invalid UTF-8 and duplicate keys recursively before decoding; cover result and Claude-envelope ambiguity in provider tests. |
| TR-02 | A provider could replace its result with a symlink or FIFO between inspection and open. | Open once with `O_NOFOLLOW` and `O_NONBLOCK`, then inspect the descriptor and accept only a regular file; test symlink and FIFO rejection. |
| TR-03 | A FIFO at a startup or config path could block a read-only command indefinitely. | Open nonblocking, inspect the opened descriptor, bound reads, and reject non-regular files; test prompt rejection. |
| TR-04 | Inline assignments and `env` could change `PATH` while a known basename remained classified safe. | Treat leading assignments and every normalized `env` layer as review; add direct and deep-wrapper cases. |
| TR-05 | `sudo ls` could inherit the read-only safe classification. | Any normalized privileged execution is at least review; destructive privileged operations remain dangerous. |
| TR-06 | `curl ... > >(sh)` and its `wget` equivalent bypassed the pipeline-only download-to-shell rule. | Relate top-level downloads to nested process-substitution shells and classify both forms as dangerous. |
| TR-07 | Bash could leave its private macro continuation mapped to `accept-line` after an earlier safe command. | Reset the continuation to a no-op before and after every rewrite; a PTY regression presses the private key directly against a newly generated dangerous marker command. |
| TR-08 | `/tmp/ls` inherited the `ls` safe rule even though the explicit path could name arbitrary code. | Explicit executable paths are at least review, including commands embedded in `find -exec`. |
| TR-09 | Eight or more supported wrappers could hide a destructive inner command. | Normalize supported wrappers until no wrapper remains; a 32-layer regression must still find recursive deletion. |
| TR-10 | `env -S` can turn one opaque string into a hidden command. | Treat split-string forms as dangerous shell evaluation, including attached long-option syntax. |

## Residual limits

- The local classifier is a heuristic, not a shell sandbox. Aliases, functions, the pre-existing `PATH`, shell options, executable versions, unsupported wrappers/interpreters, filesystem state, and remote targets can change behavior. Unknown or obscured operations are at least review, but only recognized dangerous structures receive the two-Enter guard.
- Bash Readline and ZLE expose the current buffer at widget callbacks, not a durable edit-generation counter. If text is changed and then restored byte-for-byte between guard callbacks, the adapter can only observe the final exact value. It never accepts a different command under the old fingerprint, but it cannot prove that no transient edit occurred.
- The selected provider CLI is a trusted local executable running as the user. CLI isolation flags, the empty working directory, and the environment allowlist reduce accidental access; they are not an OS sandbox against a compromised provider binary. The CLI retains the home/config access needed for its official login and network service.
- A manual edit makes the line user-owned shell input. The adapter does not re-run the provider classifier on every keystroke, so ordinary shell acceptance applies after the generated text differs. The visible line and the user's Enter remain the final authority.

These limits are also reflected in the README trust-boundary and risk sections. They do not weaken the no-auto-execution invariant: neither the model, provider CLI, Go binary, nor adapter accepts the target command on the user's behalf.
