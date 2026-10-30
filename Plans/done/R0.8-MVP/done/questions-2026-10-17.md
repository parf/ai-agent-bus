# MVP questions settled on 2026-10-17

📌 **TL;DR:** History: the threads of Q171–Q173, Q179 and Q180, kept as they
were settled. Their substance is in the [decision index](../../../../docs/decisions.md#settled)
and the docs it links; this file is not a current contract.

## Settled on 2026-10-17

- ✅ Q180 How the Slack agent learns whether a sender controls a subscription's destination (№S.8): no API answers "may this sender manage that record" today
  - Settled by: owner, 2026-10-17 ([decision](../../../../docs/decisions.md#settled), [route](../../../../docs/09-daemon-api.md#registry))
  - Home: [Slack authority](../../../R1.2/done/slack/README.md#subscriptions-authority-errors)
  - (x) The daemon answers it: `GET /manages?who=<name>&name=<record>`, yes or no, by the same rule `/manage` applies, to a caller who may see the record 🌟
  - ( ) The message carries it: the daemon adds the sender's roles toward a record the body names — the daemon would have to read bodies, which it never does
  - ( ) The agent works it out from `/lookup` and the groups — a second copy of the daemon's authority rules, wrong the day those change
  > 💡👾 Recommend the daemon answering 🌟: one authority, one small read endpoint, nothing in the body. Its cost is a new endpoint, and it tells a caller who manages a record it can already see; group membership comes along with it.
  >
  > 💬 👤parf (in the session) 1 - yes, do it

- ✅ Q179 How `--help` and `--mcp` reach an agent, so an agent that knows nothing of them is not confused by them (№K.47)
  - Settled by: owner, 2026-10-17 ([decision](../../../../docs/decisions.md#settled))
  - Home: [agent self-description](../TODO.md#agent-self-description)
  - ( ) The body is the flag, `--help` or `--mcp`, as the [constitution](../../../../docs/constitution.md#agent-self-description) says today: every agent sees it as input and must at least not act on it
  - (x) An envelope field marks the ask, with an empty body. The runner and `agent-bus run` give a script agent only an environment variable, `AGENT_BUS_ASK=help` or `mcp`, and no input; an agent serving itself reads the field. A later ask is invisible the same way 🌟
  - ( ) The body carries the flag on the wire, and the runner and `agent-bus run` turn it into the environment variable and an empty input; an agent serving itself still sees the body
  > 💡👾 Recommend the envelope field 🌟: the owner's aim, a script that ignores its environment answering an ask as it would answer nothing, holds for every agent, and no body text is reserved. Its cost is one optional envelope field in the daemon and every client library. The third option needs no daemon change, but an agent serving itself is still handed `--help` as input.
  >
  > 💬 👤parf (in the session) Envelope field — lets do this right now. Do not like an idea of "magic input params" at all

- ✅ Q172 Which dotenv conventions the [env-file syntax](../../../../docs/03-records-service.md#env-file-syntax) (№Q171) takes, beyond evaluation, which stays out: no `${VAR}`, no `$(…)`. The owner's references: [dotenvy's format](https://dotenvy.hexdocs.pm/dotenv-file-format.html) — double quotes decode `\n \r \t \f \b \" \' \\ \uFFFF`, triple quotes make multi-line values, `#` after a value is a comment — and [dotenv.org](https://www.dotenv.org/docs/security/env.html), "purposefully simple": `#` comments and `KEY=value`. The syntax as settled is stricter than both on two points: `\n` is not decoded, and an unquoted value may not hold a space.
  - Settled by: owner, 2026-10-17 ([decision](../../../../docs/decisions.md#settled), [contract](../../../../docs/03-records-service.md#env-file-syntax))
  - Home: [env-file syntax](../../../../docs/03-records-service.md#env-file-syntax)
  - (x) The official [dotenv](https://github.com/motdotla/dotenv#what-rules-does-the-parsing-engine-follow) format, as its own parser reads it, nothing evaluated — the owner's answer in the session; the [contract](../../../../docs/03-records-service.md#env-file-syntax) states it
  - ( ) dotenv as commonly written, nothing evaluated: an unquoted value runs to a ` #` or the line's end, trimmed, spaces inside allowed; `'…'` literal; `"…"` decodes `\n \r \t \" \\` (dotenvy's set without `\f \b \u`); a value may span lines inside `"""…"""` or `'''…'''`; `export ` allowed 🌟
  - ( ) the settled strict subset: unquoted values without spaces, `"…"` escapes only `\"` and `\\`, one line per value ⭐
  - ( ) dotenvy in full but evaluation: all its escapes, `\uFFFF` included, and triple-quoted multi-line
  - [ ] And beyond Go: a parser in another language — the Slack agent's `src/agents/slack/envfile.ts` reads its bus secret today — passes the same shared table of cases the Go parser does, rather than calling into Go (as №Q17's conformance suite does for the clients)
  > 💡👾 Recommend dotenv as commonly written, nothing evaluated 🌟: a `.env` copied from any project works unchanged, `\n` in a double-quoted private key becomes the newline it means, and a multi-line PEM fits in `"""…"""`. The strict subset ⭐ is simpler to validate but refuses ordinary files — `NAME=Jane Doe`, `KEY="line1\nline2"`. Full dotenvy adds `\f \b \u` that secrets do not use. Evaluation stays out in every option: what is written is what the agent gets. Tick the last box too: one table of cases, run against every parser, is what keeps "universal" true when an agent is not written in Go.
  >
  > 💬 👤parf Basic Syntax Rules
  >
  > • Key-Value Pairs: Written as KEY=VALUE separated by an equal sign (=) with no spaces around the =.
  > • Naming Keys: Use uppercase letters, numbers, and underscores (_). Keys must never start with a digit (e.g., DATABASE_URL, not 2DATABASE).
  > • Line-by-Line: Each environment variable must occupy its own separate line.
  > • Comments: Lines starting with a hash symbol (#) are treated as comments. You can also place comments at the end of a line after a value.
  > • Spacing: Leading and trailing spaces around unquoted values are typically ignored
  >
  >
  > Value Formatting & Quotes
  >
  > • Unquoted Values: Treated as literal strings (e.g., PORT=8080).
  > • Double-Quotes ("): Allow escape sequences (like \n for a newline) and support variable interpolation (referencing other variables like URL=https://${HOST}/api).
  > • Single-Quotes ('): Treat everything strictly as literal text; variable interpolation and escape sequences are ignored (e.g., LITERAL='no ${interpolation}').
  > • Multi-line Values: Triple-quotes or special escape sequences can define values spanning multiple lines.
  >
  > 💡👾 Reading your rules as the first option 🌟, made exact: `KEY=VALUE` with no spaces around `=`; a key of letters, digits and `_`, not starting with a digit (upper case is the convention, and lower case is still accepted, since `http_proxy` and its kind exist); one variable per line, `export ` allowed in front; `#` begins a comment on its own line or after a value; an unquoted value is literal and trimmed, spaces inside it kept; `'…'` literal, escapes and `$` ignored; `"…"` decodes `\n \r \t \" \\`; a value may span lines inside `"""…"""` or `'''…'''`. One of your rules disagrees with №Q171 — interpolation in double quotes, against "No $ expansion and no command substitution" — and it is asked as №Q173 below rather than chosen here. Tick the last box as well: the Slack agent's TypeScript parser passes the same table of cases.
  >
  > 💬 👤parf (in the session) yes, lets implement official dotenv spec — yes
  >
  > 💬👾 Closed: the official dotenv format, as its own parser reads it — [the rules](../../../../docs/03-records-service.md#env-file-syntax). Two of your notes above differ from the reference and the reference wins, as you asked for the official spec: it allows spaces around `=`, and it skips a line it cannot read, which the bus refuses instead, by line number. The Slack agent's parser passes the same cases ([K.39](../TODO.md#constitution-conformance)). It also differs from the first option as I wrote it, on four points, the first worth knowing before writing a password: an unquoted value ends at a `#` with or without a space before it, so `A=a#b` is `a` — quote a value that holds `#`; in double quotes only `\n` and `\r` are decoded, and `\t`, `\"` and `\\` stay as written; any quote kind may span lines, and there are no triple quotes; keys take `.` and `-`, and `KEY: value` is accepted.

- ✅ Q173 Does a double-quoted value interpolate `${VAR}`? №Q172's rules say so ("support variable interpolation, like `URL=https://${HOST}/api`"); №Q171, settled earlier the same day, says "No $ expansion and no command substitution". Both are the owner's, so the choice is too.
  - Settled by: owner, 2026-10-17 ([decision](../../../../docs/decisions.md#settled), [contract](../../../../docs/03-records-service.md#env-file-syntax))
  - Home: [env-file syntax](../../../../docs/03-records-service.md#env-file-syntax)
  - (x) No interpolation anywhere: `${HOST}` in any value arrives as written, as №Q171 says 🌟
  - ( ) `${VAR}` in double quotes only, from keys earlier in the same file and nothing else — never the process's environment, never `$(…)` ⭐
  - ( ) `${VAR}` in double quotes from earlier keys or the runner's own environment
  > 💡👾 Recommend no interpolation 🌟. A secret is a credential, and the commonest secret character set includes `$`: with interpolation, `PASS="pa${x}word"` silently becomes `pa` + nothing + `word`, the very loss that ruled out godotenv on 2026-10-16. If a value must be built from another, earlier keys only ⭐ keeps it inside the one file and away from the host's environment. Interpolating the runner's environment would let a host's variable leak into an agent's secret.
  >
  > 💬👾 Closed by №Q172's answer: the official dotenv format has no interpolation — expansion is a separate tool, dotenv-expand — so `${VAR}` arrives as written, as №Q171 says.

- ✅ Q171 What a record's secret may be written as. The daemon's check (`validEnv`) accepts lines the runner's parser refuses — `TOKEN="abc"def`, an unclosed quote — so a secret can be saved that stops its agent from starting. One grammar is to be shared by the save and the runner: (a) the runner's approved dotenv, `KEY=value` with `#` comments, single or double quotes and no `$` expansion (recommended: it is already the runner's [file format](../../../R1.0-Release/runner-config.md#file-formats), decided 2026-10-16); or (b) strict shell assignments, as `sh` would read them. Either way, a secret stored under the looser check stays readable, with a warning, so it can be corrected rather than lost.
  - Settled by: owner, 2026-10-17: the basic shell env syntax, universal, nothing evaluated, no legacy form ([decision](../../../../docs/decisions.md#settled), [contract](../../../../docs/03-records-service.md#env-file-syntax), [K.39](../TODO.md#constitution-conformance))
  - Home: [secrets](../../../../docs/03-records-service.md#secrets)
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
  > 💬👾 Closed. One syntax and one parser for every `KEY=value` list — record secrets and the runner's `env.dist` and `env` — which is how I read "universal"; say otherwise if you meant more or less. Nothing is evaluated, and there is no legacy form: the "stays readable with a warning" part of my proposal is withdrawn, and a stored secret or env file that does not fit is fixed by hand ([K.39](../TODO.md#constitution-conformance)).

Q122–Q129, Q141 and Q143 were settled on 2026-09-30 and moved to the
[decision index](../../../../docs/decisions.md#settled).

Q115, Q105, Q106 and Q108 were settled on 2026-09-23 and Q94–Q104 on
2026-09-22, all moved to the
[decision index](../../../../docs/decisions.md#settled).

Settled before 2026-09-22: Q79–Q82, Q85 and Q87–Q91 were settled on 2026-09-20 and moved to the
[decision index](../../../../docs/decisions.md#settled). Q83 was withdrawn after a
publish-versus-reload wording error. Q84 was withdrawn because the existing
no-compatibility rule already requires nonconforming state to be fixed before
activation. Q86, Q92 and Q93 were withdrawn when the owner limited the request
to logging entity edits rather than an audit subsystem. Their IDs remain
reserved, as do all earlier settled question IDs.

The [constitution](../../../../docs/constitution.md#-channels) owns the settled
forwarding contract. Q87 was settled last on 2026-09-21 by the hop ACL rule
there; no permission choice remains open and none blocks K.15.
