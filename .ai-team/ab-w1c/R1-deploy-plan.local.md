# Concrete dev-node cutover proposal (not executed)

Candidate: 06fabb3c (0.9.303), tree82388f930a03213fd9f26d074a2ca722e7c8368e.
This cutover stages and validates only the daemon and Go CLI binary changes.
J1 changes the Codex pusher: existing MCP faces/pushers are NOT refreshed by
this cutover and keep their loaded code. Refreshing those sessions is a separate
owner-approved step after checkpoints; this plan does not add that restart.
No runner Go code changed; web changes are tests/docs. Keep unit configuration,
records, ACLs, credentials and agents untouched. Publish/tag/origin operations
are separate and not authorized by this proposal.

1. Stage stamped agent-busd + agent-bus with src/build.sh, PROGRAMS selecting
   those two packages, BUILD_COMMIT naming the exact candidate, in a detached
   worktree, using its src/build.sh with default output '.' (no non-dot output
   that would package TS/agents). Verify --version/build stamp and record checksums.
   Staging is complete at tmp/ab-w1c-R1-deploy-stage/src; no live changes.
2. Once owner authorizes the CEO-only restart substitution: confirm gate idle,
   no slow lock held, main/tree unchanged; W2C holds all further runs/landings.
3. Preserve the current two binaries and unit text with the stopped-daemon
   backup under /var/lib/agent-bus/daemon.backup/; never under tmp/.
4. One stop/start cycle: sudo systemctl stop agent-busd.service; archive the
   whole /var/lib/agent-bus/daemon directory (tar -C /var/lib/agent-bus -cpf ...
   daemon, with umask077 and the archive opened0600 from creation), preserving
   owners/modes and keeping credentials local. Never create a world-readable
   archive and tighten it afterwards.
5. Install each staged binary to a uniquely named temporary file IN its final
   checkout src directory, with the existing owner/group and0755 mode, then
   rename over that binary on the same filesystem. Preserve the existing
   /usr/local/bin symlinks. Start agent-busd.service.
6. Verify active unit, HTTP GET /identity protocol1, CLI --version/build stamp,
   src/check-deploy.sh6/6, gate/worker reconnection. No second restart or rollback
   without owner input if start fails; retained backup/binaries are the rollback
   material. Report exact candidate, backup path, result and outstanding limits.

Authority boundary: CLAUDE.local.md reserves daemon restarts to owner/CEO.
Owner made W1C release lead, but has not explicitly transferred CEO-only live
restart/push/tag authority. Ask only after staged candidate+slow proof are ready.

## Owner approvals now required

( ) 🌟 Authorize W1C's one prepared daemon/CLI cutover of06fabb3c, backed up as
above, then verify identity/check-deploy. Existing MCP/pushers are not refreshed.
( ) ⭐ Hold all live changes for the CEO.
💡👾 Authorize the narrow cutover: exact binaries and green slow proof are ready,
and W2C independently reviewed this plan CLEAN.

( ) 🌟 Accept the independent Codex reviews and corrected1305/0 slow run for the
final delta gate; authorize preparing a LOCAL STABLE candidate for review only.
( ) ⭐ Wait for the CEO/Fable final delta pass required by the existing runbook.
💡👾 Prepare the local candidate: existing fixes have independent reads and the
exact tree is slow-green; do not describe this as a new Fable pass. No push, tag,
publication or other owner-only step is included in either approval.
