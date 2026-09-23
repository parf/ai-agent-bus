# MVP questions

📌 **TL;DR:** One 0.7 question is open, Q105 on renaming without a launcher.
The 2026-09-22 plan review raised Q94–Q104 and the owner settled them the same day. Settled choices live in the
decision index, withdrawn ones are recorded below with the reason they were
withdrawn, and every ID stays reserved.

## Open questions

| ID | Question | Blocks |
|---|---|---|
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
