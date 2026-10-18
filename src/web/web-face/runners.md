# Web face runner pages

📌 **TL;DR:** `/runners` and `/runner` show the runners a visitor may see and
what each answers to `status`, its agents linked to their own pages, and a
`reload` whose result is the runner's own answer. The face is a client of the
runner: it calls it as the visitor, stores nothing, and edits no record here.
Every page follows the [shell](shell.md#web-face-shell); the plan is
[runners in the web face](../../../Plans/R1.0-Release/runner/web-face.md#goal).

## What a runner is here

A record of kind agent named `#runner/<instance>@<host>`
([№RD84](../../../Plans/R1.0-Release/DECISIONS.md#recorded-decisions)), among
the records the visitor may see. A `runner` tag, or a name such as
`#runner-x@host`, does not make one.

## How the face asks a runner

| | |
|---|---|
| The call | `POST /send` to the runner as the visitor, body `status` or `reload`, topic `web`, a fresh tag, the wait as the message's deadline; then `GET /consume` with that topic and tag, so only the answer is taken from the visitor's inbox ([one reader per inbox](../../../docs/04-messaging.md#one-reader-per-inbox)) |
| Waits | `status` 10 s on a runner's page, 3 s per row of the chooser, `reload` 30 s |
| No answer | named as no answer, never as a runner with no agents; a `done` receipt with no answer is named too |
| Sent, then lost | a wait that fails after the send was accepted, or a send the daemon never answered, is an unknown outcome: the call may have gone, so it is never shown as refused or as not carried out |
| Known limits | the answer is matched only once the wait is outstanding: a client already reading the visitor's whole inbox may take a very fast answer first, and the face then reports no answer. An answer that comes after the wait stays in the visitor's inbox under its own tag; no later call takes it |
| A refusal | the bus's sentence, as the runner's ACL or visibility decided it; the face adds no access rule of its own |
| A text answer | a runner refusing a verb answers text, shown as it said it |

## `/runners`

| Accessible runners | Answer |
|---|---|
| none | the empty state, linking [a person's runner](../../../docs/07-setup.md#a-persons-runner) |
| exactly one | `301` to its `/runner?name=`, decided from this visitor's set on every request; `Cache-Control: no-store` as on every page |
| several | a table: runner and description, Owner, its answer (`answering · v<version>`, `no answer`, `refused`; `inactive` is not asked), its agent count |

## `/runner?name=`

`404` for a name the visitor may not see, and for a visible record that is not
a runner.

| Card | Content |
|---|---|
| Head | the runner's name, Owner and description, and **Ask again**, which reloads the page and asks `status` once more |
| Runner | answering, host, version — with a warning when it differs from this node's, older meaning the runner was not restarted after an update — the count of agents per state, and, from a runner that says them, when it started, its agent code directory and its host settings directory; when asked, a whole-load error; or why there is no answer. A link to the runner's own record |
| Agents | one row per agent the runner answered: the name, linked to `/agent?name=…&return=/runner?name=…` when the visitor may see its record, with its description and tags, otherwise "its record is not visible to you"; state, with "not enabled" when it is not, and whether it serves itself or runs a script per message; processes running of max (min and max on hover), and since when; protection: sandbox with its memory, process and disk limits, a warning when the sandbox is off, nothing when the runner does not say; its inbox, queued and reading, and last used, both from the visitor's own record listing, so a hidden record shows none; the runner's reason and the last exit, its code (or killed by a signal), when, and its last stderr line. The runner's own fields are [№RD99](../../../Plans/R1.0-Release/DECISIONS.md#recorded-decisions); a runner before 0.8.203 sends none of them, and their cells stay empty. Not answered: "not known", never an empty list |
| Actions | Reload, explained as runner-wide; starting and stopping one agent marked as not built |

## `POST /runner-reload`

Form field `name`. Sends `reload` and renders the runner's page:

| Outcome | Shown |
|---|---|
| an answer | the toast "Reloaded: the runner answered with the status below", with that status |
| no answer, `done`, or a wait that failed after the send | a warning that the outcome is unknown and may still take effect, with a fresh `status`; nothing is sent again. If the page cannot be drawn around it, the warning is shown alone (502), never the generic "nothing was changed" |
| a refusal | the warning "The reload was not carried out", with the reason |

## Returning

An agent page opened from a runner keeps `return=/runner?name=…`: the record
detail, settings and save paths admit `/runner` beside the record's own list,
under the same local-path check ([shell § return](shell.md#web-face-shell)).
