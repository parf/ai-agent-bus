# MVP questions

📌 **TL;DR:** Q143 is open: no client on this bus can yet use a face that speaks only MCP 2026-07-28. Q125, the last [constitution review](constitution-review.md#code-violations) question, was settled on 2026-09-30. Q130 was settled on 2026-09-25 and moved to the [decision index](../../docs/decisions.md#settled). Q115, raised by H.9.6, and Q105, Q106 and Q108 were settled on 2026-09-23.
The 2026-09-22 plan review raised Q94–Q104 and the owner settled them the same day. Settled choices live in the
decision index, withdrawn ones are recorded below with the reason they were
withdrawn, and every ID stays reserved.

## Open questions

Recommendation first in each.

| ID | Question | Recommendation | Blocks |
|---|---|---|---|
| Q143 | The [constitution](../../docs/constitution.md#external-protocols) allows only MCP 2026-07-28, but no client here can use such a face yet. Measured 2026-09-30: Claude Code 2.1.286 speaks it, then reports "Channel messages … are unavailable: this connection's protocol version has no channel delivery path", so push stops; Codex 0.159.2 and OpenCode 1.18.30 send only a 2025 `initialize`, which a latest-only face refuses. Serving both revisions does not help, since Claude Code then picks 2026-07-28 and loses push. Keep the face on 2025-11-25 until the clients catch up, or follow the rule and lose push and two clients? | keep 2025-11-25 as a stated, dated exception, and move to the v2 SDK with legacy refused once Claude Code delivers channels on 2026-07-28 and Codex and OpenCode speak it; recheck on each client release | K.37 |

Q122–Q129 and Q141 were settled on 2026-09-30 and moved to the
[decision index](../../docs/decisions.md#settled).

Q115, Q105, Q106 and Q108 were settled on 2026-09-23 and Q94–Q104 on
2026-09-22, all moved to the
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
