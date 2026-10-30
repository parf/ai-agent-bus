# macOS

📌 **TL;DR:** macOS comes in three steps, cheapest first: the client and the runner without a sandbox, then the daemon with a reduced setup, then a sandbox only if the owner asks. A sandbox request on macOS is refused, never silently ignored. Test host, signing and the full-set direction are in [distribution § macOS](distribution.md#macos).

## Order

Owner, 2026-10-30 ([RD166](DECISIONS.md#recorded-decisions)):

| Step | Ships | Without |
|---|---|---|
| 1 Client and runner | CLI, runner, MCP face and launchers, the client edition for darwin-arm64 | any sandbox |
| 2 Daemon | the daemon with a reduced setup for macOS | the full Linux setup (systemd units, per-role accounts, cgroup limits) |
| 3 Sandbox | an equivalent of the Linux sandbox | deferred; built only if the owner asks |

## Rules without a sandbox

| Rule | Meaning |
|---|---|
| Refused, not ignored | a sandbox request on macOS fails with the reason, as the [runner contract](../../docs/06-dedicated-runner.md#sandboxing) already requires of any unavailable sandbox; nothing runs unsandboxed after asking for one |
| Dedicated account | the runner runs scripts as its own unprivileged account, never as an administrator or the owner |
| Said plainly | the docs say agent scripts then hold that account's rights, and that this suits one owner's own Mac, not a shared one |
