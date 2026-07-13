# Provider CLI compatibility

The real-provider smoke test is opt-in because it contacts the configured provider service. It runs from a disposable directory, logs only the CLI name and bounded version, and does not record prompts, generated commands, raw model output, environment values, or credentials.

Run one or both providers with:

```sh
INTENT_SH_REAL_PROVIDER_SMOKE=codex go test ./internal/smoketest -run '^TestRealProviderSmoke$' -count=1 -v
INTENT_SH_REAL_PROVIDER_SMOKE=claude,codex go test ./internal/smoketest -run '^TestRealProviderSmoke$' -count=1 -v
```

## Verified versions

| Date | Platform | Provider CLI | Version | Result |
| --- | --- | --- | --- | --- |
| 2026-07-13 | macOS arm64 | Codex CLI | 0.144.3 | Pass: capability/login probe, structured generation, and local read-only validation |
| 2026-07-13 | macOS arm64 | Claude Code | Not installed | Not run; requires a compatible installed and officially logged-in `claude` CLI |

The Codex run identified and fixed a compatibility issue: its structured-output endpoint rejects a root-level `oneOf`. The transport schema now uses one closed object with required nullable branch fields, while the local decoder continues to reject missing, mixed, or malformed `ok`/`clarify` branches.
