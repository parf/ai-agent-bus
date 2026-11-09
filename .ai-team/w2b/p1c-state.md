# #ab-w2b checkpoint (2026-10-30 ~19:50)

## Open
- P.1c: ab-w2b/P1c 0c111de1 (tmp/ab-w2b-P1c), ready sent; waits the gate answer + readers #ab-w2c, #ab-w1b. Mac run is #ab-ceo's (Mac asleep). Keep worktree. Scratch to delete after the Mac run: tmp/p1c-mac, tmp/ab-w2b-P1b-dist, tmp/p1b-*, tmp/p1c-*.
- DW11b (from #ab-lead2 19:43, due 11-02 18:00, test+docs only, no bump, one reader, "ready <branch> <sha>"): start after P.1c's gate answer.
  (1) container check that extracts INSTALL.md's (src/INSTALL.md) checkout Install + Update blocks verbatim (git clone, bun install --cwd src/mcp, src/build.sh, sudo agent-bus-setup, sudo src/git-install.sh --no-build, update block), clean container, ends with `agent-bus version` + one call; a mutant breaking one doc command must fail on an assertion.
      Recon: reuse src/acceptance/fresh-install.Containerfile + fresh-install.sh style (podman --privileged --systemd=always); clone must be redirected to the local commit (git url.<local>.insteadOf https://github.com/parf/ai-agent-bus) so the doc line stays verbatim; bun install + go modules need network or a warm cache (check go vendoring); podman takes smoke-gate slow lock.
  (2) durable record for the final two-identity PHP serve/call example (src/clients/php/README.md:28-48): fixture with T-14 flags (log_errors=0, empty SASL_PATH), assert output.
- RT25a/RT25b/M3a landed and removed; M3a verified fully landed/superseded (sent).

## Rules
Lock: until agent-bus lock smoke-gate@parf.us slow --ttl=15m --wait=55s >/dev/null 2>&1; do sleep 1; done ; release: agent-bus release smoke-gate@parf.us slow. Never git fetch (hangs). Never php without empty SASL_PATH. TS client tests: env -u AGENT_BUS_*.
