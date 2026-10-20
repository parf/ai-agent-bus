# Create a new agent

📌 **TL;DR:** Paste the stub into your coding AI with this repository available.
This reference supplies the workflow, contracts and completion checks; fill in
the job, scope and deployment inputs before implementation.

## Prompt to paste

```text
Read docs/agents/create-agent-prompt.md and follow its checklists.
Job and expected answer:
Proposed name and owner-provided realm:
Resources and read/write scope:
Callers and application roles:
Configuration keys and credential names (never values):
Dependencies and required Linux account:
Live deployment authorized? Target:
Finish with the commit, mutation evidence and safe live verification.
```

## Plan and choose a kind

Read [CLAUDE.md](../../CLAUDE.md#working-rules). For a bundled agent, first add an
[R1 TODO row](../../Plans/R1.0-Release/TODO.md#bundled-agents) with scope and falsifiable
acceptance checks, and get the owner's OK before implementation. Resolve essential
missing inputs with one focused question.

Use `git worktree add --detach tmp/<task> HEAD`; work there, never directly in
the live checkout. On the development node, installed commands link to that
checkout. Preserve others' edits and follow the repository's commit/deploy order.

| Kind | Choose when | Owning instructions |
|---|---|---|
| Runner per-message script | One bounded request starts, answers and exits | [Package a script](../runner/agents.md#package-a-script) |
| Serves-itself | Retain state/connections or make bus calls with its own credential | [Self-serving agents](../runner/configuration.md#agents-that-serve-themselves) |
| Standalone on shared starter | Host access or an explicit Linux account is needed | [Starter](../../src/agents/standalone/starter.md#interface), including its long-lived mode |

Reuse supervision and [client libraries](../../src/clients/README.md). A standalone
manifest omits `command`: the starter receives the executable through `--script`.

## Required checklist

- [ ] If help depends on runtime Configuration, set `"help":"native"` and implement all four help asks; generic metadata cannot apply runtime caps/scope. See [package help](../runner/configuration.md#package-help).

| Check | Reason and contract |
|---|---|
| [ ] Use `#name@realm`, with `/instance` only when it distinguishes a real configuration/account | Stable [names](../01-identity-and-authority.md#names) are dependencies; never repurpose one |
| [ ] Prefer lowercase ASCII and `-`; use the realm the owner gives; if the name exists, stop and ask | Follow the linked grammar, refuse collisions, and do not invent duplicate numbering |
| [ ] Write a verb-first description of useful work/scope, under about 80 characters | Repeating the adjacent name, kind, “bus”, “agent”, project or repo wastes attention: [authoring](authoring.md#checklist-writing-an-agents-description-and-help) |
| [ ] Describe every method, return, argument, enum, default and limit; share repeated arguments and order methods by use | Generated help must match executable grammar: [package help](../runner/configuration.md#package-help) |
| [ ] Declare `body_formats` honestly, independently of runner `form` | One describes accepted body strings; the other transports the envelope to the script |
| [ ] Keep config in Configuration and credentials in Secret or private runner files | Configuration is shown and injected as `AGENT_BUS_CONFIG`; [configuration](../03-records-agent.md#configuring-a-template), [secrets](../runner/security.md#secrets) |
| [ ] Check only delivered envelope roles or `AB_ROLE_*`; Owner/Maintainer grant access within resource caps; role arrays are any-of | Assigned roles grant others access; no inferred hierarchy or body-claimed authority: [roles](../03-records-agent.md#roles) |
| [ ] Mark destructive methods, then enforce roles, scope and validated fixed argv before effects | Metadata cannot enforce the [process boundary](../08-processes.md#the-rule) |
| [ ] Answer ordinary errors clearly, bound waits/retries/sizes/counts, and honor deadlines | A nonzero script exit sends no answer; [outcomes](../runner/agents.md#package-a-script), [deadlines](../04-messaging.md#request-and-reply) |
| [ ] No ack when answering this turn; ack only for a later answer; never done after an answer; done only when no answer is expected; nothing for notices/receipts | [Receipt rules](../04-messaging.md#receipts); runner-executed scripts leave receipts to the runner |

## Package and examples

Follow the [authoring checklist](authoring.md#checklist-writing-an-agents-description-and-help),
including its recommendations. Author `agent.json`, executable, tests and README.
Use SPDX `license`, `author`, `homepage`, `tags`, ordered `methods`, shared
`arguments` and `config`. Set `version` for an independent package; bundled agents
omit it and use the shared version. Method arguments support types/enum arrays,
`optional`, final `rest`, `json_only` and defaults; methods support text usage,
roles, destructive marks and real examples. Configuration keys need help/examples.

Bundled agents declare a `help` method and handle body `help` with the shared
manifest helper, never duplicated prose. The runner intercepts packaged help asks;
self-serving agents answer all four through their language helper. An independent
package must not declare body `help` unless it implements it.

### Complete minimal manifest

Implement `ping` → `pong`, `echo <text...>` → the text, and generated body `help`.
Make `ping.py` executable. No keys are needed, so **omit `env.dist`**.

```json
{
  "command": "./ping.py", "form": "args",
  "description": "Answer pings and echo text for delivery checks",
  "license": "PolyForm-Noncommercial-1.0.0",
  "author": "Serg Parf and agent-bus contributors",
  "homepage": "https://github.com/parf/ai-agent-bus",
  "body_formats": ["text"], "tags": ["diagnostics"],
  "methods": {
    "ping": {"help": "Return pong", "args": [], "example": "agent-bus call '#ping-demo@example.net' ping"},
    "echo": {"help": "Return text unchanged", "args": [{"name": "text", "help": "Text to return", "rest": true}], "example": "agent-bus call '#ping-demo@example.net' echo hello"},
    "help": {"help": "Describe available methods and settings", "args": []}
  },
  "config": {}
}
```

Use the real [systemd manifest](../../src/agents/standalone/systemd/agent.json)
as the rich example of shared arguments, typed/enum arguments, role unions,
destructive operations, examples and Configuration. Copy the structure, not its
privileges or host commands. Use [debug's package](../../src/agents/debug/package.sh)
as a helper/client dependency packaging example.

For a proposed tickets variant, merge `{"needs":{"network":["tickets.example.net"]}}`
into its runner manifest. This **opens network access**, not a restriction to that
one host; see [package options](../runner/configuration.md#package-options).
A copyable application-JSON call would be:

```sh
agent-bus call '#tickets@example.net' '{"method":"search","queue":"support","query":"login failure","limit":10}'
```

`Envelope.Body` is always a **string**. With runner `form: json`, decode the
envelope, then decode its body only for an advertised JSON body format:

```python
envelope = json.load(sys.stdin)
body = envelope["body"]
call = json.loads(body)  # JSON application body; text takes a separate parser
```

Implement that grammar before declaring `json`; return parsing errors as answers.
In commented `env.dist`, explain each key and where to obtain it. An empty
`KEY=` means **required**, not an empty default; never include credentials.
For serves-itself, `env.dist` is metadata only: no defaults or requirements are
injected/enforced, and the process fetches its own bus Secret.

## Self-serving credential handoff

The runner **never registers** a serves-itself agent and refuses it until its
own token is in `runner/tokens/`. As an authorized installer, register the new
name, then provision it once. Resolve `RUNNER`, `RUNNER_DIR` and `RUNNER_USER`
from the actual runner status/service; do not guess paths or accounts.
See [what runs as what](../runner/security.md#what-runs-as-what).

```sh
agent-bus register '#tickets@example.net' --kind agent --allow '@owner'
sudo install -d -m 700 -o "$RUNNER_USER" "$RUNNER_DIR/tokens"
sudo install -m 600 -o "$RUNNER_USER" /dev/null "$RUNNER_DIR/tokens/%23tickets@example.net"
agent-bus-token '#tickets@example.net' | sudo tee "$RUNNER_DIR/tokens/%23tickets@example.net" >/dev/null
agent-bus call "$RUNNER" reload
```

The starter's own long-lived mode handles its credential lifecycle;
do not copy a human token into either package or process environment.

## Before you say done

- [ ] Prove valid calls, refusals, roles/caps, deadlines and output bounds; every compiling mutant must fail a named assertion ([mutation first](../../CLAUDE.md#mutation-first-then-belief)).
- [ ] Add help-versus-grammar and example checks, including bundled `help`; use exact-fit [debug fixtures](debug.md#check-the-bus-in-a-minute) such as sleep/fail/crash/echo/roles/unicode rather than throwaway scripts. Avoid core-generating signals.
- [ ] Run focused checks, then `src/smoke.sh` under the [shared gate lock](../../CLAUDE.md#verification); check disk space first.
- [ ] Verify `package.sh` includes metadata, helpers/client dependencies and licenses outside the checkout; install only the authorized target, preserving ACLs, Configuration and Secret.
- [ ] Commit, use the stamped build/deploy process, and back up before daemon changes. Check `agent-bus help '<name>'` plus `config`, `secret`, `json`, a safe real call, service status and **`src/check-deploy.sh`** (no argument needed).
- [ ] Report commit, mutant/check/live results and unavailable capabilities; ensure no credential leaked, then remove your worktree and scratch outputs.
