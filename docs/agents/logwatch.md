# Logwatch

## Why you'd want it

Find the clues behind an outage without opening a separate server window. Your
AI can collect recent errors, watch what happens during a repair, and send new
alerts to one inbox.

## What it does

Logwatch keeps a short, recent history of log lines from one server. Each line
names its file or Linux service. It can search that history and forward new
matching lines to another helper or inbox.

📌 **TL;DR:** Ask logwatch for recent lines, mark a point before an operation,
then ask what appeared afterwards. An administrator chooses which logs it may
read and who may ask. It never changes a service or its log files.

## You'll need

- An installed bus and Python 3 on the server.
- An administrator to install the helper.
- A Linux account allowed to read the chosen logs.
- Permission to call the helper. Only people you allow can read these logs;
  changing an alert destination also needs permission to manage that destination.

## Try this first

See what the installed helper can do:

```sh
agent-bus help '#logwatch@parf.us'
```

See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do)
for its Configuration and Secret help, JSON output, and script forms
`--help-config` and `--help-secret`.

## Ask your AI

Your AI session must be connected to the bus and allowed to use logwatch.

| Say this | What the AI calls |
|---|---|
| “Show me the last five log lines on parf.us.” | `tail 5` |
| “Find errors received in the last five minutes.” | `since 5m`, then inspect the result; `grep /error/i` searches all retained lines |
| “Mark the logs, restart the web service if I'm allowed, then show what appeared.” | `mark before`, permitted systemd `restart`, then `after before` |
| “Send new ERROR lines to my session.” | `sub add` with your session name and `ERROR`, when you manage that destination |

## Use examples

These examples use `#logwatch@parf.us`. Replace that with your installed name.

### Look for an outage

```sh
agent-bus call '#logwatch@parf.us' 'tail 5'
agent-bus call '#logwatch@parf.us' 'grep /error/i 20'
```

You see recent lines, oldest first, with a source such as
`[/var/log/syslog]` or `[journal:agent-bus-web.service]`. The second call shows
only matching lines. A plain `tail` never adds an error filter.

### Check a repair

```sh
agent-bus call '#logwatch@parf.us' 'mark before'
agent-bus call '#systemd@parf.us' 'restart agent-bus-web.service'
agent-bus call '#logwatch@parf.us' 'after before'
```

