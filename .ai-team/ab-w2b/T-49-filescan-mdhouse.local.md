# T-49 part 1: `#filescan/mdhouse@parf.us` on `#runner/parf-user@parf.us`

Written by #ab-w2b, read only, nothing run. `[ext]` = an external user, on its own server and own user-account runner ([setup](../../docs/runner/setup.md#a-persons-user-account-runner)). `[parf-here]` = only because today one account is owner, operator and user. Names agreed with #ab-w1b: `#filescan/mdhouse@parf.us`, `#rag/mdhouse@parf.us`, same runner, keys `filescan/mdhouse` and `rag/mdhouse`, plain allow entry.

## Steps

0. [ext] Pre-check as the person: `find /home/parf/src/mdhouse -type s` prints nothing (a socket under a root refuses the bind, bind_dir.go:131; node_modules is inside the root).
1. [ext] Package: no copy. The user runner lists bundled packages (22 installed; configuration.md:64-67: bundled starts only with `enabled:true`). Not verified that filescan resolves there: if step 6 shows it missing, report it as a gap.
2. [ext] Inventory, merge into `~/.local/state/agent-bus/runner/agents.json`, keeping existing keys: `"filescan/mdhouse": {"enabled": true, "roots": ["/home/parf/src/mdhouse"]}`. Sandbox key omitted = on. `roots` is the node owner's decision (configuration.md:222, security.md:46).
3. [ext] Register as Owner, then grant: `agent-bus register '#filescan/mdhouse@parf.us' --kind agent --allow '@owner'`; `agent-bus manage '#filescan/mdhouse@parf.us' --add-to-set-maintainers '#cx/mdhouse-cto'`. Maintainer only, no allow entry. [parf-here] the `parf.us` realm and the Owner being parf.
4. [ext] Configuration: `agent-bus config '#filescan/mdhouse@parf.us' '{"dir":"/home/parf/src/mdhouse","include":["*.md"],"exclude":["node_modules/**","*/node_modules/**",".*","*/.*"],"namespace":"mdhouse","table":"docs"}'`. An explicit `exclude` replaces the dotfile defaults (settings.py:63), so they are listed again.
5. [ext] Token, then reload (create-agent-prompt.md:131-146): `umask 077; agent-bus token '#filescan/mdhouse@parf.us' > ~/.local/state/agent-bus/runner/tokens/'%23filescan%2Fmdhouse@parf.us'`; `agent-bus call '#runner/parf-user@parf.us' reload`.
6. [ext] Check: `journalctl --user -u agent-bus-runner` shows "serving #filescan/mdhouse@parf.us" and 0 refused; `agent-bus call '#runner/parf-user@parf.us' status` has no `needs.dir` refusal; `agent-bus call '#filescan/mdhouse@parf.us' status` shows `effective_mode` and no `paused`; `ls` shows hosts [parf.us].
7. [ext] After `#rag/mdhouse@parf.us` exists (W1B step 3), the owner admits its own sender: `agent-bus manage '#rag/mdhouse@parf.us' --add-to-set-allow '#filescan/mdhouse@parf.us'` (plain; RAG declares no roles). No `sub add`, no scan, no ingest: `sub add` starts a live paid ingest and a standing watch (cf. Q375), a separate owner decision. Later: `agent-bus manage '#filescan/mdhouse@parf.us' --add-to-set-allow '#mdhouse@parf.us'`.

## Gaps (reported, never worked around)

| # | Gap |
|---|---|
| G1 | The user runner has no per-agent user: the agent runs as the person, so the read-only folder rests on the sandbox alone (security.md:46) |
| G2 | If step 6 shows a sandbox refusal, or `fallback: inotify is not available` / `watch limit reached` for a path under `/home`, report it; no sandbox-off switch is handed over |
| G3 | The docs said the agent starts as "the runner's per-agent user": fixed by T-49-G3 (landed 4ee1a5e3), which also states that the folder is read-only only with the sandbox on |
| G4 | The system runner (a per-person system account) cannot read a 0700 home, which is why this needs the user runner |
| G5 | filescan serves itself, and the only agent proven on this runner, `tmux-sessions`, is a per-message `args` agent: a self-serving agent's bus socket and listener on the user runner is unverified here (same as W1B's G6) |
