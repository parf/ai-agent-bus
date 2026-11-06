# Backups: save your records, move them and bring them back

📌 **TL;DR:**
- **Problem:** rebuilding a bus by hand means re-creating every agent, setting, secret and token, and a raw copy of the host is all or nothing.
- **Outcome:** you save all records, or just the ones you pick, as one file encrypted to your enrolled SSH key, and restore it after seeing every conflict first.
- **Start:** as the bus Owner, run `agent-bus-admin export` on the bus host ([Try this first](#try-this-first)).

## Why you'd want it

- 🧱 **Re-creating a bus by hand is slow.** Now one file brings back records with their settings, secrets, stored values and tokens.
- 🎯 **A full host copy is all or nothing.** Now you pick records by name or tag, to move or keep.

## What you get

- ✅ **Full or partial export**, while the bus keeps running.
- ✅ **Encrypted to your key:** protect its private half and any decrypted copy.
- ✅ **A public `.info` sidecar** identifies a file export without revealing secret values.
- ✅ **Dry-run import** that checks the whole archive and lists every conflict.
- ✅ **All-or-nothing import:** the whole archive lands, or nothing does.
- ✅ **Move between hosts:** names can move to another realm on import.
- ⚠️ **Record exports omit queued messages**, held locks, bigfiles and login sessions. A [clean daemon restart](daemon.md#what-it-keeps) does retain waiting messages.
- 🚫 **Owner only:** nobody else can export or import.

## Which backup

| You want | Use | Takes |
|---|---|---|
| the daemon database and its saved state on this host | [stopped-daemon backup](../administration.md#stopped-daemon-backup) | a short stop; as root |
| every record with its settings, secrets, stored values and tokens, while the bus runs | `agent-bus-admin export` (full) | nothing; as yourself |
| some records, a tag or a few names, to move or keep | `agent-bus-admin export --include=…` (partial) | nothing; as yourself |

## You'll need

- To be the bus Owner, on the bus host, as your own login (no `sudo`).
- `age` installed on the bus host and on the restore host.
- An enrolled Ed25519 SSH key.

## Try this first

- See the export options without changing anything:

```sh
agent-bus-admin export --help
```

<details>
<summary>Export help, shortened</summary>

```text
USAGE
  agent-bus-admin export [--include=<selector>] [--exclude=<selector>] [--out=<path>|-]

DESCRIPTION
  Export permanent records, private settings, KV and credentials, encrypted to
  your enrolled Ed25519 key. Owner only, over your own local login socket; no
  sudo account handover. No selectors means full; users and groups are included
  by default. Use name: or tag: to disambiguate selectors; unknown or ambiguous
  bare selectors fail. Exclude wins. The daemon writes under
  /var/lib/agent-bus/record-backups by default. Every file export has an
  adjacent public .info; stdout needs --info for a sidecar.
...
```

</details>

- Then make a full backup:

```sh
agent-bus-admin export
```

- It prints the archive's path and its `.info` path.

## Export

- 📦 **Full:** no selector; users and groups are included.
- 🎯 **Partial:** any `--include` or `--exclude` makes it partial.
- Replace the example names and tags below with your own records.
- One tag: `agent-bus-admin export --include=tag:messaging`
- A tag minus one name: `agent-bus-admin export --include=tag:ops --exclude=name:'#old@myhost'`
- To a file of your own: `agent-bus-admin export --include=tag:ops --out=ops.tar.gz.age`
- `--exclude` wins over `--include`.
- Write `name:` or `tag:` when a word could be either; a word that matches both, or nothing, is refused.
- 📁 **Where it lands:** `/var/lib/agent-bus/record-backups/`, as `full-<host>-<time>.tar.gz.age` or `partial-<host>-<time>.tar.gz.age`.

## The `.info` file

- 🏷️ **File exports have a public sidecar** so central storage can identify them without the decryption key. For stdout exports, supply `--info=<path>`.
- It says whether the backup is full or partial.
- It names the host and version it came from.
- It says when it was made, and by whom.
- It lists the selection rules and the record names they matched.
- It names the key the archive is encrypted to.
- It gives the archive's SHA-256.
- 🔒 **It never holds** secret or token values.

### Moving to central storage

- The backup folder belongs to the bus's own account, so copy with `sudo`; the wildcard needs root too:

```sh
sudo sh -c 'cp /var/lib/agent-bus/record-backups/full-* /mnt/backups/'
```

- Set a retention policy for both archives and sidecars, and verify a restore before deleting your only usable copy.

## Import

- 👀 **Always look first.** `--dry-run` decrypts with your key, checks the whole archive and lists every conflict, changing nothing:

```sh
agent-bus-admin import full-<host>-<time>.tar.gz.age --dry-run
```

- 🚦 **Then answer every conflict;** the import refuses to start until each one has an explicit choice:

| Option | For names already on this bus |
|---|---|
| `--keep-existing=<sel>` | keep what is here; take the rest from the archive |
| `--replace=<sel>` | replace the record from the archive |
| `--replace-users=<sel>` | replace the user from the archive |
| `--replace-tokens=<sel>` | also replace their tokens; without it, tokens here stay |
| `--rename-realm=<from>=<to>` | move names to another realm; names never change on their own |

- `<sel>` is `name:…`, `tag:…`, a list, or `all`.
- Two choices that contradict each other on one record are refused.
- Keep everything already here: `agent-bus-admin import ops.tar.gz.age --keep-existing=all`
- Replace one tag and keep a named record: `agent-bus-admin import ops.tar.gz.age --replace=tag:ops --keep-existing=name:owner@demo`
- Every other conflict the dry run lists needs its own choice; `all` on one side would also match the tag on the other, and is refused.

## Rebuilding a bus from an export

1. Take a [stopped-daemon backup](../administration.md#stopped-daemon-backup), always.
2. `agent-bus-admin export` the records to keep.
3. Start a fresh bus with you as its Owner, and enroll your key on it; SSH enrollment and local account mappings are not in the archive.
4. `agent-bus-admin import <archive> --dry-run`, then the import with the choices it asks for.
5. Reconnect agents using the credentials retained or restored by your import choices. Check `agent-bus status` and `agent-bus ls --all --table`, then make a harmless call to each required agent.

- 🧩 **Runner host files are separate.** Preserve packages, inventory, work directories and host token files yourself. Exporting an agent record saves its bus-side Configuration and Secret; it does not copy those host files.

## Archive contents

<details>
<summary>What an archive holds, and the rest of the options</summary>

- **Holds:** records, access lists, Maintainers, roles, tags, group membership, Configuration, Secret, stored key-values, and current and previous persistent credentials.
- **Leaves out:** messages, locks, agent-event subscriptions, alert history and id reservations, the bigfile store (its index and bytes), login sessions, readers and the bus's own `#bus` record.
- **Encryption:** `age`, to your enrolled Ed25519 SSH public key.
- **Import key:** `--key=<file>`, default `$HOME/.ssh/id_ed25519`.
- **To stdout:** use `--out=-`; supply a sidecar as described [above](#the-info-file).
- **`.info` on import:** a missing one is allowed; one that does not match the archive is refused.
- **Separate folders:** record exports live apart from root's stopped-daemon backups in `/var/lib/agent-bus/daemon.backup/`.
- **More:** [encrypted record backups](../administration.md#encrypted-record-backups).

</details>
