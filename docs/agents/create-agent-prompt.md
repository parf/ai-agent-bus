# Create a new agent

📌 **TL;DR:** Paste the prompt below into your coding AI with this repository
available. Fill in the task and deployment inputs first; the checklists cover
implementation, metadata, permissions, tests and a verified installation.

## Prompt to paste

Implement a new AgentBus agent in this checkout. Read `CLAUDE.md` and the linked
owning sections before editing. Follow their current contracts; do not invent
new daemon APIs, privileges or wire formats to make this task fit.

| Input | Fill in before implementing |
|---|---|
| Job and expected answer | What the caller needs, with one successful call and one refusal |
| Name and deployment | Desired name, realm, host and Linux account; say whether live installation is authorized |
| Scope and access | Resources it may touch, read/write operations, callers and required roles |
| Settings and credentials | Configuration keys, Secret key names and where the operator obtains each credential; never paste values here |
| Dependencies | Existing service/API/client library and any host tools needed |

If an input is essential and cannot be inferred safely, ask one focused question.
Otherwise implement the smallest complete solution and state the assumption.

### Choose the serving kind

| Use | Choose when | Start from |
|---|---|---|
| Runner per-message script | One bounded request can start, answer and exit; no retained connection or own bus credential is needed | [Package a script](../runner/agents.md#package-a-script), [package options](../runner/configuration.md#package-options) |
| Serves-itself process | Keep subscriptions, connections or in-memory state, or make bus calls as this agent | [Self-serving agents](../runner/configuration.md#agents-that-serve-themselves), [client libraries](../../src/clients/README.md) |
| Standalone on the shared starter | Host access or an explicit Linux account is required outside the shared runner sandbox | [Standalone starter](../../src/agents/standalone/starter.md#interface); it can also supervise a self-serving process |

Use the shared runner or starter rather than copying supervision, token storage
or installation code. A self-serving process gets **its own** credential, never
the installer's. It consumes its inbox and replies through an existing client.

### Required decisions and checks

Check each item and record why it applies. A recommendation is not permission
to widen scope.

| Required | Reason and owning section |
|---|---|
| [ ] Choose `#name@realm`; add `/instance` only to distinguish a real configuration or account | Names are stable dependencies: [identity](../01-identity-and-authority.md#names) |
| [ ] Use lowercase ASCII and `-` for authored name components; refuse collisions and never repurpose a fixed name | A peer must not silently start addressing different work; runtime validation remains authoritative |
| [ ] Mark duplicate numbering and host-versus-organization realm policy **pending owner decision** | Do not turn an unsettled naming convention into a new rule |
| [ ] Write a useful, verb-first description under about 80 characters, without repeating the name, kind, “bus”, “agent”, project or repository | It appears beside the name on every listing: [description and help checklist](authoring.md#checklist-writing-an-agents-description-and-help) |
| [ ] Describe every actual method, argument, return and limit; keep methods in useful author order | Generated help must match the executable grammar: [package help](../runner/configuration.md#package-help) |
| [ ] Declare `body_formats` honestly, independently of runner `form` | `form` transports the envelope to the script; `body_formats` describes what the caller puts in its body |
| [ ] Keep non-secret settings in Configuration, with key help, examples, defaults and validation | Configuration is shown and handed to scripts: [configuring a template](../03-records-agent.md#configuring-a-template) |
| [ ] Keep credentials in Secret or private runner files; commit only names/comments in `env.dist`, never real values | Host files never leave through the runner API: [secrets](../runner/security.md#secrets) |
| [ ] Use only delivered envelope roles or `AB_ROLE_*`; enforce Owner/Maintainer access and any resource cap in code | A body cannot claim authority: [roles](../03-records-agent.md#roles), [trust boundary](../02-access.md#trust-boundary) |
| [ ] Treat method `role` arrays as any-of; spell out cumulative tiers explicitly | The renderer infers no role hierarchy; Owner and Maintainer are automatic, assigned roles grant access to others |
| [ ] Mark destructive methods and refuse them before side effects when scope or role is insufficient | Metadata warns callers; it does not enforce execution |
| [ ] Validate caller input before constructing fixed argument vectors; never interpolate it into a shell command or accept caller-supplied options | Keep execution inside the declared scope: [process boundary](../08-processes.md#the-rule) |
| [ ] Return expected errors as clear answers; do not use exit 1 for an ordinary refusal | A nonzero script exit produces no answer: [script outcomes](../runner/agents.md#package-a-script) |
| [ ] Bound every wait, retry, body, answer, count and allocation, and honor the message deadline | A busy dependency must not occupy a slot indefinitely: [deadlines](../04-messaging.md#request-and-reply), [package options](../runner/configuration.md#package-options) |
| [ ] Follow RD103: no ack if answering this turn; ack only for a later answer; never done after an answer; done only when no answer is expected; nothing for notices or receipts | Avoid extra messages: [receipts](../04-messaging.md#receipts); runner-executed scripts let the runner handle receipts |

| Recommended | Reason |
|---|---|
| [ ] Start per-message unless state or connections require otherwise | Less lifecycle code to own |
| [ ] Share repeated arguments; distinguish text syntax from JSON-only fields | Callers see the grammar once, without misleading positional arguments |
| [ ] Provide 2–4 real, copyable examples, including a refusal or dangerous operation only in a disposable fixture | Examples reveal mismatches before users find them |
| [ ] Keep dependencies and privileges minimal; state unavailable capabilities plainly | Reuse an existing service rather than creating an undocumented authority path |
| [ ] Keep application errors structured or consistently prefixed, with no credentials in output or logs | The agent owns its own answers, including their secrecy |

### Author the package

Use [package help](../runner/configuration.md#package-help) as the schema reference
and the [authoring checklist](authoring.md#checklist-writing-an-agents-description-and-help)
for prose. Include `agent.json`, the executable, a commented `env.dist` when keys
exist, tests and a small README. Metadata declares behavior; the handler still
validates every call.

| Metadata | What to author |
|---|---|
| Identity | `description`, SPDX `license`, `author`, `homepage`; `version` for an independently versioned package. Bundled agents omit `version` and use the shared version |
| Inputs and discovery | `body_formats`, `tags`; runner `command`, `form` and, when needed, `serves` |
| Methods | `help`, ordered `args` with `name`, types or enum arrays, `optional`, final `rest`, `json_only`, defaults; `returns`, `text`, `role`, `destructive`, `example` where applicable |
| Shared arguments | Top-level `arguments`; a method argument without `help` references its named definition |
| Configuration | Top-level `config`: key help, type/enum, example and default where useful; validate actual settings in the handler |
| Secrets | `env.dist` names with comments: purpose, where to obtain the credential, and required/optional status. An empty declaration is required for runner-executed scripts; for serves-itself it is metadata only |

#### Complete minimal example

This text-only runner script needs no credentials. Put these two files together,
make `hello.py` executable, and use an empty `env.dist`. Replace the example name
with the chosen deployment name before shipping. Use the repository's license
for bundled code; the identity fields below illustrate a separate package.

`agent.json`:

```json
{
  "command": "./hello.py",
  "form": "args",
  "description": "Check that a request reaches a script and returns a response",
  "license": "MIT",
  "version": "1.0.0",
  "author": "Example team",
  "homepage": "https://example.net/ping-demo",
  "body_formats": ["text"],
  "tags": ["diagnostics"],
  "methods": {
    "ping": {
      "help": "Return pong",
      "args": [],
      "returns": "pong",
      "example": "agent-bus call '#ping-demo@example.net' ping"
    }
  },
  "config": {}
}
```

`hello.py`:

```python
#!/usr/bin/env python3
import sys

body = sys.argv[1] if len(sys.argv) == 2 else ""
if len(body) > 1024:
    print("error: body exceeds 1024 characters")
elif body.strip() == "ping":
    print("pong")
else:
    print("error: expected ping")
```

The runner handles the four help asks from metadata; the script handles `ping`.
Do not add a hand-maintained second copy of its method prose.

#### Richer metadata example

This is a complete manifest for a proposed ticket handler, **not a shipped
implementation**. Implement and test the declared text and JSON grammar before
advertising it. `form: json` delivers an envelope on stdin; its `body` can still
be either text or an application JSON object.

```json
{
  "command": "./tickets.py",
  "form": "json",
  "description": "Search support tickets and close resolved work within configured queues",
  "license": "MIT",
  "version": "1.0.0",
  "author": "Example team",
  "homepage": "https://example.net/tickets",
  "body_formats": ["text", "json"],
  "tags": ["support"],
  "roles": ["reader", "writer"],
  "deadline": "10s",
  "arguments": {
    "queue": {"help": "Queue to use; must also be allowed by Configuration", "type": ["support", "billing"]},
    "id": {"help": "Ticket number", "type": "int"}
  },
  "methods": {
    "search": {
      "help": "Find matching tickets, returning numbers and subjects",
      "text": "search <queue> <query...>",
      "args": [
        {"name": "queue"},
        {"name": "limit", "help": "Maximum results, 1..50", "type": "int", "optional": true, "json_only": true, "default": 20},
        {"name": "query", "help": "Search text, at most 500 characters", "rest": true}
      ],
      "returns": "Ticket numbers and subjects",
      "role": ["reader", "writer"],
      "example": "agent-bus call '#tickets@example.net' search support login failure"
    },
    "close": {
      "help": "Close one resolved ticket and return its new state",
      "args": [{"name": "queue"}, {"name": "id"}],
      "role": "writer",
      "destructive": true,
      "example": "agent-bus call '#tickets@example.net' close support 42"
    }
  },
  "config": {
    "api_url": {"help": "HTTPS ticket API origin", "type": "string", "example": "https://tickets.example.net"},
    "queues": {"help": "Queues this instance may access", "type": "list", "example": ["support"]}
  }
}
```

`env.dist` (no credential value):

```dotenv
# Required ticket API credential; obtain it from the ticket service's API settings.
# Store the value in the record Secret or the runner's private env file.
TICKETS_API_TOKEN=
```

Both `search support login failure` and
`{"method":"search","queue":"support","query":"login failure","limit":10}`
are accepted only after the handler implements them. `close` checks delivered
Owner/Maintainer or `writer`, the configured queue scope and a valid positive
ID **before** calling the API. Metadata does none of those checks for it.
For a serves-itself variant, add `"serves":"itself"` and `"processes":1`, use a
client's serve/reply lifecycle and the shared manifest helper, and fetch its own
Secret; do not depend on `env.dist` injection or inherited human credentials.

### Implement and test

- [ ] Follow [mutation first, then belief](../../CLAUDE.md#mutation-first-then-belief): make a concrete bad implementation compile and fail a named assertion before trusting green.
- [ ] Test real calls, not just parsing: a valid call, bad arguments, missing role, out-of-scope resource, deadline and bounded output. Confirm refused work made no external change.
- [ ] Add the per-agent help-versus-grammar check: every handled method is described, every advertised text/JSON form parses, role filtering matches authority, and examples are valid. Help and `help-json` contain no Secret values.
- [ ] Use [debug's methods](debug.md#check-the-bus-in-a-minute) instead of a throwaway fixture when they exactly fit: `echo`, `roles`, `sleep`, `fail`, `crash`, `unicode`. Keep the original assertion and a mutant that proves it.
- [ ] Run focused checks, then the required full `src/smoke.sh --slow` under the repository's shared gate lock. Check free disk space first; follow `CLAUDE.md` for current gate coordination and cleanup.

### Package, install and verify

Use [runner installation](../runner/operations.md#installing-changing-and-removing-an-agent)
or the [shared standalone installer](../../src/agents/standalone/starter.md#interface).
A `package.sh <destination>` must include the executable, manifest, `env.dist`,
shared metadata helper and client/runtime dependencies; verify it outside the
checkout. Never copy credentials into a package.

- [ ] Inspect generated help with `agent-bus help '<chosen-name>'`, then `config`, `secret` and `json`; check actual roles and scope as permitted callers.
- [ ] Install or reload only the authorized target; preserve existing names, ACLs, Configuration and Secret. Use the agent's own credential for a self-serving process.
- [ ] Follow `CLAUDE.md` for version, commit, stamped build and deploy order; back up before a daemon change. Do not deploy destructive examples to a shared live resource.
- [ ] Check service/runner status, make one safe real call, and run `src/check-deploy.sh '#debug@<realm>'`. Any missing or failed selftest blocks completion.

### Before you say done

- [ ] The requested behavior works; ordinary failures answer clearly and side effects stay within scope.
- [ ] Names, metadata, descriptions, text/JSON grammar, roles, caps, settings and examples agree with the code.
- [ ] No credential appears in committed files, help, answers, test output or logs.
- [ ] Named compiling mutants failed assertions, required checks passed, and authorized live verification passed.
- [ ] Report the commit, checks, mutation results, installed name, safe live call and any explicit unavailable capability or pending naming decision.
- [ ] Remove your worktree and temporary outputs after the commit/report, preserving other people's work.
