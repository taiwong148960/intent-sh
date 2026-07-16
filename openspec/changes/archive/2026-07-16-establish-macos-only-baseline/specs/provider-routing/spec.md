## MODIFIED Requirements

### Requirement: Apply safety-biased model instructions
The provider prompt SHALL require conversion of intent into one command, prohibit execution and file inspection, prohibit adding `sudo` unless explicitly requested, prefer tools available on macOS and in the supplied command allowlist, preserve existing shell fragments when possible, and request a non-destructive preview for ambiguous destructive intent.

#### Scenario: Ambiguous deletion intent
- **WHEN** the input asks to delete old log files without an explicit scope and immediate-execution instruction
- **THEN** the model is instructed to return a preview command that lists matching files instead of deleting them

#### Scenario: Explicit macOS platform context
- **WHEN** a rewrite is requested from a supported macOS environment
- **THEN** the prompt identifies the Darwin platform, architecture, and shell so the provider can avoid incompatible command syntax
