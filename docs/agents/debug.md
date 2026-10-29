# Debug: check the bus in a minute

📌 **TL;DR:** Debug is a health check for the bus. One call tells you whether
the bus, the runner (the program that hosts helpers) and the other helpers on
a server are working, without logging in. Only the owner should be able to
call it, because one of its tests crashes it on purpose.

## Why you'd want it

**The problem.** After an update, "is everything still working?" usually means
poking at several programs by hand, and a quiet failure is easy to miss.

**The idea.** A small helper that runs the same checks every time and answers
with a ✅ or ❌ per step. The post-deploy check and your AI use the same call,
so there is one answer to "is the bus healthy?". Its own password stays hidden
in every answer.

**When to use it**

- Right after a deploy or a configuration change.
- When something feels off and you want a quick overview.
- When diagnosing message size, text encoding or permission oddities.

**When not to:** checking your own permissions (use `agent-bus may-send`),
fixing anything, or monitoring over time.

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| Post-deploy check | Run `src/check-deploy.sh` | Pass or fail, with the failing step |
| Something is off | Ask `doctor` | One verdict per check: failed services, full disk, old versions, certificate |
| Disk or certificate warning | Ask `doctor full` or `doctor disks` | The raw facts behind the verdict |
| Wrong permissions arrive | Ask `roles` | The roles the bus actually delivered |
| Big or non-English messages break | Ask `big 64` or `unicode` | A known test message to compare |

## What it does

| Call | Does |
|---|---|
| `selftest` | Six quick bus checks, each ✅ or ❌ |
| `doctor` | Asks the other helpers on this server for their health |
| `ping`, `hello` | Answers, to show it is alive |
| `echo`, `roles` | Shows what the bus delivered |
| `sandbox`, `env` | Shows its own restrictions and settings, secrets masked |
| `sleep`, `fail`, `crash` | Waits, answers with an error, or kills itself (the runner restarts it) |
| `big`, `unicode` | Sends a large or special-character answer |

Doctor asks other helpers as debug, with no special access; a refusal or a
missing fact is reported, never guessed.

<details><summary>Technical details: methods and doctor</summary>

- Method forms: `hello [name]`, `sleep <seconds>` (0–30, also bounded by the message deadline), `fail [code]` (1–255, plus a fixed stderr diagnostic), `crash` (SIGKILL without a core dump, no answer), `big <KB>` (1–64 KiB of ASCII), `unicode` (fixed Cyrillic, emoji, newline, tab and a control byte), `doctor [mode]` with mode `full` or `disks`. `env` shows names and masked values; bus fields and roles visible, the token always masked. `sandbox` shows observed uid/gid, restrictions, limits and path writability. `selftest` runs one at a time.
- Input up to 16 KiB. The installed `agent.json` is the method reference (`--help`, `--help-json`); `--help-config` and `--help-secret` describe declarations only.
- Doctor finds visible helpers in its own realm by exact template name or template/instance prefix; systemd by its shared capability tag. It never assumes a default name and says "none installed" when none is visible. Up to 32 targets and 30 seconds in total; two seconds and 4096 characters per remote answer; structured facts capped at 256 KiB and 1000 disk/unit rows. It never forwards caller-chosen commands.
- Verdicts: the fullest filesystem warns at 85% and fails at 95%; any failed unit is a warning, instances counted by template prefix; versions compared from help-json, missing ones marked unavailable; [server-info TLS facts](server-info.md#tls-certificate) flag fewer than 30 days or a pin mismatch. Debug never reads the host certificate.

</details>

## You'll need

- A [person's runner](../07-setup.md#a-persons-runner) with Python 3. Nothing else: the runner gives debug its own credential.

## Try this first

```sh
agent-bus help '#debug@node.example'
```

See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

## Check the bus in a minute

```sh
agent-bus call '#debug@<node>' selftest
agent-bus call '#debug@<node>' doctor
```

Any ❌ fails the [post-deploy check](../07-setup.md#post-deploy-check).
`doctor full` and `doctor disks` add the raw facts.

<details><summary>Technical details: what selftest covers</summary>

Selftest exercises ping, echo, a short wait and an error locally, then debug's own KV and lock through the bus; your request and its reply are the external round trip. It does not test self-addressed bus calls (RT21).

</details>

## Use examples

| I want to… | Command |
|---|---|
| Check after a deploy | `src/check-deploy.sh` |
| Run the bus checks | `agent-bus call '#debug@node.example' selftest` |
| Get a health overview | `agent-bus call '#debug@node.example' doctor` |
| See raw disk facts | `agent-bus call '#debug@node.example' 'doctor disks'` |
| See delivered roles | `agent-bus call '#debug@node.example' roles` |
| Check I may send to a helper | `agent-bus may-send <name>` (RT20), as you |

<details><summary>Why debug has no permission check</summary>

There is no `can` method: debug cannot act as you, so a check made as debug would only describe debug's own access.

</details>

## Installing it

Package it into the runner's code folder and reload the runner, as in
[deploy only the changed component](../07-setup.md#deploy-only-the-changed-component).
Keep its access list at `@owner`.

<details><summary>Step by step</summary>

```sh
src/agents/debug/package.sh /var/lib/agent-bus/agent.d/debug
```

- Then reload the runner, which grants debug its own credential.
- The package holds the Python bus client, metadata helper and version; copy the whole package, never only `debug.py`.
- With explicit arguments `debug.py` runs one method and exits; KV, selftest and doctor need the serving process.

</details>

## Configuration

None.

## Secret keys

None. Its bus credential comes from the runner and stays masked in every answer.

## Who may use it

Keep the access list at `@owner`: `crash` deliberately kills debug.

| Call | Requires |
|---|---|
| `selftest` and the other probes | The access list only, so a session agent can run the post-deploy check |
| `doctor`, `doctor full`, `doctor disks` | A delivered Owner or Maintainer role for debug itself, even with a wider access list; a role carried from another forwarded record never counts |

## Troubleshooting

| If you see | Do this |
|---|---|
| ❌ on a selftest step | That bus function failed and the post-deploy check fails with it; read the step's line |
| `not allowed: <agent>` from doctor | Debug is not on that helper's access list; grant it there if the check matters |
| `unavailable: <reason>` | Check unsupported or facts missing; nothing was guessed |
| "none installed" | No visible instance of that helper in debug's realm |
| Doctor refused, selftest works | Doctor needs a delivered Owner or Maintainer role for debug itself |
