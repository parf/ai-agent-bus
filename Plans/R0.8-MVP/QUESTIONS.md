# MVP questions

📌 **TL;DR:** Q124 and Q125 (its inactive-record half) are open, raised by the [constitution review](constitution-review.md#code-violations) of 2026-09-24. Q130 was settled on 2026-09-25 and moved to the [decision index](../../docs/decisions.md#settled). Q115, raised by H.9.6, and Q105, Q106 and Q108 were settled on 2026-09-23.
The 2026-09-22 plan review raised Q94–Q104 and the owner settled them the same day. Settled choices live in the
decision index, withdrawn ones are recorded below with the reason they were
withdrawn, and every ID stays reserved.

## Open questions

Recommendation first in each.

| ID | Question | Recommendation | Blocks |
|---|---|---|---|
| Q124 | `agent-bus start` records its script path in an Agent's `addr`, a 📡-only field | document it as the runner's own note on a 👾, read-only to callers; or move it into the Agent's configuration | K.28 |
| Q125 | Owner and Maintainers read inactive records, and an Agent reaches its Owner and Maintainers; the constitution names only `allow`. Which is intended? Its private-values half was settled on 2026-09-30: the Owner and Maintainers read and write them ([decision index](../../docs/decisions.md#settled)) | the code: whoever manages a record already rewrites its values, so hiding them protects nothing; the constitution names them | K.35 |

Q122, Q123, Q126–Q129 and Q141 were settled on 2026-09-30 and moved to the
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
