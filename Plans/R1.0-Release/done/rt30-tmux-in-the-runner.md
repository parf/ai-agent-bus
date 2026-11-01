# RT30 `#tmux-sessions` in the person's runner (cancelled)

📌 **TL;DR:** Cancelled 2026-10-27: the person's runner runs as its own account, so it cannot reach their tmux server. [RT31](../TODO.md#runner-waves) (a person's user-account runner) replaced it and is built.

## `#tmux-sessions` runs under the person's runner, not as a standalone service (owner, 2026-10-27: "any reason why this is not a part of Runner?" → "yes")
  - A kept agent of the personal runner with `"sandbox": "off"`, declared and documented as such: it needs the person's tmux server, `systemd-run --user --scope`, their HOME logins and network, and windows that outlive any call
  - Windows already leave the agent's cgroup through `systemd-run --user --scope`; reload, stop or a crash of the agent never touches them
  - The main and the `ab-dvp` instance move with their Configuration, ACL and owner; their standalone units are removed; a second instance is a second runner record
  - Acceptance: both instances answer help and status through the runner; `reload` and `stop` of the agent leave every tmux window and its pid unchanged; no standalone tmux-sessions unit remains
  - Cancelled 2026-10-27: the person's runner runs as its own account (`parf-ab-runner`), not as the person, so it cannot reach their tmux server, user manager or logins without crossing the process boundary. `#tmux-sessions` stays a standalone service run as the person
