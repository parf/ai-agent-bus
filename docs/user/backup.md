# Backups: save your records, move them and bring them back

📌 **TL;DR:**
- **Problem:** rebuilding a bus by hand means re-creating every agent, setting, secret and token, and a raw copy of the host is all or nothing.
- **Outcome:** you save all records, or just the ones you pick, as one encrypted file only your SSH key opens, and restore it after seeing every conflict first.
- **Start:** as the bus Owner, run `agent-bus-admin export` on the bus host ([Try this first](#try-this-first)).

## Why you'd want it

- 🧱 **Re-creating a bus by hand is slow.** Now one file brings back records with their settings, secrets, stored values and tokens.
- 🔑 **A backup file holds secrets.** Now it is encrypted to your SSH key; a stolen copy is useless.
- 🎯 **A full host copy is all or nothing.** Now you pick records by name or tag, to move or keep.
- 🙈 **A restore can overwrite what is there.** Now a dry run lists every conflict, and nothing changes until you answer each one.
- 🗂️ **Old backup files are a mystery.** Now an open `.info` file beside each one says what is inside.

## What you get

- ✅ **Full or partial export**, while the bus keeps running.
- ✅ **Encrypted to your key**, readable by nobody else.
- ✅ **A plain `.info` file** next to every archive, with no secret values in it.
- ✅ **Dry-run import** that checks the whole archive and lists every conflict.
- ✅ **All-or-nothing import:** the whole archive lands, or nothing does.
- ✅ **Move between hosts:** names can move to another realm on import.
- ⚠️ **Only what survives a restart:** never queued messages, held locks or login sessions.
- 🚫 **Owner only:** nobody else can export or import.

## Which backup

| You want | Use | Takes |
|---|---|---|
| everything, exactly as it is now, on this host | [stopped-daemon backup](../07-setup.md#stopped-daemon-backup) | a short stop; as root |
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
<summary>See it on a real node (ran today)</summary>

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
- One tag: `agent-bus-admin export --include=tag:messaging`
- A tag minus one name: `agent-bus-admin export --include=tag:ops --exclude=name:'#old@myhost'`
- To a file of your own: `agent-bus-admin export --include=tag:ops --out=ops.tar.gz.age`
- `--exclude` wins over `--include`.
- Write `name:` or `tag:` when a word could be either; a word that matches both, or nothing, is refused.
- 📁 **Where it lands:** `/var/lib/agent-bus/record-backups/`, as `full-<host>-<time>.tar.gz.age` or `partial-<host>-<time>.tar.gz.age`.

## The `.info` file

- 🏷️ **Every archive has one** beside it, open, so a file in central storage explains itself without the key.
- It says:
  - whether the backup is full or partial;
  - the host and version it came from;
  - when it was made, and by whom;
  - the selection rules and the record names they matched;
  - the key it is encrypted to;
  - the archive's SHA-256.
- 🔒 **It never holds** secret or token values.

### Moving to central storage

- The backup folder belongs to the bus's own account, so copy with `sudo`; the wildcard needs root too:

```sh
sudo sh -c 'cp /var/lib/agent-bus/record-backups/full-* /mnt/backups/'
```

- 🧹 **A weekly cleanup** keeps the newest five full and ten partial archives there ([the bounds](../07-setup.md#development-node-cleanup)).

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
- Replace one tag, keep the rest: `agent-bus-admin import ops.tar.gz.age --replace=tag:ops --keep-existing=all`

## Rebuilding a bus from an export

1. Take a [stopped-daemon backup](../07-setup.md#stopped-daemon-backup), always.
2. `agent-bus-admin export` the records to keep.
3. Start a fresh bus with you as its Owner, and enroll your key on it; SSH enrollment and local account mappings are not in the archive.
4. `agent-bus-admin import <archive> --dry-run`, then the import with the choices it asks for.
5. Agents reconnect with their old tokens. Check with `src/check-deploy.sh` and `agent-bus ls --all --table`.

- 🧩 **Runner agents** keep their host files; their bus-side Configuration and Secret come back from the archive, so export their records too.

## Technical details

<details>
<summary>What an archive holds, and the rest of the options</summary>

- **Holds:** records, access lists, Maintainers, roles, tags, group membership, Configuration, Secret, stored key-values, and current and previous persistent credentials.
- **Leaves out:** messages, locks, login sessions, readers and the bus's own `#bus` record.
- **Encryption:** `age`, to your enrolled Ed25519 SSH public key.
- **Import key:** `--key=<file>`, default `$HOME/.ssh/id_ed25519`.
- **To stdout:** `--out=-`, with `--info=<path>` for the `.info` file.
- **`.info` on import:** a missing one is allowed; one that does not match the archive is refused.
- **Separate folders:** record exports live apart from root's stopped-daemon backups in `/var/lib/agent-bus/daemon.backup/`.
- **More:** [encrypted record backups](../07-setup.md#encrypted-record-backups).

</details>
