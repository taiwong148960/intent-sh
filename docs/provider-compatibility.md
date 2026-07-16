# Provider CLI compatibility

Provider evidence has three separate trust tiers. Required CI uses deterministic fake executables to cover unavailable binaries, feature/login failure, timeout and fallback, malformed or excessive output, crashes, explicit-provider no-fallback, cancellation, process-tree reaping, and temporary-workspace cleanup. Scheduled CI installs the exact integrity-locked `@openai/codex` 0.144.4 and `@anthropic-ai/claude-code` 2.1.210 packages into an empty home, verifies the required flags, and requires the expected login-not-ready result; it never generates a model request or reads a credential.

The real-provider smoke remains opt-in because it contacts the configured provider service. It runs only locally on macOS or in the manually dispatched `Trusted Manual Qualification` workflow on a protected `[self-hosted, macOS, intent-sh-trusted]` runner and `trusted-qualification` environment. It uses the runner's existing official login, runs from a disposable directory, and emits only provider name, bounded compatible version, and pass/fail through the structured auditor. No authenticated log artifact is uploaded.

Run one or both providers with:

```sh
INTENT_SH_REAL_PROVIDER_SMOKE=codex make real-provider-test
INTENT_SH_REAL_PROVIDER_SMOKE=claude,codex make real-provider-test
```

## Verified versions

| Date | Platform | Provider CLI | Version | Result |
| --- | --- | --- | --- | --- |
| 2026-07-13 | macOS arm64 | Codex CLI | 0.144.3 | Pass: capability/login probe, structured generation, and local read-only validation |
| 2026-07-13 | macOS arm64 | Claude Code | Not installed | Not run; requires a compatible installed and officially logged-in `claude` CLI |

The Codex run identified and fixed a compatibility issue: its structured-output endpoint rejects a root-level `oneOf`. The transport schema now uses one closed object with required nullable branch fields, while the local decoder continues to reject missing, mixed, or malformed `ok`/`clarify` branches.

The scheduled unauthenticated probe is capability evidence only and does not refresh this dated authenticated table. Likewise, a deterministic fake-provider pass does not claim a real provider version works. Add a new dated row only after the bounded protected smoke actually runs; never paste prompts, generated commands, provider output, account identifiers, home/config paths, or credentials into this file.
