# Debug: check the bus in a minute

📌 **TL;DR:**
- **Problem:** after an update, "is everything still working?" means poking at several programs by hand, and a quiet failure is easy to miss.
- **Outcome:** one call answers ✅ or ❌ for each part of the bus and for the other helpers on a server, without logging in.
- **Start:** ask it what it can do: `agent-bus help '#debug@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔎 **Nobody knows if the bus is healthy.** Now one call gives the same checks and one verdict every time.
- 🚀 **A deploy needs a yes or no.** Now the post-deploy check and your AI ask the same question.
- 🧩 **Odd symptoms are hard to pin down.** Now a big message, special characters or the roles actually delivered are one call away.
- 🔒 **A diagnostic tool must not leak.** Now its own password stays hidden in every answer.

## What it does

- ✅ **Bus check in a minute:** six quick checks, each ✅ or ❌, with the failing step named.
- ✅ **Server health overview:** asks the other helpers on the server for their health, and reports a refusal or a missing fact instead of guessing.
- ✅ **Known test messages:** a large answer, a special-character answer, a wait or an error, to compare against what your tool shows.
- ✅ **What the bus delivered:** your roles and the message fields exactly as they arrived.
- ✅ **Its own limits:** the restrictions and settings it runs with, secrets masked.
- 🚫 **Not for:** checking your own permissions (use `agent-bus may-send`), fixing anything, or monitoring over time.

The calls, last:

| Call | Does |
|---|---|
| `selftest` | Six quick bus checks, each ✅ or ❌ |
| `doctor` | Asks the other helpers on this server for their health |
| `ping`, `hello` | Answers, to show it is alive |
| `echo`, `roles` | Shows what the bus delivered |
| `sandbox`, `env` | Shows its own restrictions and settings, secrets masked |
| `sleep`, `fail`, `crash` | Waits, answers with an error, or kills itself (the runner restarts it) |
| `big`, `unicode` | Sends a large or special-character answer |

## You'll need

- A [person's runner](../07-setup.md#a-persons-runner) with Python 3. Nothing else: the runner gives debug its own credential.

## Try this first

- Ask it what it can do (read-only):

```sh
agent-bus help '#debug@node.example'
```

- See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

<details>
<summary>See it on a real node (ran today)</summary>

```sh
agent-bus call '#debug@parf.us' --wait 10s ping | jq -r .body
```

```text
{"received_at":1793347856.9322708,"answered_at":1793347856.9322953,"version":"0.9.172"}
```

- The two times are when the helper received and answered, so the gap is its own delay; `version` is the release it runs.
- The output is from the development node on 2026-10-30.

</details>

## Check the bus in a minute

```sh
agent-bus call '#debug@node.example' selftest
agent-bus call '#debug@node.example' doctor
```

- Any ❌ fails the [post-deploy check](../07-setup.md#post-deploy-check).
- `doctor full` and `doctor disks` add the raw facts.
- `selftest` exercises ping, echo, a short wait and an error locally, then debug's own KV and lock through the bus; your request and its reply are the external round trip.
- It does not test self-addressed bus calls.

## Use examples

| I want to… | Command |
|---|---|
| Check after a deploy | `src/check-deploy.sh` |
| Run the bus checks | `agent-bus call '#debug@node.example' selftest` |
| Get a health overview | `agent-bus call '#debug@node.example' doctor` |
| See raw disk facts | `agent-bus call '#debug@node.example' 'doctor disks'` |
| See delivered roles | `agent-bus call '#debug@node.example' roles` |
| Check I may send to a helper | `agent-bus may-send <name>`, as you |

<details>
<summary>More use cases, and why debug has no permission check</summary>

| Use case | What you do | What you get |
|---|---|---|
| Post-deploy check | Run `src/check-deploy.sh` | Pass or fail, with the failing step |
| Something is off | Ask `doctor` | One verdict per check: failed services, full disk, old versions, certificate |
| Disk or certificate warning | Ask `doctor full` or `doctor disks` | The raw facts behind the verdict |
| Wrong permissions arrive | Ask `roles` | The roles the bus actually delivered |
| Big or non-English messages break | Ask `big 64` or `unicode` | A known test message to compare |

- There is no `can` method: debug cannot act as you, so a check made as debug would only describe debug's own access.

</details>

## Installing it

- Debug [serves itself](../runner/configuration.md#agents-that-serve-themselves), so the runner never registers it.
- Package it into the runner's code folder, register it as its Owner, hand the runner its token once, then reload the runner.
- Keep its access list at `@owner`.

<details>
<summary>Step by step</summary>

For `alice`'s runner on `node.example`:

```sh
src/agents/debug/package.sh /var/lib/agent-bus/agent.d/debug   # a release: copy /usr/local/lib/agent-bus/current/agents/debug
agent-bus register '#debug@node.example' --allow=@owner
agent-bus token '#debug@node.example' | sudo install -o alice-ab-runner -m 600 /dev/stdin \
  '/var/lib/agent-bus/alice-ab-runner/tokens/%23debug@node.example'
