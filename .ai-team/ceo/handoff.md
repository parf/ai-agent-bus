# #ab-ceo handoff — 2026-10-30 ~16:30

Written by the account-1 #ab-ceo session before the move to Claude account 2 (owner's request). Read this first, then `.ai-team/ceo.md` (role), `CLAUDE.md`, `CLAUDE.local.md`. `.ai-team/ceo/tickets.md` is stale (morning); this file supersedes it.

## Standing owner rules (2026-10-30)

| Rule | Detail |
|---|---|
| Do it yourself | "do all yourself": deploys, runner/web restarts, window restarts, push. Ask the owner only what you cannot do (decision, password, account, money, GUI). Check facts yourself first (ssh, status, git). |
| Push | #ab-ceo pushes main on #ab-dir's "push-ready <sha>": verify VERSION, changelog head squashed above the last pushed entry, no conflict markers, origin/main ancestor (ff only), then `git push origin <sha>:main`. Never ask the owner. Row in CLAUDE.local.md. |
| Deploy | Only on #ab-dir's "deploy-ready <sha>" (gate idle, leads holding). `sudo src/git-install.sh` (restarts the daemon); `sudo systemctl restart agent-bus-runner@parf` if runner code changed; `sudo systemctl restart agent-bus-web` if src/web changed; `src/check-deploy.sh`; wait up to 90 s for `#ab-gate` readers 1 (it reconnects by itself, J33: within 10 s) — restart the GATE window only if gate.sh is dead; check every #ab-* and the agents below at readers 1; report "done" to #ab-dir so it releases the leads. |
| Agent package deploy | Package into an EMPTIED agent.d/<name> (overlay left stale modules → imap crash-loop 13:13–15:17, J46). After it, check every instance. #ab-ceo is now Maintainer of #runner/parf: `agent-bus call '#runner/parf@parf.us' reload` works. |
| Workers | W1A and W2A out until 2026-11-05 (owner, 11-01: "no WXA workers out for 4 days"): B and C workers only. |
| Windows client | Parked until the R1 release (owner, 11-01: "no, not until R1 release"). Plan: Plans T-15 221b2551 (Go ~43d vs bun ~8.5d, Q268 open). |
| Window restarts | Leads cannot; they go #lead → #ab-dir → me: `agent-bus call '#tmux-sessions/ab-dvp@parf.us' 'restart <WIN>'`, then message the worker its identity, role file and checkpoint. J34 (live): a restart resumes the window's own transcript. J40 (pending): `--fresh`. |
| Mac | `ssh mac` = parf-air.local, macOS 26.5.2 arm64, passwordless sudo (owner enabled). Go 1.27 + bun via Homebrew; podman 5.4.2 machine running (owner's el10-php85 images: never touch). No heavy tests there; no disk clutter, only ~/agent-bus-dev (one checkout); sudo only for MAC.2 install acceptance, then uninstall. |
| Syslog | Tests never write to the host journal (T-12 done). |
| Docs | WRITING.md rules; owner reviews only on request now; #ab-dir lands docs batches on a clean non-author read. Review links: http://localhost:7777/ai-agent-bus/<path>, checked 200. |
| Scope | Discord → R1.1 (being moved, #ab-w1b); Google Chat → R2.0; Teams/WhatsApp/Messenger R2.0. |

## State at handoff

- Pushed and deployed: 0.9.211 on origin (7c6555aa). Deployed on node: 0.9.214 (0654815c), 16:17. Unpushed: 0.9.212–0.9.214 (next fold).
- Live agents to check after a deploy: `#imap/{scienteco,gmail,eprofit,difive}@parf.us`, `#slack@parf.us`, `#telegram/parf@parf.us`, `#at@parf.us`, `#cron@parf.us`, `#eventlog@parf.us`, `#logwatch@parf.us` (standalone unit), all readers 1. `#discord@parf.us` registered, configured (guild 860203799858642966 DI5, default channel 860203799858642969), no token, to be disabled (R1.1).
- R1 estimate: 2026-11-04 12:00 (excl. owner-blocked). Critical path: runner chain M.6a share (#ab-w2b, 11-01) → M.8 (#ab-w1b, 11-03 12:00); MAC.1 (#ab-w1b) and MAC.2 (#ab-w1c) 11-01 18:00.
- Owner decisions today: M.3a jsonl answer line {"message_id","body"} (empty = done); M.6a host on consume, 2-min in-memory; M.8 code from bundled name + git URL pinned to full commit; Q260 asks-only consume param; J27 cap 60 s; #eventlog kept + standing row widened; Q226 repo public from the start, Actions release path RD176.
- Open with the owner (not urgent): release signing keys (RD171), RT22 #bus setup page proposal, M.7a, Q165 edge runners, npm account (P.1), RT31 ops switchover, B.15 (observe till 11-06).
- In flight: T-13 per-window start dirs (ab-w1b/T-13 5e145840, in review). After it lands, apply per window at that window's next restart: `python3 .ai-team/windows/windows.py apply --window <bus name>`, then push the config (below) and restart. J56 runner shows standalone agents as refused (#ab-w2c). J51, J47, B.26, B.29 cron/#at (#ab-w2b), RT24 (#ab-w2c), RT25 (#ab-w2b).
- Window config: the #tmux-sessions record's Configuration (parf's, private). Source file `.ai-team/ab-dvp-v2-config.json`; apply as the owner: `env -u AGENT_BUS_TOKEN -u AGENT_BUS_NAME agent-bus config '#tmux-sessions/ab-dvp@parf.us' - < .ai-team/ab-dvp-v2-config.json`. Owner credential = same shell with AGENT_BUS_TOKEN/NAME unset.
- Lessons: deploys restart the daemon — never while a gate runs; check-deploy + readers after every deploy; my 13:13 imap overlay deploy broke mail reads for 2 h (no mail lost; agents are manual-cache).

## Memory

Account-1 memory: `~/.claude/projects/-usr-local-src-ai-agent-bus/memory/` (copied into account 2's same project memory, existing files kept). Index MEMORY.md.
