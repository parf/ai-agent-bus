# MVP questions

📌 **TL;DR:** Ten 0.7 questions are open, Q94–Q103, raised by the 2026-09-22
plan review; each names the rows it blocks. Settled choices live in the
decision index, withdrawn ones are recorded below with the reason they were
withdrawn, and every ID stays reserved.

## Open questions

| ID | Question | Blocks |
|---|---|---|
| Q94 | How does the daemon know that a message to a 👤 User is a reply to what that User sent, when it [keeps no exchange state](../../docs/04-messaging.md#reply-routing)? Is ending person-to-person sends and agent-to-person notices intended? | K.10, K.15 |
| Q95 | Does an Agent that creates a record gain any authority over it, given that the [owner is always its User](../../docs/constitution.md#-registry-record)? | K.4, K.7 |
| Q96 | Where do the entity-edit log and the error log live, how are they rotated and retained, and does anyone read them other than the daemon account? | K.14, K.21 |
| Q97 | Does the `*` grant, "every registered user", also admit Agents acting for those Users? | K.8 |
| Q98 | Do record-defined roles stay in MVP scope, given that the constitution does not name them? | K.7, K.18 |
| Q99 | What happens at startup to a User found without its 👤 record, and to a 👤 record whose `owner_id` names no User, beyond the [conceptual error](../../docs/constitution.md#errors-and-alerts)? | K.2, K.12 |
| Q100 | May a Group name carry a realm, as in `@support@srv1`? | K.9, K.19 |
| Q101 | Does the Personal cohort also restrict a 📣's `deliver_to` and a 📮's or 👾's forwarding destination? | K.15, K.20 |
| Q102 | On a record other than 👾, is `@agent` refused or accepted as resolving to nothing? | K.8 |
| Q103 | Does the version move to 0.7.0 at the first 0.7 change or at release, and when does `CHANGELOG.0.7.md` begin? | K.18 |

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