agent-bus call '#runner/alice@node.example' reload
```

- Until it holds the token, the runner refuses debug by name and says where the token goes.
- Later changes need only the package and a reload ([deploy only the changed component](../07-setup.md#deploy-only-the-changed-component)).
- The package holds the Python bus client, metadata helper and version; copy the whole package, never only `debug.py`.
- With explicit arguments `debug.py` runs one method and exits; KV, selftest and doctor need the serving process.

</details>

## Configuration

None.

## Secret keys

None. Its bus credential comes from the runner and stays masked in every answer.

## Who may use it

- Keep the access list at `@owner`: `crash` deliberately kills debug.

| Call | Requires |
|---|---|
| `selftest` and the other probes | The access list only, so a session agent can run the post-deploy check |
| `doctor`, `doctor full`, `doctor disks` | A delivered Owner or Maintainer role for debug itself, even with a wider access list; a role carried from another forwarded record never counts |

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#debug` reports these.

| Level | When | What to do |
|---|---|---|
| alert | A `selftest` step fails (❌) | Read the step's line; the bus function it names is broken |
| alert | `crash` was called: the runner records the kill | Expected after a deliberate `crash`; otherwise find who called it |
| warning | `fail` was called: its fixed diagnostic line | Expected after a deliberate `fail` |
| warning | `doctor` could not ask a helper (`not allowed`, `unavailable`) | Allow debug on that helper if the check matters |

## Troubleshooting

| If you see | Do this |
|---|---|
| ❌ on a selftest step | That bus function failed and the post-deploy check fails with it; read the step's line |
| `not allowed: <agent>` from doctor | Debug is not on that helper's access list; grant it there if the check matters |
| `unavailable: <reason>` | Check unsupported or facts missing; nothing was guessed |
| "none installed" | No visible instance of that helper in debug's realm |
| Doctor refused, selftest works | Doctor needs a delivered Owner or Maintainer role for debug itself |

## Technical details

<details>
<summary>Methods</summary>

- Forms: `hello [name]`, `sleep <seconds>` (0–30, also bounded by the message deadline), `fail [code]` (1–255, plus a fixed stderr diagnostic), `crash` (SIGKILL without a core dump, no answer), `big <KB>` (1–64 KiB of ASCII), `unicode` (fixed Cyrillic, emoji, newline, tab and a control byte), `doctor [mode]` with mode `full` or `disks`.
- `env` shows names and masked values: the delivered fields, roles and the bus address visible; the token, the Configuration and every other value masked.
- `sandbox` shows observed uid/gid, restrictions, limits and path writability. `selftest` runs one at a time.
- Input up to 16 KiB. The installed `agent.json` is the method reference (`--help`, `--help-json`); `--help-config` and `--help-secret` describe declarations only.

</details>

<details>
<summary>How doctor decides</summary>

- Doctor finds visible helpers in its own realm by exact template name or template/instance prefix; systemd by its shared capability tag. It never assumes a default name and says "none installed" when none is visible.
- Up to 32 targets and 30 seconds in total; two seconds and 4096 characters per remote answer; structured facts capped at 256 KiB and 1000 disk/unit rows. It never forwards caller-chosen commands.
- Verdicts: the fullest filesystem warns at 85% and fails at 95%; any failed unit is a warning, instances counted by template prefix.
- Versions are compared from help-json, missing ones marked unavailable; [server-info TLS facts](server-info.md#tls-certificate) flag fewer than 30 days or a pin mismatch. Debug never reads the host certificate.

</details>
