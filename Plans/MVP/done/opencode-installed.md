# OpenCode installed verification

Historical evidence, 2026-09-13. Current behavior belongs to
[runtime adapters](../../../docs/08-runner-role.md#adapters) and
[smart launchers](../../../docs/08-runner-role.md#smart-launchers).

| Check | Result |
|---|---|
| Runtime | OpenCode 1.18.30, launched as the ordinary user with the existing configured provider |
| Fresh-session defect | Initial TUI startup created a session without the selection event assumed by the fixture; MCP connected but the inbox had no reader |
| Fix | Create a fresh session through the runtime, then bind the reader and attach the actual TUI to the same explicit ID |
| Live exchange | An external test principal sent a bus message to the attached fresh session; it completed `ab_ls` and returned `OPENCODE_READY` through `ab_send` with the matching topic and tag |
| Installed entry point | Shared stamped build 0.5.10; `ab-opencode` installed beside the other launchers and resolved from fish |
| Resume | Installed launcher attached its TUI to the same session ID, connected MCP and received a second bus message after the derived-address rename |
| Fish discovery defect | Fish's `opencode` function called the user wrapper, but PATH discovery chose `/usr/bin/opencode`; that process lacked the wrapper's external-skill setting and failed inside `prompt_async` with `TypeError: undefined is not an object (evaluating 'a.name')` |
| Wrapper fix | Prefer the user wrapper under the [executable selection rule](../../../docs/08-runner-role.md#running-the-launchers); verify the actual child received its setting |
| Resumed live exchange | With the corrected bundle launched from fish, the same attached session returned a correlated `OPENCODE_READY`; this diagnostic used the configured small model. The earlier fresh-session exchange used the default model; global model configuration was unchanged |
| Endpoint authentication | An unauthenticated request from a second OS account received HTTP 401 |
| Automated verification | `src/smoke.sh --slow`: 529 passed, zero failed; TypeScript check passed |
| Mutations | Removing the explicit TUI session argument failed the installed-launcher smoke; bypassing the user wrapper failed the executable-discovery test. Both mutations were exercised through the slow suite |

This does not close the full cross-account, crash-recovery or fresh-host
matrices in [MVP acceptance](../TODO.md#remaining-work).
