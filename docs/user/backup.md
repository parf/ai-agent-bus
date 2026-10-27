# 💾 Backups: export, move, restore

📌 **TL;DR:** `agent-bus-admin export` saves records as one encrypted archive,
either all of them or the ones you pick. Only your SSH key can open it. Copy it
anywhere: the `.info` file beside it says what is inside. `agent-bus-admin
import --dry-run` shows every conflict before anything changes. Only the daemon
Owner can export or import.

## 🧭 Which backup

| You want | Use | Takes |
|---|---|---|
| everything, exactly as it is now, on this host | [stopped-daemon backup](../07-setup.md#stopped-daemon-backup) | a short stop; as root |
| every record with its settings, secrets, KV and tokens, while the bus runs | `agent-bus-admin export` (full) | nothing; as yourself |
| some records — a tag, a few names — to move or keep | `agent-bus-admin export --include=…` (partial) | nothing; as yourself |

A record export keeps only what survives a restart: never queued messages, held
locks or login sessions. [Details](../07-setup.md#encrypted-record-backups).

## 📤 Export

Run it as yourself on the daemon's host, without `sudo`. You need `age`
installed and an enrolled Ed25519 key.

```sh
agent-bus-admin export                                    # full backup
agent-bus-admin export --include=tag:messaging            # one tag
agent-bus-admin export --include=tag:ops --exclude=name:'#old@myhost'
agent-bus-admin export --include=tag:ops --out=ops.tar.gz.age   # to a file of your own
```

- With no selector, the backup is full, users and groups included. Any `--include` or `--exclude` makes it partial.
- `--exclude` wins over `--include`.
- Write `name:` or `tag:` when a word could be either. A word that matches both,
  or nothing, is refused.
- By default the archive lands in `/var/lib/agent-bus/record-backups/`. Full
  and partial archives get different names:
  `full-<host>-<time>.tar.gz.age` and `partial-<host>-<time>.tar.gz.age`.
  The command prints both paths.

## 🏷️ The `.info` file

Every archive has an open `<archive>.info` beside it, so a file in central
storage explains itself without the key. It holds:
- whether the backup is full or partial;
- the host and version it came from;
- when it was made, and by whom;
- the selection rules and the record names they matched;
- the key it is encrypted to;
- the archive's SHA-256.

It never holds Secret or token values.

Moving to central storage: the directory belongs to the daemon account, so copy
with `sudo`:

```sh
sudo sh -c 'cp /var/lib/agent-bus/record-backups/full-* /mnt/backups/'   # the glob needs root too
```

A weekly cleanup keeps the newest five full and ten partial archives there;
[the bounds](../07-setup.md#development-node-cleanup).

## 📥 Import

Always look first. `--dry-run` decrypts with your key, checks the whole archive
and lists every conflict, changing nothing:

```sh
agent-bus-admin import full-<host>-<time>.tar.gz.age --dry-run
```

The import then refuses to start until every conflict has an explicit answer:

| Option | For names already on this daemon |
|---|---|
| `--keep-existing=<sel>` | keep what is here; take the rest from the archive |
| `--replace=<sel>` | replace the record from the archive |
| `--replace-users=<sel>` | replace the user from the archive |
| `--replace-tokens=<sel>` | also replace their tokens; without it, tokens here stay |
| `--rename-realm=<from>=<to>` | move names to another realm; names never change on their own |

`<sel>` is `name:…`, `tag:…`, a list, or `all`. Two choices that contradict
each other on one record are refused. The import is one transaction: all of it
lands, or nothing does.

```sh
agent-bus-admin import ops.tar.gz.age --keep-existing=all
agent-bus-admin import ops.tar.gz.age --replace=tag:ops --keep-existing=all
```

## 🔁 Rebuilding a daemon from an export

1. Take a [stopped-daemon backup](../07-setup.md#stopped-daemon-backup), always.
2. `agent-bus-admin export` of the records to keep.
3. Start a fresh daemon with you as its Owner, and enroll your key on it. SSH
   enrollment and local account mappings are not in the archive.
4. `agent-bus-admin import <archive> --dry-run`, then the import with the
   choices it asks for.
5. Agents reconnect with their old tokens. Check with `src/check-deploy.sh` and
   `agent-bus ls --all --table`.

Runner agents keep their host files; their bus-side Configuration and Secret
come back from the archive, so export their records too.
