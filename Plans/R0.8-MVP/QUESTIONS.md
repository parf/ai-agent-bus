# MVP questions

📌 **TL;DR:** One MVP question is open, Q171, what a secret may be written as. Q143, the MCP 2025-11-25 exception, and Q125, the last [constitution review](constitution-review.md#code-violations) question, were settled on 2026-09-30. Q130 was settled on 2026-09-25 and moved to the [decision index](../../docs/decisions.md#settled). Q115, raised by H.9.6, and Q105, Q106 and Q108 were settled on 2026-09-23.
The 2026-09-22 plan review raised Q94–Q104 and the owner settled them the same day. Settled choices live in the
decision index, withdrawn ones are recorded below with the reason they were
withdrawn, and every ID stays reserved.

## Open questions

- ❓ Q171 What a record's secret may be written as. The daemon's check (`validEnv`) accepts lines the runner's parser refuses — `TOKEN="abc"def`, an unclosed quote — so a secret can be saved that stops its agent from starting. One grammar is to be shared by the save and the runner: (a) the runner's approved dotenv, `KEY=value` with `#` comments, single or double quotes and no `$` expansion (recommended: it is already the runner's [file format](../R1.0-Release/runner-config.md#file-formats), decided 2026-10-16); or (b) strict shell assignments, as `sh` would read them. Either way, a secret stored under the looser check stays readable, with a warning, so it can be corrected rather than lost.
  - Settled by: owner
  - Home: [secrets](../../docs/03-records-service.md#secrets)
  - ( ) Share the runner’s approved dotenv grammar: `KEY=value`, comments, quotes and no `$` expansion 🌟
  - ( ) Require strict shell assignments, as `sh` would read them ⭐
  > 💡👾 Recommend the runner’s existing dotenv grammar 🌟 so a newly saved secret is accepted by the same parser that delivers it. Strict shell syntax is the alternative ⭐ only if shell compatibility is the intended contract; that revises the approved runner format. Either choice must preserve old stored secrets for read-back and correction, rather than ignoring their records on restore.

Q122–Q129, Q141 and Q143 were settled on 2026-09-30 and moved to the
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
