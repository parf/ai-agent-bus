# Web face runner pages

📌 **TL;DR:** `/runners` and `/runner` show the runners a visitor may see,
what each answers to `status` and what needs a look; they change nothing.
Each agent's **Manage**, last in its row, opens `/runner-settings` for it:
its controls, stop, disable and delete in one closed danger-zone expand, its files and
instances, every result the runner's own answer. With no agent the same page is
the host's settings: `reload` and the inventory. There is no page managing
every agent at once. The face is a client of the runner: it
calls it as the visitor, stores nothing, and edits no record here.
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
| The call | `POST /send` to the runner as the visitor, body one of the runner's [calls](../../../docs/runner/operations.md#controlling-one-agent), topic `web`, a fresh tag, the wait as the message's deadline; then `GET /consume` with that topic and tag, so only the answer is taken from the visitor's inbox ([one reader per inbox](../../../docs/04-messaging.md#one-reader-per-inbox)) |
| Waits | `status` 10 s on a runner's page, 3 s per row of the chooser, `reload`, a control, a save, a create or a delete 30 s, each `read` 10 s |
| No answer | named as no answer, never as a runner with no agents; a `done` receipt with no answer is named too |
| Sent, then lost | a wait that fails after the send was accepted, or a send the daemon never answered, is an unknown outcome: the call may have gone, so it is never shown as refused or as not carried out |
| Known limits | the answer is matched only once the wait is outstanding: a client already reading the visitor's whole inbox may take a very fast answer first, and the face then reports no answer. An answer that comes after the wait stays in the visitor's inbox under its own tag; no later call takes it. A late `read` answer holds the file, an env file's secrets included, and stays in the visitor's inbox, stored in the daemon's queue, until consumed or expired |
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
| Head | the runner's name, Owner and description, **Ask again**, which reloads the page and asks `status` once more, and **Host settings**, to [`/runner-settings`](#runner-settingsnamekey) with no agent |
| Alerts | only when something needs a look: a whole-load error, each refused or unconfigured agent with its reason, each stopped one, each with its sandbox off, each whose last exit was not 0, a runner older than this node, and one too old for per-agent control |
| Runner | answering, version — with a warning when it differs from this node's, older meaning the runner was not restarted after an update — host, the count of agents per state and, from a runner that says it, when it started; below them, a line each, its agent code and host settings directories; a whole-load error; or why there is no answer. A link to the runner's own record |
| Agents | one row per agent the runner answered: drawn as the agents list draws a record, its description over its name, linked to `/agent?name=…&return=/runner?name=…` when the visitor may see it, then its tags; otherwise "its record is not visible to you"; state, with "not enabled" when it is not, and whether it serves itself or runs a script per message; processes running of max (min and max on hover), and since when; protection: sandbox, its memory, process and disk limits a bulleted list on hover, a warning when the sandbox is off, nothing when the runner does not say; its inbox, queued only when some are, then reading, and last used, both from the visitor's own record listing, so a hidden record shows none; last, **Manage**, for an agent with a `key` (a runner from 0.8.210). The runner's own fields are [№RD99](../../../Plans/R1.0-Release/DECISIONS.md#recorded-decisions); a runner before 0.8.203 sends none of them, and their cells stay empty. Not answered: "not known", never an empty list |

The page offers no operation: it is information and alerts.

## `POST /runner-control`

Fields `name`, `verb` (start, stop, restart, enable, disable) and `agent`, a
key; anything else is `400` and nothing is sent. Sends `<verb> <agent>` and
renders that agent's page with the outcomes of [reload](#post-runner-reload),
the toast naming the verb and agent.

## `/runner-settings?name=&key=`

One agent or instance managed, or with no `key` the host's settings. Each file
is a `read` of the runner
([the host's files](../../../docs/runner/operations.md#the-hosts-files-over-the-bus)).
Its Owner and Maintainers only: the runner refuses anyone else, and its
refusal is what such a visitor sees.

| Card | Content |
|---|---|
| Runner, no `key` | Reload, explained as runner-wide, and that it serves again what a stop took down; then the host's inventory `runner/agents.json` below |
| Head | as the agent's Edit page: back to the runner, the agent kind, its name and, when its record is visible, its Owner; the host's settings name the runner |
| Agent | from a fresh `status`, the agent the runner names with this `key`: one line, its state, then Start when stopped, Restart when enabled, Enable when not, and Reload, which returns here; below it the runner's reason and its last exit — code or killed by a signal, when, and its last stderr line — when it says them; a key the runner does not name, or a runner before 0.8.210, said so, with no controls |
| Danger zone | one closed `<details>`, its summary naming what it holds; inside, a row each with what it does and how long it lasts: Stop, unless stopped or disabled; Disable, when enabled; for an instance, Delete, a link to its confirmation |
| The author's files | `agent.d/<agent>/agent.json` and `env.dist`, read only, as shown; absent said so |
| The host's files | "Restart the agent to apply changes" (with no `key`, reload the runner); `runner/<agent>/env` and `agent.json`, and for an instance its own two, each its own form with a Save; absent said so; an `agent.json` or `agents.json` links its sample of every option on GitHub ([configuration](../../../docs/runner/configuration.md)); a file the runner would not read shows why, with no form |
| Instances | a link to its record, which holds the bus secret and configuration; a Create of `<agent>/<instance>` |

The runner answers the controls and files only to its Owner and Maintainers;
anyone else sees the runner's refusal where the file or the result would be.

## `POST /runner-file`, `POST /runner-instance`

`/runner-file`: `name`, `key`, `file` — only `runner/agents.json` or a
`runner/<agent>[/<instance>]/env` or `agent.json`, else `400` — and `content`,
CRLF made LF. Sends `write <file>` and the content. An answer is the toast
"Saved …"; a refusal is a warning with the runner's reason, and the form keeps
what was typed; an unknown outcome is said as for reload.

`/runner-instance`: `name`, `verb` create or delete, `key` `<agent>/<instance>`.
A created instance opens its page; a delete, and any warning, its agent's
page. A delete needs `confirm` equal to the key: otherwise the confirmation
page again, with a warning, and nothing sent.

`GET /runner-instance-delete?name=&key=`: what deleting the instance takes —
its `agents.json` entry, its env with its secrets, its override — and what
stays — its work, kept token and record — and the key to type again.

## `POST /runner-reload`

Form fields `name` and, from an agent's page, `key`. Sends `reload` and renders the page it came from, that agent's or the host's settings:

| Outcome | Shown |
|---|---|
| an answer | the toast "Reloaded: the runner answered" |
| no answer, `done`, or a wait that failed after the send | a warning that the outcome is unknown and may still take effect; nothing is sent again. If the page cannot be drawn around it, the warning is shown alone (502), never the generic "nothing was changed" |
| a refusal | the warning "The reload was not carried out", with the reason |

## Returning

An agent page opened from a runner keeps `return=/runner?name=…`: the record
detail, settings and save paths admit `/runner` beside the record's own list,
under the same local-path check ([shell § return](shell.md#web-face-shell)).
