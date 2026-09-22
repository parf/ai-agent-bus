# MVP questions

📌 **TL;DR:** Six 0.7 questions are open, raised by the 2026-09-22 plan
review; each names the rows it blocks. Settled choices live in the
decision index, withdrawn ones are recorded below with the reason they were
withdrawn, and every ID stays reserved.

## Open questions

| ID | Question | Blocks |
|---|---|---|
| Q94 | How does the daemon know that a message to a 👤 User is a reply to what that User sent, when it [keeps no exchange state](../../docs/04-messaging.md#reply-routing)? | K.10, K.15 |
| Q96 | Where do the entity-edit log and the error log live, how are they rotated and retained, and does anyone read them other than the daemon account? | K.14, K.21 |
| Q97 | Does the `*` grant, "every registered user", also admit Agents acting for those Users? | K.8 |
| Q98 | Do record-defined roles stay in MVP scope, given that the constitution does not name them? | K.7, K.18 |
| Q101 | Does the Personal cohort also restrict a 📣's `deliver_to` and a 📮's or 👾's forwarding destination? | K.15, K.20 |
| Q102 | On a record other than 👾, is `@agent` refused or accepted as resolving to nothing? | K.8 |

Q95, Q99, Q100 and Q103 were settled on 2026-09-22 and moved to the
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
