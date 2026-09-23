# MVP questions

📌 **TL;DR:** Three 0.7 questions are open: Q105 on renaming without a launcher, Q106 on how a committed write is published, Q108 on which log a refused flow writes.
The 2026-09-22 plan review raised Q94–Q104 and the owner settled them the same day. Settled choices live in the
decision index, withdrawn ones are recorded below with the reason they were
withdrawn, and every ID stays reserved.

## Open questions

| ID | Question | Blocks |
|---|---|---|
| Q108 | Which log takes the "daemon log entry" the [constitution](../../docs/constitution.md#common-record-fields) and K.11 require when a flow is refused? The term predates the three logs, and K.21 forbids an error-log line for an ordinary refusal such as an unknown name. Options: (a) error.log (with syslog) only for a flow that had recipients and lost them — a publication whose recipients all failed, a copy discarded beside working ones, a forwarding route whose destination is gone — while a direct send to an absent or inactive name is an ordinary refusal seen only in the debug log; (b) error.log for every refused flow, a direct send to an unknown name included. 0.7.9 does (a) | K.11, K.15 log wording |
| Q106 | Does a write staged in the live maps under the registry lock, with an undo log and every reader behind the same lock, satisfy the constitution's "one complete new view, never a mutation of the live maps in place"? Built through 0.7.6: no reader can observe a staged or rolled-back write, a failed commit restores every entity, and the next write rewrites no stale state. Options: (a) accept it and reword the publication rule to "no reader observes a write before its commit"; (b) build copy-on-write views published by pointer, so reads need no lock. (a) keeps K.3's failure checks; (b) is a core-wide refactor | K.3 wording |
| Q105 | May an Agent obtain the credential of another Agent owned by the same User? 0.7 says every record is its User's and a credential is issued to the Owner, so a bare MCP face holding only an agent token cannot take the new address `ab_rename` registers. Options: (a) keep it refused — renaming needs an `ab-*` launcher, which acts for the User; (b) let an Agent be issued a credential for any Agent of its own Owner; (c) a daemon rename that moves the caller's own credential. 0.7.5 does (a) as the conservative default | bare-MCP `ab_rename` |

Q94–Q104 were settled on 2026-09-22 and moved to the
[decision index](../../docs/decisions.md#settled).

Settled before 2026-09-22: Q79–Q82, Q85 and Q87–Q91 were settled on 2026-09-20 and moved to the
[decision index](../../docs/decisions.md#settled). Q83 was withdrawn after a
publish-versus-reload wording error. Q84 was withdrawn because the existing
no-compatibility rule already requires nonconforming state to be fixed before
activation. Q86, Q92 and Q93 were withdrawn when the owner limited the request
to logging entity edits rather than an audit subsystem. Their IDs remain
reserved, as do all earlier settled question IDs.

The [constitution](../../docs/constitution.md#-channels) owns the settled
forwarding contract. Q87 was settled last on 2026-09-21 by the hop ACL rule
there; no permission choice remains open and none blocks K.15.