The last call shows only lines received after the mark. Restarting needs the
[systemd permission](systemd.md#who-may-use-it); logwatch itself needs no
extra action permission. Give concurrent repairs different mark names.

### Watch one inbox for errors

```sh
agent-bus call '#logwatch@parf.us' 'sub add #claude/main@parf.us ERROR'
agent-bus call '#logwatch@parf.us' 'sub list'
```

New matching lines go to that session, if you manage it and it allows logwatch
to send. To stop, copy the returned subscription id:

```sh
agent-bus call '#logwatch@parf.us' 'sub delete a12b34c56d78'
```

The first occurrence is sent straight away. Identical lines from the same
source are counted together; a summary such as `×37 in 5 min` follows when the
five-minute window closes. A queue or publish-and-subscribe inbox can also be
a destination, subject to the same access checks.

### Check what is being watched

```sh
agent-bus call '#logwatch@parf.us' sources
agent-bus call '#logwatch@parf.us' stats
agent-bus call '#logwatch@parf.us' '{"method":"since","age":"5m","n":50}'
```

`sources` shows files, excluded paths, journal sources and read errors.
`stats` shows how full the history is and how many older lines it has replaced.
The JSON call returns structured lines received during the last five minutes.
Here “received” means when logwatch read the line, not a date inside its text.

## Installing it

Follow [standalone connection setup](README.md#standalone-helpers) first.
Run these steps from the checkout root. No external bot token is needed.

1. Install with an account that can read the chosen logs. This example explicitly
   uses the Linux administrator account:

   ```sh
   sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
     ./src/agents/standalone/logwatch/install.py install \
     --name '#logwatch@parf.us' --user root --allow '@owner' \
     --bus-binary /usr/local/bin/agent-bus
   ```

   You should see `agent-bus-logwatch.service` installed. It keeps running
   between calls. Your personal credential is used only to install it; the
   helper stores its own bus credential.

2. Check it:

   ```sh
   agent-bus call '#logwatch@parf.us' sources
   agent-bus call '#logwatch@parf.us' 'tail 5'
   ```

   You should see source information and recent lines, or `(no lines)` if the
   chosen sources are empty.

3. Stop keeping your personal token in the shell:

   ```sh
   unset AGENT_BUS_TOKEN
   ```

   The installed service continues with its own credential.

For a narrower Linux account, omit `--user root` to create `ab-logwatch`.
Give it read access to the selected files. For the system journal, an admin
can add it to `systemd-journal` (or `adm` on systems that use that group), then
restart the helper:

```sh
sudo usermod -aG systemd-journal ab-logwatch
sudo systemctl restart agent-bus-logwatch
```

That group must exist on your host. Group membership does not automatically
make every application log readable. Check `sources` for permission errors.

To remove it, obtain your current installation credentials again and run:

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  ./src/agents/standalone/logwatch/install.py remove --name '#logwatch@parf.us'
```

You should see the service and bus name removed. Other log readers, including
an existing `rd-tail-log`, are left alone.

## Configuration

Keep settings in the record's **Configuration**. Do not put passwords, keys or
bot tokens in logwatch settings.

This full sample uses comments to explain each setting. **Remove comments
before saving it as JSON.** Omitted settings use the values shown here.

```jsonc
{
  "sources": [
    "/rd/var/log/**/*.log",       // Include logs in subfolders too.
    "syslog",                    // Use /var/log/messages or /var/log/syslog.
    "/var/log/nginx/*error*log"
  ],
  "excludes": [
    "/rd/var/log/realmo.com_ai-query.log",
    "/rd/var/log/cron/realmo.com_ai-query.log",
    "/rd/var/log/realmo.com_ai-search-query.log",
    "/rd/var/log/cron/realmo.com_ai-search-query.log",
    "/rd/var/log/realmo.com_ai-*.log",
    "/rd/var/log/cron/realmo.com_ai-*.log",
    "/rd/var/log/realmo.com_realmo-ai-property-type.log",
    "/rd/var/log/realmo.com_realmo-ai-*.log",
    "/rd/var/log/cron/realmo.com_realmo-ai-*.log",
    "/var/log/nginx/paperhouses-error.log"
  ],
  "journal": [],                 // Or ["agent-bus-web.service"], or "all".
  "ring_size": 2000,             // Maximum number of recent lines to keep.
  "line_bytes": 16384,           // Per-line byte cap, including truncation marker.
  "ring_bytes": 8388608,          // Retained source/text byte cap; oldest evicted first.
  "startup_bytes": 65536,        // Tail window read from each file at startup.
  "rescan_interval": 600         // Seconds between searches for new files.
}
```

Oversized lines keep their head and end with `…[truncated N bytes]`; the marker
counts toward `line_bytes`. Partial lines are bounded while waiting for a
newline. The ring stops at either the line count or the source/text byte cap;
Python object overhead is additional. Startup keeps up to ten complete lines
from the final `startup_bytes`, dropping a first line cut by that window.
An oversized journal JSON record keeps its bounded raw head under
`journal:unknown`, since its truncated fields cannot be decoded reliably.
The per-line cap also bounds each queued entry; queues retain their existing
count limit. A poll reads at most 64 KiB per file before splitting it.
Configuration bounds are listed by `agent-bus help '#logwatch@parf.us' config`.

For example, save this smaller, valid configuration to watch the web journal:

```sh
printf '%s\n' '{"sources":[],"excludes":[],"journal":["agent-bus-web.service"],"ring_size":2000,"rescan_interval":600}' \
  | agent-bus config '#logwatch@parf.us' -
sudo systemctl restart agent-bus-logwatch
```

You should now see `journal:agent-bus-web.service` in `sources`. A restart is
needed after changing settings. `"all"` reads everything that account may read;
it does not grant access to private journals.

On start, each existing nonempty file contributes its last ten lines. A newly
found file starts at its end, so old contents are not sent as new alerts. Empty
files are skipped until they have content. Terminal colour codes are removed
and hidden control characters become `...`, using the original tail-log cleaner.

History and marks are kept only while the helper runs. A restart clears them;
older lines also disappear when the history fills. If a mark is too old,
`after` tells you rather than presenting an incomplete repair history as complete.
Subscriptions survive restart. Failed alert sends are reported by `sub list`;
they are not replayed later.

## Secret keys

None: logwatch needs no Secret keys.

## Who may use it

There are no action roles for logwatch. Its access list decides who may call
it. Logs may contain private information. Use the owner-only access list in
[the install steps](#installing-it) when creating the record.

To add a trusted session, include it explicitly:

```sh
agent-bus manage '#logwatch@parf.us' --add-allow '#claude/main@parf.us'
```

You should then be able to call from that session. A caller changing a
subscription must manage its destination. The logwatch record's Owner and
Maintainers may also change subscriptions when calling it directly.

## Troubleshooting

| If you see… | Do this |
|---|---|
| `(no lines)` | Check `sources`; choose a readable source and wait for a new line. |
| A permission error | Give the service account read access, or select an appropriate account at install time. Restart after group changes. |
| A journal source with an error | Check `journalctl -u <unit>` as that Linux account; fix the reported permission or unit problem. |
| A newly created file missing | Wait for the configured rescan interval; only lines appended after discovery are read. |
| An unknown or expired mark | Create a new mark before the next operation. Marks do not survive restart or unlimited history. |
| A refused call | Ask the administrator to check the helper's access list. |
| A refused subscription | Check who manages the destination and whether it accepts messages from logwatch. |
| A filter error | Try plain text first. Regular expressions use `/pattern/i`; long-running patterns are refused. |

Implementation and verification notes: [source README](../../src/agents/standalone/logwatch/README.md).
