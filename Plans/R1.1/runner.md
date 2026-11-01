# Runner extensions

📌 **TL;DR:** M.6b edge naming, M.7 named-host filtering and RT19 root-runner proposals moved from R1 to R1.1 on 2026-11-01 (Q299). These are targets, not shipped guarantees.

## Edge runners

An edge box hosts agents and holds no bus state; its daemon is elsewhere.

**An edge runner takes the bus's realm** (owner decision, [№Q15](../R1.0-Release/DECISIONS.md#recorded-decisions)). It has no
daemon to vouch for an `@<edge>` realm, so it registers as an instance under
the daemon it reports to, `#runner/<edge>@<realm>`, the way a
[pool member](../R1.0-Release/runner-implementation.md#the-name-a-member-registers) takes the pool's name. Where it
runs is the hostname it states, not its name. The instance is the one slot a
person's runner also uses ([who it runs as](../R1.0-Release/runner-implementation.md#who-it-runs-as)), so an edge's
name and a person's never mean two things at once ([№Q165](../R1.0-Release/DECISIONS.md#recorded-decisions)).

## Named hosts

M.7 carries the named-host filtering goal from [RD33 / Q158](../R1.0-Release/DECISIONS.md#recorded-decisions). The proposed filtering proxy is the runner’s, reached through a socket mounted into the sandbox and named in `HTTPS_PROXY`, passing only the hosts in `needs`; it covers what goes through a proxy, so an agent needing raw UDP needs the full network. The mechanism and format still need owner approval ([open choice](QUESTIONS.md#runner-extensions)); today’s behavior is [current isolation](../../docs/runner/security.md#isolation-today).

## Root runner

Owner, 2026-10-19: "root runner; allow to run services under specific user; or under root (optional drop permissions)"

Proposed, not designed: today each person runs their own runner as `<person>-ab-runner`, and agents that need host access (systemd, logwatch, git) are standalone units on the shared starter. A root runner could keep all of them in one place, per agent `user: <account>` or `user: root` with `drop` of capabilities (e.g. keep only CAP_DAC_READ_SEARCH for logwatch)

Settled, owner 2026-10-19: an agent's account or root is set by whoever has access to the host (the owner’s words, translated: "whoever has access to the host"): it lives in a root-owned host file the runner reads, never in a bus record or call, so no bus caller, Owner included, can raise an agent's privilege ([process boundary](../../docs/08-processes.md#the-rule))

The [open choices](QUESTIONS.md#runner-extensions) precede design. Execution belongs in [RT19](TODO.md#runner-extensions).
