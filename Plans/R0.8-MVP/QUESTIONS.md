# MVP questions

📌 **TL;DR:** One MVP question is open, Q172, which dotenv conventions the env-file syntax takes; Q171, what a secret may be written as, was settled on 2026-10-17. Q143, the MCP 2025-11-25 exception, and Q125, the last [constitution review](constitution-review.md#code-violations) question, were settled on 2026-09-30. Q130 was settled on 2026-09-25 and moved to the [decision index](../../docs/decisions.md#settled). Q115, raised by H.9.6, and Q105, Q106 and Q108 were settled on 2026-09-23.
The 2026-09-22 plan review raised Q94–Q104 and the owner settled them the same day. Settled choices live in the
decision index, withdrawn ones are recorded below with the reason they were
withdrawn, and every ID stays reserved.

## Open questions

- ❓ Q172 Which dotenv conventions the [env-file syntax](../../docs/03-records-service.md#env-file-syntax) (№Q171) takes, beyond evaluation, which stays out: no `${VAR}`, no `$(…)`. The owner's references: [dotenvy's format](https://dotenvy.hexdocs.pm/dotenv-file-format.html) — double quotes decode `\n \r \t \f \b \" \' \\ \uFFFF`, triple quotes make multi-line values, `#` after a value is a comment — and [dotenv.org](https://www.dotenv.org/docs/security/env.html), "purposefully simple": `#` comments and `KEY=value`. The syntax as settled is stricter than both on two points: `\n` is not decoded, and an unquoted value may not hold a space.
  - Settled by: owner, before [K.39](TODO.md#constitution-conformance) is built
  - Home: [env-file syntax](../../docs/03-records-service.md#env-file-syntax)
  - ( ) dotenv as commonly written, nothing evaluated: an unquoted value runs to a ` #` or the line's end, trimmed, spaces inside allowed; `'…'` literal; `"…"` decodes `\n \r \t \" \\` (dotenvy's set without `\f \b \u`); a value may span lines inside `"""…"""` or `'''…'''`; `export ` allowed 🌟
  - ( ) the settled strict subset: unquoted values without spaces, `"…"` escapes only `\"` and `\\`, one line per value ⭐
  - ( ) dotenvy in full but evaluation: all its escapes, `\uFFFF` included, and triple-quoted multi-line
  - [ ] And beyond Go: a parser in another language — the Slack agent's `src/agents/slack/envfile.ts` reads its bus secret today — passes the same shared table of cases the Go parser does, rather than calling into Go (as №Q17's conformance suite does for the clients)
  > 💡👾 Recommend dotenv as commonly written, nothing evaluated 🌟: a `.env` copied from any project works unchanged, `\n` in a double-quoted private key becomes the newline it means, and a multi-line PEM fits in `"""…"""`. The strict subset ⭐ is simpler to validate but refuses ordinary files — `NAME=Jane Doe`, `KEY="line1\nline2"`. Full dotenvy adds `\f \b \u` that secrets do not use. Evaluation stays out in every option: what is written is what the agent gets. Tick the last box too: one table of cases, run against every parser, is what keeps "universal" true when an agent is not written in Go.

## Settled on 2026-10-17

- ✅ Q171 What a record's secret may be written as. The daemon's check (`validEnv`) accepts lines the runner's parser refuses — `TOKEN="abc"def`, an unclosed quote — so a secret can be saved that stops its agent from starting. One grammar is to be shared by the save and the runner: (a) the runner's approved dotenv, `KEY=value` with `#` comments, single or double quotes and no `$` expansion (recommended: it is already the runner's [file format](../R1.0-Release/runner-config.md#file-formats), decided 2026-10-16); or (b) strict shell assignments, as `sh` would read them. Either way, a secret stored under the looser check stays readable, with a warning, so it can be corrected rather than lost.
  - Settled by: owner, 2026-10-17: the basic shell env syntax, universal, nothing evaluated, no legacy form ([decision](../../docs/decisions.md#settled), [contract](../../docs/03-records-service.md#env-file-syntax), [K.39](TODO.md#constitution-conformance))
  - Home: [secrets](../../docs/03-records-service.md#secrets)
  - ( ) Share the runner’s approved dotenv grammar: `KEY=value`, comments, quotes and no `$` expansion 🌟
  - ( ) Require strict shell assignments, as `sh` would read them ⭐
  > ✅ 💡👾 Recommend the runner’s existing dotenv grammar 🌟 so a newly saved secret is accepted by the same parser that delivers it. Strict shell syntax is the alternative ⭐ only if shell compatibility is the intended contract; that revises the approved runner format. Either choice must preserve old stored secrets for read-back and correction, rather than ignoring their records on restore.
  >
  > 💬 👤parf we need to support basic shell syntax
  > as all env files do
  > validator should be pretty simple
  >
  > ✅ 💡👾 Recommend the basic shell env syntax that env files share 🌟, one small parser for the save and the runner: `KEY=value` lines, an optional `export ` before the name, `#` comments on their own line or after an unquoted value, a value unquoted (no spaces or quotes), in single quotes (literal) or in double quotes (only `\"` and `\\` escaped). No `$` expansion and no command substitution: what is stored is what the agent gets, so the validator stays simple. A line that is none of these is refused, named by its line number and never echoed. A secret saved under today's looser check stays readable, with a warning, until it is corrected.
  >
  > 💬 👤parf agree
  >
  > 💬 👤parf yes, and make sure it is universal
  > No $ expansion and no command substitution
  >
  > 💬 👤parf agree
  >
  > 💬 👤parf (in the session, 2026-10-17) DO NOT support legacy format / everything until release; if we have old secret in wrong format (most likely we do not) - just fix it
  >
  > 💬👾 Closed. One syntax and one parser for every `KEY=value` list — record secrets and the runner's `env.dist` and `env` — which is how I read "universal"; say otherwise if you meant more or less. Nothing is evaluated, and there is no legacy form: the "stays readable with a warning" part of my proposal is withdrawn, and a stored secret or env file that does not fit is fixed by hand ([K.39](TODO.md#constitution-conformance)).

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
