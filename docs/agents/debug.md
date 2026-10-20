# Debug: check the bus in a minute

📌 **TL;DR:** Debug is a bounded bus doctor running inside the runner sandbox,
with its own agent credential. Keep its ACL at `@owner`: it can deliberately
crash itself for a restart check. Private environment values and its credential
stay masked.

## Try this first

See what the installed helper can do:

```sh
agent-bus help '#debug@parf.us'
```

See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do)
for its Configuration and Secret help, JSON output, and script forms
`--help-config` and `--help-secret`.

## Check the bus in a minute

```sh
agent-bus call '#debug@<node>' selftest
agent-bus call '#debug@<node>' doctor
```

Selftest exercises ping, echo, a short wait and an error through the local handler,
then checks debug’s own KV and lock through the bus. Your selftest request and
its reply are the external round trip. It does not claim to test self-addressed
bus calls (RT21). Every step says ✅ or ❌; any ❌ fails the
[post-deploy check](../../src/check-deploy.sh). Doctor asks the other local
agents as debug, without host access or an ACL bypass. A refusal is reported as
`not allowed: <agent>`; an unsupported check is `unavailable: <reason>`.

### Ask your AI

| Say this | What it will call |
|---|---|
| “Check that the bus and runner are healthy.” | Debug `selftest` |
| “Check failed units, disks and Telegram connectivity.” | Debug `doctor` |
| “Check which roles this call delivers.” | Debug `roles` |
| “Can I send to this agent?” | `agent-bus may-send <name>` (RT20), as you |

Debug cannot impersonate you. There is no debug `can` method: a permission
check authenticated as debug would only describe debug's own access.

## Methods and limits

The installed `agent.json` is the method reference: ask `--help` or
`--help-json`. `--help-config` and `--help-secret` describe declarations only.

| Method | Answer |
|---|---|
| `hello [name]` | Greeting, for existing setup checks |
| `echo` / `roles` | Delivered envelope fields / daemon-delivered roles and origin |
| `ping` | Receive and answer timestamps plus version |
| `sleep <seconds>` | Wait 0–30 seconds, bounded by the message deadline |
| `fail [code]` | Error answer and fixed stderr diagnostic; integer 1–255 |
| `crash` | SIGSEGV without a core dump; no answer, runner restarts debug |
| `sandbox` | Observed uid/gid, restrictions, limits and path writability |
| `env` | Names and masked values; bus fields/roles visible, token always masked |
| `selftest` | Six bounded bus checks; one selftest at a time |
| `doctor` | Fixed queries to local server-info, systemd, logwatch and Telegram |
| `big <KB>` | 1–64 KiB of ASCII, for size checks |
| `unicode` | Fixed Cyrillic, emoji, newline, tab and a control byte |

Input is limited to 16 KiB. Doctor caps each remote answer at 4096 characters
and each call at two seconds; it never forwards caller-selected commands.
Doctor compares the versions components return through help-json; missing answers
are marked unavailable. TLS certificate expiry/pin is unavailable until an
existing agent exposes that check. No privilege is added.

## Install and test use

Run `src/agents/debug/package.sh <agent.d/debug>`, then reload the runner.
The package contains the Python bus client, metadata helper and version. Keep
the record's allow list at `@owner`; the runner grants debug its own credential.
No configuration or Secret is required.

With explicit arguments, `debug.py` also runs one method and exits. Tests may
use that mode where it exactly replaces a throwaway script. KV, selftest and
doctor require the serving process. A standalone fixture must copy the whole
package, not only `debug.py`.
