## Why

`intent-sh` is maintained and qualified from macOS, while its current product contract, implementation branches, evidence, and CI promise Linux support that the maintainer cannot independently reproduce. The repository needs one truthful macOS-only baseline so every support claim is backed by macOS qualification and future work does not carry unowned cross-platform complexity.

## What Changes

- **BREAKING**: Restrict the supported operating-system contract, source/release builds, diagnostics, shell setup behavior, terminal qualification, remote-host qualification, and CI to macOS.
- Preserve macOS `arm64` and `amd64` as build targets, while distinguishing artifacts that were executed natively from artifacts that received format/architecture inspection only.
- Remove Linux artifacts, ELF inspection, Linux startup-file behavior, Linux test fixtures, Ubuntu jobs, distribution matrices, Linux-only capacity claims, and Linux-specific CI tooling paths.
- Run required and scheduled repository qualification on macOS, including static checks, unit/race tests, native PTYs, tmux, artifacts, coverage, fuzzing, provider capability probes, and security checks.
- Retain SSH semantics only for a supported macOS remote host. Remove the Linux-only loopback daemon fixture; keep remote qualification explicit and protected/manual until a hermetic macOS fixture exists.
- Replace the broad Unix terminal capability with an explicit macOS terminal compatibility capability and update current documentation and evidence so they make no non-macOS support claim.
- Make macOS integration tests deterministic about their locale, including the Zsh UTF-8 cursor journey.
- Define repository-scope exclusions for generated third-party lock metadata and macOS implementation identifiers such as Go's `darwin` target and `golang.org/x/sys/unix`.

## Capabilities

### New Capabilities

- `macos-terminal-compatibility`: Behavioral PTY, terminal, tmux, key-probe, and macOS-to-macOS SSH qualification contract for supported macOS environments.

### Modified Capabilities

- `installation-diagnostics`: Limit builds, setup discovery, doctor platform readiness, artifacts, and installation guidance to macOS.
- `provider-routing`: Require model platform context and remote context to represent supported macOS environments only.
- `shell-rewrite-workflow`: Limit the Zsh and Bash adapter support contract to macOS while preserving the existing protocol and safety behavior.
- `continuous-integration-qualification`: Replace macOS/Linux and Linux-only gates with a macOS-only qualification graph and evidence policy.
- `unix-terminal-compatibility`: Remove the superseded broad Unix capability after its macOS requirements move to `macos-terminal-compatibility`.

## Impact

The change affects the README and qualification records; OpenSpec baseline capabilities; the Makefile; GitHub required, scheduled, and protected-manual workflows; CI manifests and helper scripts; setup and doctor platform handling; artifact qualification; shell/SSH integration harnesses; platform fixtures in unit tests; and generated release/evidence names. Linux users and Linux remote targets will no longer be supported or qualified. Existing Git commits and archived OpenSpec changes remain historical records and are not rewritten, and generated dependency lockfiles are not manually edited solely to remove upstream platform metadata.
