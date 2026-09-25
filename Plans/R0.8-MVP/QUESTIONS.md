# MVP questions

📌 **TL;DR:** Q122–Q129 are open, raised by the [constitution review](constitution-review.md#code-violations) of 2026-09-24. Q130 was settled on 2026-09-25 and moved to the [decision index](../../docs/decisions.md#settled). Q115, raised by H.9.6, and Q105, Q106 and Q108 were settled on 2026-09-23.
The 2026-09-22 plan review raised Q94–Q104 and the owner settled them the same day. Settled choices live in the
decision index, withdrawn ones are recorded below with the reason they were
withdrawn, and every ID stays reserved.

## Open questions

Recommendation first in each.

| ID | Question | Recommendation | Blocks |
|---|---|---|---|
| Q122 | Which incorrect stored rows refuse the start instead of being ignored? Today a malformed record body, message rows without their queue, out-of-range IDs, a damaged `@administrators` and an account mapping to a missing OS account all refuse it; the constitution says every incorrect record is ignored | keep those as refusals, since each breaks the store's shape or the node's authority, and list them in the constitution; ignore and report everything else | K.35 |
| Q123 | A 📣 already stored with `ttl`, `bound` or `overflow`: what does load do? | drop the fields at load with a warning, since they never had a meaning of their own; ignoring the whole record would take a working topic away. Built this way in 0.8.52 on the owner's "fix pubsub now"; awaiting the owner's confirmation | — |
| Q124 | `agent-bus start` records its script path in an Agent's `addr`, a 📡-only field | document it as the runner's own note on a 👾, read-only to callers; or move it into the Agent's configuration | K.28 |
| Q125 | Owner and Maintainers read private values and inactive records, and an Agent reaches its Owner and Maintainers; the constitution names only `allow`. Which is intended? | the code: whoever manages a record already rewrites its values, so hiding them protects nothing; the constitution names them | K.35 |
| Q126 | A User inside a Group on a 📣's `deliver_to` | skip it at publication with an error-log warning, not counted as a drop, and say so; refusing the Group would break a working topic when membership changes | K.30 |
| Q127 | A 📣 copy that fails through a forwarding 👾 or 📮: whose `dropped` counts it? | the listed recipient's, since that branch is what failed; the forwarding rule "counts neither overflow case" is for a direct send | K.35 |
| Q128 | Edits made in the web face reach the daemon over the socket, so the audit log has no client IP although the visitor has one | document that web-face edits carry no IP in MVP; forwarding the visitor's IP from the face is an R1 choice | — |
| Q129 | An Agent's old Owner keeps a working token after a transfer | rotate the token on transfer, since the old Owner holds live bytes it may no longer fetch | — |

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
