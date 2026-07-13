## ADDED Requirements

### Requirement: Decode provider output strictly and within bounds
The safety boundary SHALL accept exactly one schema-valid result and SHALL reject unknown fields, trailing non-whitespace content, NUL characters, Markdown fences, unexpected CR/LF in a command, an empty command, or a command longer than 8 KiB.

#### Scenario: Provider wraps a command in Markdown
- **WHEN** an otherwise valid command contains a Markdown code fence
- **THEN** validation rejects the result and no replacement reaches the adapter

#### Scenario: Provider returns a multiline script
- **WHEN** the command contains a carriage return or newline
- **THEN** validation rejects the result before syntax or risk classification

#### Scenario: Provider returns excessive command text
- **WHEN** the decoded command exceeds 8 KiB
- **THEN** validation rejects it with a bounded error

### Requirement: Permit only one simple command or pipeline
The system SHALL structurally parse the command and permit one top-level simple command or pipeline with ordinary redirections. It MUST reject command lists, multiple statements, background jobs, compound statements, function definitions, heredocs, and nested substitutions containing multiple statements.

#### Scenario: Accept a read-only pipeline
- **WHEN** the result is `find . -type f -exec du -h {} + | sort -rh | head -n 10`
- **THEN** structural validation accepts it for subsequent shell syntax and risk checks

#### Scenario: Reject chained commands
- **WHEN** the result contains two commands joined by `;`, `&&`, or `||`
- **THEN** structural validation rejects it as more than one allowed statement

#### Scenario: Reject a heredoc or function
- **WHEN** the result defines a heredoc, shell function, loop, conditional, or other compound form
- **THEN** structural validation rejects it even if the target shell could parse it

### Requirement: Validate target-shell syntax without execution
After structural parsing, the system SHALL ask the declared target shell to perform a no-execution syntax check with startup files disabled. A syntax check MUST NOT execute substitutions or any part of the generated command.

#### Scenario: Valid target-shell syntax
- **WHEN** a structurally allowed command passes `bash` or `zsh` no-execution parsing for the active shell
- **THEN** it proceeds to local risk classification

#### Scenario: Invalid target-shell syntax
- **WHEN** the target shell reports a parse error
- **THEN** validation fails and the shell buffer remains unchanged

### Requirement: Derive risk locally and conservatively
The system SHALL assign `safe`, `review`, or `dangerous` using deterministic local rules over parsed commands, normalized wrappers, arguments, pipelines, and redirections. A provider risk hint MUST NOT lower the local classification, and an unknown or dynamically obscured operation MUST default to at least `review`.

#### Scenario: Model understates risk
- **WHEN** a provider labels `rm -rf ./build` as safe
- **THEN** the local engine classifies it as dangerous and reports the recursive-delete reason

#### Scenario: Unknown executable is generated
- **WHEN** no local safe or dangerous rule can characterize the executable
- **THEN** the engine classifies the command as review rather than safe

#### Scenario: Wrapper hides a command name
- **WHEN** a command is prefixed by a recognized wrapper such as `sudo`, `env`, `command`, or `builtin`
- **THEN** classification examines the effective command and never treats the wrapper alone as proof of safety

### Requirement: Recognize MVP risk baselines
The rule set SHALL classify known read-only operations such as `ls`, `find`, `grep`, `rg`, `lsof`, `ps`, `git log`, and `docker ps` as safe when no riskier pipeline stage or redirection is present. It SHALL classify state-changing operations such as `mv`, potentially overwriting `cp`, `kill`, `chmod`, `chown`, `git reset`, and `docker rm` as review. It SHALL classify recursive forced deletion, privileged deletion, destructive raw-disk tools, download-to-shell pipelines, destructive Git cleanup/reset, shutdown/reboot, and destructive database statements as dangerous.

#### Scenario: Read-only inspection command
- **WHEN** all executable stages and redirections match a known read-only pattern
- **THEN** the engine returns safe

#### Scenario: State-changing command
- **WHEN** a command invokes `kill`, changes permissions, moves data, resets Git state, or removes a container without a dangerous pattern
- **THEN** the engine returns review with a stable reason code

#### Scenario: Known destructive command
- **WHEN** a command matches `rm -rf`, `sudo rm`, `dd`, `mkfs`, `fdisk`, `curl` or `wget` piped to a shell, `git clean -fdx`, `git reset --hard`, destructive SQL, `shutdown`, or `reboot`
- **THEN** the engine returns dangerous with a stable reason code and a concise specific warning

### Requirement: Render risk without changing the proposed target command
The adapter SHALL insert the validated command itself. Safe results SHALL not add a risk warning, review results SHALL show a visible review warning without blocking normal acceptance, and dangerous results SHALL show a prominent warning and enter the guarded-confirmation state.

#### Scenario: Review command is inserted
- **WHEN** local classification is review
- **THEN** the command remains editable, a review warning is shown, and one deliberate Enter uses the shell's normal acceptance behavior

#### Scenario: Dangerous command is inserted
- **WHEN** local classification is dangerous
- **THEN** the unchanged target command remains visible and editable and the adapter marks its exact fingerprint as requiring confirmation

### Requirement: Require two Enters for an unchanged dangerous result
For an unchanged dangerous generated command, the first Enter SHALL NOT execute or accept the target command. It SHALL display the specific risk and arm only that exact command fingerprint. A second consecutive Enter with the same buffer SHALL invoke the shell's normal accept-line behavior.

#### Scenario: First Enter warns
- **WHEN** a dangerous generated command is visible and unarmed and the user presses Enter
- **THEN** the target command does not run, the line stays editable, and the adapter displays a second-Enter warning

#### Scenario: Second Enter accepts
- **WHEN** the same dangerous command remains unchanged and armed and the user presses Enter again
- **THEN** the adapter delegates to the shell's native accept-line behavior

#### Scenario: Safe command uses normal acceptance
- **WHEN** a generated command is not dangerous and the user presses Enter
- **THEN** the adapter does not introduce an additional confirmation press

### Requirement: Disarm confirmation whenever the command changes
Any buffer edit, rewrite, regeneration, undo, cancellation, accepted command, or request-ID change SHALL clear a dangerous command's armed state. Confirmation state MUST be scoped to the current shell session and exact command fingerprint.

#### Scenario: Edit after the first warning
- **WHEN** the user changes the command after the first dangerous warning and then presses Enter
- **THEN** the previous fingerprint is not considered confirmed

#### Scenario: Generate another command after warning
- **WHEN** the user rewrites, regenerates, or undoes after arming a dangerous command
- **THEN** the old confirmation state is removed before the new buffer state is applied

### Requirement: Describe safety limits honestly
User-facing documentation and diagnostics SHALL state that local classification reduces accidental risk but is not a sandbox or guarantee. The system MUST NOT describe a `safe` classification as proof that a command cannot cause harm.

#### Scenario: User reads safety documentation
- **WHEN** installation or usage guidance explains risk levels
- **THEN** it explains local heuristic limits and that the user remains responsible for reviewing and accepting the command
