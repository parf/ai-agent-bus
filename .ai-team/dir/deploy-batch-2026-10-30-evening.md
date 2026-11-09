# Deploy batch, 2026-10-30 evening (target 19:30-20:00)

Landed and waiting: J56 156e3744 (runner), J57 6db2f0a5 (runner + slack/telegram packages), M.6a c38b05d0 (daemon + runner), B.29 823e17c0 (cron + #at packages), RT24 bfeb5f58 (daemon), J60 2fce2f67 (MCP face + ab-codex launcher; then W1C/W2C restart between tasks; then #ab-ceo live /compact check), M.8 e952c374 + M.8b 20769d8f (runner), J59 368635bd + J59b (setup), P3d 27f15a52 (release-verify), P.1a 66fafca0 / P.1b 94e7974a (release/npm build only, nothing live), J46 623de7e1 (eventlog package), RT22 dcf5e419 (agent-bus-web restart). RT25a 7b03df8b + RT25b 0430a095 (0.9.237, daemon + runner; Slack package for RT25(b)). Pending: RT25(a)+(b) after #ab-qa1's read; J49 dd34f63a landed (0.9.230), standalone packagers; nothing to deploy now.

Agent packages always replace, never overlay.

## Eventlog redeploy (J46), from #ab-w1c 18:51, run by #ab-ceo only

From /usr/local/src/ai-agent-bus on main containing 623de7e1:

1. `sudo src/agents/eventlog/package.sh /var/lib/agent-bus/agent.d/eventlog`
2. Both obsolete root files absent (no dangling symlinks either); keep licenses/NOTICE.md:
   - `sudo test ! -e /var/lib/agent-bus/agent.d/eventlog/NOTICE.md && sudo test ! -L /var/lib/agent-bus/agent.d/eventlog/NOTICE.md`
   - `sudo test ! -e /var/lib/agent-bus/agent.d/eventlog/THIRD-PARTY-NOTICES.md && sudo test ! -L /var/lib/agent-bus/agent.d/eventlog/THIRD-PARTY-NOTICES.md`
3. As parf, reload only: `env -u AGENT_BUS_TOKEN -u AGENT_BUS_ADDR -u AGENT_BUS_NAME agent-bus call '#runner/parf@parf.us' --wait 20s reload`
4. As parf: `env -u AGENT_BUS_TOKEN -u AGENT_BUS_ADDR -u AGENT_BUS_NAME src/check-deploy.sh '#debug@parf.us'`

Stop on any failure; preserve inventory, env, work, tokens, bus Configuration/Secret/ACLs.

5. Only after the whole batch is green: `sudo rm -rf -- /usr/local/src/ai-agent-bus/tmp/j46-imap-old`, then `sudo test ! -e … && sudo test ! -L …` on the same path.

## Steps for #ab-ceo (per docs/07-setup.md § deploy only the changed component)

Main at the hold point (RT22b + P.1c landed). Stop on any failure.

1. Daemon (M.6a, RT24, RT25a): back up the stopped daemon home to /var/lib/agent-bus/daemon.backup/, stamped `src/build.sh` (via `sudo src/git-install.sh`), restart agent-busd. Verify reconnection; `agent-bus ls --table <a shared name>` shows the Hosts line; `agent-bus call '#bus' status --json` answers (RT24); `consume?asks=only` accepted (RT25a).
2. Runner (J56, J57, M.6a share+host, M.8/M.8b deploy calls, RT25b reader-less status): stamped build, restart `agent-bus-runner@parf` (binary changed, so a restart, not a reload). Verify: #git/#logwatch/#server-info/#systemd no longer show refused (J56).
3. Agent packages, each with its own `src/agents/<name>/package.sh` into its installed agent.d dir (replace, never overlay): slack + telegram (J57 shared modules, RT25b Slack starting/ok), cron + at (B.29; legacy-id migration on first start, watch for its warning), eventlog (J46 steps above). Then ONE runner reload.
4. Web (RT22): restart agent-bus-web; the create form shows the configured realm.
5. J60 + RT22b (0.9.238, launcher automatic names take the #bus realm): confirm `ab-claude`/`ab-codex` and the MCP face resolve to the new code (git-install links); rebuild the MCP bundle if it is compiled. Existing bound names must not change.
6. `src/check-deploy.sh` as parf (env -u AGENT_BUS_TOKEN -u AGENT_BUS_ADDR -u AGENT_BUS_NAME …).
7. Wait for #ab-gate readers 1 (or 90 s).
8. After green: `sudo rm -rf -- /usr/local/src/ai-agent-bus/tmp/j46-imap-old` (+ absence check); then tell #ab-dir "deployed <sha>". #ab-dir releases the leads.
9. Afterwards, between tasks: restart W1C and W2C (Codex) to load J60; then the J60 live check on a throwaway session.
