# Scratch files and cleanup

📌 **TL;DR:** Where throwaway files go, and when to remove them. The every-session rule is [CLAUDE.md § scratch files](../../CLAUDE.md#scratch-files); this page holds the reason, the full cleanup table and the one exception.

## Why

On 2026-10-19 `tmp/` reached 164 GB and filled the root disk: 430 worktrees, none removed, each with its own 1.3 GB Rust build and test output. Gates failed with `no space left` and the daemon's database was at risk.

## Clean up after yourself

| When | Do |
|---|---|
| Your commit lands, or your review or report is sent | `git worktree remove --force tmp/<yours>`, test output included; remove loose `tmp/` files you wrote for it |
| You hand work to another agent | it removes the worktree once it lands, and the hand-off message says so |
| You run Rust | the shared `tmp/cargo-target` only, never a target in your worktree (`cargo clean` one you find) |
| Before a full gate | `df -h /`: under 30 GB free, clean your own finished `tmp/` first and tell Claude B |
| End of a task or session | `git worktree list` and `ls tmp`: nothing of yours remains except work still in progress |

- Keep evidence a review needs in the commit message, the report or a plan's `done/` file, never in `tmp/`.
- A daemon backup lives only in `/var/lib/agent-bus/daemon.backup/`.
- Remove only what is yours; another agent's live worktree is its work in progress.
- `.gitignore` covers every scratch home; the point is to use them rather than to rely on it.

## Not /tmp

`/tmp` is shared with every other process on the box, it is not cleaned up on any schedule anybody controls, and a name collision there is somebody else's afternoon.

Exception: the smoke driver and shards use private mktemp directories with EXIT cleanup traps. A sandbox-isolation fixture whose home must sit outside the sandbox's read-only binds (/usr) likewise uses a private mktemp directory with a cleanup trap.
