# Administer a bus node

📌 **TL;DR:** Manage accounts, TLS, storage and backups after installation. For initial setup, use [the install guide](07-setup.md#install).

### The two accounts

The installer creates two system accounts, the daemon's and the web face's,
each running its own program under its own unit. A person's runner gets an
account of its own when it is installed, never by default
([a person's runner](runner/setup.md#a-persons-runner)).

| Account | Home | Shell | Runs |
|---|---|---|---|
| `agent-busd` | `/var/lib/agent-bus/daemon` | `/bin/sh`, for forced commands | the daemon |
| `agent-bus-web` | the [web code directory](08-processes.md#the-web-face), read only | `nologin` | the [web face](08-processes.md#the-web-face); holds no privileged bus credential |

| Directory under `/var/lib/agent-bus` | Owner | Mode | Current purpose |
|---|---|---|---|
| `daemon/` | `agent-busd` | 700 | Daemon home and its database |
| `agent.d/` | root | 755 | Shared public agent code, readable by every runner account |
| `<person>-ab-runner/` | that runner account | 700 | Its inventory, overrides, env files, tokens and work directories |

- The daemon account uses `/bin/sh` because sshd runs even forced commands through the account shell; each issued key is restricted to its named helper, without PTY or forwarding.
- A runner's account has `nologin` and receives no SSH keys.
- Runtime sockets live at the [access location](02-access.md#local-socket), not under any home.

#### The two units

- `/etc/systemd/system/agent-busd.service` for the daemon and `/etc/systemd/system/agent-bus-web.service` for the web face, whose settings [processes § the web face](08-processes.md#the-web-face) owns.

| Daemon unit setting | Purpose |
|---|---|
| `User=agent-busd` | Run as the service account |
| Working/state directories and explicit state mode | Keep state under its own home |
| `RuntimeDirectory=agent-bus` | Recreate the runtime socket directory |
| `Restart=on-failure` | Restart a failed daemon |
| `AmbientCapabilities=CAP_CHOWN`, bounded to that capability | Own the per-user sockets without running the daemon as root |


### TLS

- Setup asks, at a terminal, whether to enable SSL on the daemon's port, but only when the node has neither TLS nor an earlier `--tls off`, and never with `--upgrade`, `--recover`, `--dry-run` or `--print-unit`.
- Yes means a certificate it generates or one you provide, chain included.
- Otherwise the flags decide, and without flags a node keeps what it has.
- An unreadable TLS directory fails with its permission error; setup never infers TLS-off from it or widens its modes.
- Writing TLS files restarts a running daemon even when its unit and release are unchanged.
- The port answers TLS beside plain HTTP ([remote access § over HTTPS](02-access-remote.md#over-https)).

| Flag | |
|---|---|
| `--tls self-signed` | generate an ECDSA P-256 certificate, valid ten years, for `localhost`, `127.0.0.1`, `::1`, the host's names and addresses, and each `--tls-name`; run again, it generates a new one, so clients fetch the fingerprint again |
| `--tls files --tls-cert c --tls-key k [--tls-chain ch]` | check that the key is the certificate's, that the chain parses and that the certificate has not expired, then copy them |
| `--tls off` | plain HTTP only, recorded as a `disabled` file in the directory, so later runs keep it so until `--tls` turns it on again; the files stay where they are |
| no `--tls` | keep what the node has: a re-run and `--upgrade` never drop TLS; `--tls` itself is refused with `--upgrade` or `--recover` |

<details>
<summary>Files, modes and what setup prints</summary>

- **Where:** `/etc/agent-bus/tls/`, `root:agent-busd 0755`, so any user can read the public certificate. Setup and upgrade preserve the certificate when repairing directory modes.
- **Files:** `cert.pem` and `chain.pem` `0644`; `key.pem` `root:agent-busd 0640`. Each is written aside and renamed into place. A choice without a chain removes an old `chain.pem`, which would otherwise be appended to the new certificate.
- **Unit:** it carries `--tls-dir /etc/agent-bus/tls` exactly while TLS is on. A present directory that does not load stops setup before the unit is written.
- **Output:** setup ends with the fingerprint and the client settings, `AGENT_BUS_ADDR=https://<host>:<port>` and `AGENT_BUS_TLS_FINGERPRINT=sha256:…`.
- **The web face:** it gets its own copy in `/etc/agent-bus/web-tls/`, `root:agent-bus-web 0755`, with the key `0640`, so neither account reads the other's key.
  - A drop-in, `agent-bus-web.service.d/tls.conf`, names that copy (`AGENT_BUS_WEB_TLS_DIR`).
  - Its port then serves TLS and redirects plain HTTP there to `https://`.
  - Setup waits for its health over `https://`, pinned to the copy.
- Setup starts the face once at the end, when its unit, TLS files, runtime or code bytes changed, or when it is not running. The applied record is written after health answers; a release switch counts as a change. Identical runs leave a healthy face alone.
  - `--tls off` removes the drop-in, and a re-run or `--upgrade` keeps what the node has.

</details>

#### Trust the web certificate in your browser

A self-signed web certificate must also be trusted by the browser. Without
trust, the browser shows **Not secure** and will not offer to save the token
as a password. Trusting it for the CLI does not change browser trust.

First, on the node (or over a trusted SSH connection), check the public web
certificate's SHA-256 fingerprint:

```sh
openssl x509 -in /etc/agent-bus/web-tls/cert.pem -noout -fingerprint -sha256
```

Compare it with the SHA-256 fingerprint in the browser's certificate viewer
before trusting or importing it. Use the public `cert.pem`, never `key.pem`.
If the browser is on another computer, copy only that public certificate over
SSH and check the same fingerprint there.

- **Chrome or Chromium on Linux:** with `certutil` installed, run this as the browser's user, without `sudo`:

  ```sh
  certutil -d "sql:$HOME/.pki/nssdb" -A -t "P,," -n agent-busd-web -i /etc/agent-bus/web-tls/cert.pem
  ```

- Fully exit Chrome/Chromium, including background processes, then reopen it.
- **Firefox:** import the public certificate under **Settings → Certificates → Servers**, or open `about:certificate`.
- Confirm that its fingerprint matches before keeping the certificate exception for the web address.
- **macOS:** import the public certificate in **Keychain Access**, open its **Trust** settings and choose **Always Trust** after checking the fingerprint.

Reopen the HTTPS web page and check that the certificate warning is gone.
If setup replaces the certificate, verify its new fingerprint and update the
browser's trust entry before signing in again.

### Local users

Setup asks for two things per user, and they are **configuration, not
credentials**:

| Config field | Purpose |
|---|---|
| local account | the OS account on this host; resolved to its uid |
| bus username | the principal it *is* — `alice`, `alice@team.example` ([identity § names](01-identity-and-authority.md#names)) |

From that the daemon opens one socket per user
([access § local socket](02-access.md#local-socket)) and knows who is calling
on every request.

- Each mapped local account uses its own socket; the bus applies record ACLs to that mapped identity.
- Setup's initial principal is the first daemon Owner; after transfer, the stored current Owner holds node-wide management authority ([identity § daemon owner](01-identity-and-authority.md#daemon-owner)).

### SSH admin

- Setup adds the installer's public key through the admin program.
- Each entry in the daemon account's `authorized_keys` is restricted to a forced command: the fixed `agent-bus-token <principal>` wrapper for an ordinary key, or `agent-bus-admin <principal>` for a bus operator.
- The wrapper executes only the sibling CLI’s `token` verb, never a verb from the SSH request.
- Existing forced-command paths remain valid.
- The entitlement is the principal in that entry; `SSH_ORIGINAL_COMMAND` is parsed as a request, not executed as shell text.
- The forced command names the program as it was invoked (`/usr/local/bin/…`), which follows the live release, never the release directory a later deploy prunes.
- For host administration, `agent-bus-admin` becomes the daemon account through sudo, which drops `AGENT_BUS_ADDR`, so with an `AGENT_BUS_ADDR` naming another daemon and no `AGENT_BUS_HOME` it refuses rather than edit the installed one.

- **`user add` creates the user it adds.** A credential is only issued to a name the daemon already holds a profile or a record for ([getting a token](02-access.md#getting-a-token)), so writing the `authorized_keys` line alone would leave a key whose forced command is refused — which is no way in at all.
- The verb writes the line and creates the user, and `--admin` additionally grants [Administrator membership](01-identity-and-authority.md#groups).
- Authority is granted only where it was asked for: Administrator standing is membership, derived by the daemon, never a field a caller may claim.

- The two halves are kept together.
- The key line is written first because it is the half that can be taken back — a user is never deleted ([user lifecycle](01-identity-and-authority.md#user-states)) — and it is removed again if the daemon refuses.
- **The daemon has to be running**: with no way to create the name, `user add` refuses rather than leaving a key that works before the name exists.
- Start the daemon and run it again.

- **From the web face.** Whoever may edit a User — the daemon Owner, or an Administrator for an ordinary User — may paste that User's `ssh-ed25519` public key on Add user or the User's page.
- The daemon writes the same line `user add` writes, forced to the credential wrapper, under a file lock both writers take; setup's unit passes `--ssh-keys` (the account's `authorized_keys`) and `--ssh-token` (the program beside the installed daemon), and without them the call answers that keys are added on the host.
- Only the key itself reaches the file — the daemon parses the one line and rebuilds it, so no option or comment is copied.
- A second key replaces the first; other key types and operator keys stay host-side, and a bus operator's line is never replaced from the web.

- `user import-local <user[@realm]> <account>` fills a blank person name from the OS account database.
- It accepts an account name rather than profile text and preserves an existing name.
- Setup runs it when it installs the first user's key; a bus operator may run it later when setup found no key.

- `user add-local <user[@realm]> <account>` creates a User with no key and maps that local account's socket to it, so the socket is its only way in; the account must exist on the host, and a User or mapping already in place is not a failure.
- A person's runner uses its own token ([runner setup](runner/setup.md#a-persons-runner)).

- The implemented admin verbs are `user add`, `user add-local`, `user import-local`, `user list`, `user remove`, `account list`, `account set`, `account remove`, `token` and `tls`, plus Owner-only local `export` and `import`.
- A forced command answers `token --fingerprint` with no name or credential.
- The token operation is delegated to the token helper with the key entitlement and original request kept separate: `token --rotate` rotates that identity; asking for another identity is refused.
- Console and SSH use the same program.
- Bundle administration and regeneration of keys are not part of this grammar.

### Administering the account map

- The durable local-account map is administered by the daemon Owner or an Administrator with credentials that already exist: an **operator SSH key**, whose forced command is `agent-bus-admin <principal>` ([SSH admin](administration.md#ssh-admin)), or that principal's **user token** against the API ([getting a token](02-access.md#getting-a-token)).

Nothing new is minted for it and no separate administrative password exists.
The entitlement is the principal in the key's entry, exactly as it is for every
other operator action, so who may edit the map is answered by the same thing
that answers who may do anything else.

| Operation | Command grammar |
|---|---|
| Inspect mappings | `agent-bus-admin account list` |
| Add or change a mapping | `agent-bus-admin account set <local-account> <user[@realm]>` |
| Remove a mapping | `agent-bus-admin account remove <local-account>` |
| Apply listener changes | A full `agent-busd` restart; host operator action |

- The mapped principal must already be known and active, and the local OS account must exist.
- The daemon Owner may map any principal.
- An Administrator may map only their own User identity or an Agent they own; they may not map another User.
- The daemon account's own socket is implicit and cannot be reassigned or removed.
- A successful edit is in the database before it is acknowledged, but listeners belong to the supervisor: the command reports `restart required` until a full daemon restart applies the desired map.
- A bus-child restart alone keeps the existing listeners.

- The first start seeds this map from setup's `--user` flags.
- Once the database carries the establishment marker, the stored map is authoritative; changing or retaining an old flag cannot restore a removed mapping.
- Startup rejects malformed mappings, duplicate accounts, missing OS accounts and any attempt to put the implicit daemon account into the editable map.
- Retired socket paths are removed when the new supervisor starts.


### Storage

| Store | Holds |
|---|---|
| SQLite database `agent-bus.db`, mode 0600, held exclusively | Daemon Owner, local-account map, users, registry, groups, credentials and issued times, queue contents, counters, each record's [key-value store](03-records-kv.md#key-value-store), the index of each record's [bigfiles](03-records-kv.md#bigfiles), its [activity history](05-discovery.md#activity-history) by day, [agent-log](agents/bus.md#agent-logs) alert rings, subscription definitions and counters, id reservations, and the clean-stop marker; only the current schema is accepted |
| Files in `bigfile/` beside the database, 0600 in a 0700 directory | The bytes of each record's [bigfiles](03-records-kv.md#bigfiles), whole or absent and expiring. Their index is a table in the database, so a database backup holds the rows and not the files, and start drops each restored row whose file is not present. |
| Memory only | Agent-log notices and warnings, browser sessions, outstanding readers, [locks](01-identity-and-authority.md#shared-locks), uptime and recent envelope feed |

- The daemon is told where the database is with `--db`; setup runs `agent-busd --init` once, as the daemon account, to create it, and the unit never passes `--create`.
- An empty file a create left behind is finished by the next create, and refused as empty without one; a file holding anything else is never taken for one.
- Durability is defined in [messaging § durability](04-messaging.md#durability).
- SQLite state and backups retain the daemon account's private directory and file boundary.

### Backup and restore

#### Encrypted record backups

- The daemon Owner exports permanent record content online, encrypted by `age` to their enrolled Ed25519 SSH public keys.
- Install `age` on the daemon and restore host.
- Run as your own login user, without sudo: export and import use your local login socket, never an administrative account handover.

| Operation | Command grammar |
|---|---|
| Full record export | `agent-bus-admin export` |
| Selected export | `agent-bus-admin export --include=<selector> --exclude=<selector> --out=<archive>` |
| Preview import | `agent-bus-admin import <archive> --dry-run --keep-existing=all` |
| Import new names, keep existing ones | `agent-bus-admin import <archive> --keep-existing=all` |

- Setup and upgrade create `/var/lib/agent-bus/record-backups/`, owned by `agent-busd`, mode 0700.
- A managed `record-backups.conf` unit drop-in grants the daemon's strict mount namespace write access to that directory.
- Default filenames are `<full|partial>-<host>-<UTC>.tar.gz.age`; the command prints the archive and adjacent `.info` paths.
- This directory is separate from root's stopped-daemon backups in `/var/lib/agent-bus/daemon.backup/`.

<details>
<summary>Selection, sidecars, conflict choices and realm mapping</summary>

- No selectors means full. `--include` and `--exclude` accept names or tags, repeatable or comma-separated; exclusion wins. Users and groups are included by default. Write `name:<name>` or `tag:<tag>` explicitly when needed: a bare selector matching both, or matching nothing, is refused. `all` is reserved for import conflict options.
- Archives hold records, ACLs, Maintainers, roles, tags, group membership, Configuration, Secret, typed KV and current/previous persistent credentials. They omit messages, locks, transient sessions, readers and the daemon's reserved `#bus` record. They do not transfer SSH enrollment or local account mappings; enroll the restoring Owner on the destination first.
- Every file export has an open `.info` with kind, source host, version, time, caller, selection rules and resolved names, recipient fingerprints and the ciphertext SHA-256. It contains no Secret or token values. The encrypted manifest holds the same immutable info; its ciphertext hash belongs only outside the ciphertext.
- `--out=<path>` writes on the caller's machine with an adjacent sidecar. `--out=- --info=<path>` streams only ciphertext to stdout and writes the sidecar separately. Without `--info`, streaming writes no sidecar and says so on stderr. `--info` is invalid without `--out=-`.
- Import decrypts locally with `--key=<file>` (default `$HOME/.ssh/id_ed25519`). A missing sidecar is allowed and reported as "no sidecar"; a present mismatched hash or info is refused. Wrong keys fail before contacting the daemon.
- Every existing record or user needs `--keep-existing`, `--replace` (records) or `--replace-users`. These and `--replace-tokens` accept the same selectors or `all`. Conflicting choices fail. Existing credential bytes stay unless `--replace-tokens` names their principal, and the report names every principal whose archived token was not retained. New principals retain their archived credentials, rebound to the destination's internal IDs.
- Import validates the whole archive and reports every name conflict before applying one transaction. `--dry-run` makes no changes. Keep-existing applies only to names already present, so `--keep-existing=all` still imports new records. An imported private agent may need its ACL dependencies present.
- Names keep their realm exactly. Repeat `--rename-realm=<from>=<to>` for an explicit mapping, or use `--rename-realm==<to>` for realm-less identities. Only typed names and ACL/group references change; Configuration, Secret and KV content stays opaque. Each identity mapping is reported.

</details>

#### Stopped-daemon backup

- **A stopped-daemon backup is the whole daemon home.** The daemon holds its database exclusively, so no online copy exists: `sqlite3 .backup` and every other reader get `database is locked`.
- A graceful stop flushes queues and counters and checkpoints the WAL into `agent-bus.db`, so the stopped home is complete and consistent.
- Use an unused archive name in a root-only directory. Create the archive with a private umask before writing any bytes; it contains credentials and secrets.

| Step | As root |
|---|---|
| Back up | `systemctl stop agent-busd`, then `(umask 077; tar -C /var/lib/agent-bus -cpf <backup>.tar daemon)`, then `systemctl start agent-busd` |
| Restore | Install the same release (on a new host, `agent-bus-setup` as in [install](07-setup.md#install)), `systemctl stop agent-busd`, `rm -rf /var/lib/agent-bus/daemon`, `tar -C /var/lib/agent-bus -xpf <backup>.tar`, `systemctl start agent-busd` |

<details>
<summary>What it covers and what to keep in mind</summary>

- The home holds the database (users, records, groups, configurations, secrets, credentials, ownership, queue contents and counters) and `.ssh/authorized_keys`; together they are the node. The archive holds live credentials and secrets: keep it root-only.
- Restore **replaces the whole home**. Copying only `agent-bus.db` over a home that still has an `agent-bus.db-wal` from the state being replaced — left by any unclean stop — lets SQLite replay that newer WAL onto the older database.
- A restore is exactly the backed-up state: later users, keys, records, rotations and traffic are gone, and credentials issued after the backup stop authenticating.
- The local-account map is restored with the database; a new host needs the same OS accounts for their sockets. Logs, the unit and the runner's `agent.d` are not part of the backup.
- A backup taken without the stop is not supported: the database file lacks what is still in the WAL and in memory.

</details>

### Logs

The daemon writes the [three logs](constitution.md#logs)
under `--log-dir`, which setup makes `/var/log/agent-bus/`: owned by
`agent-busd`, group `adm`, mode `2750`, so every log the daemon creates there is
readable by `adm` and by nobody else. The files are `0640`.

| File | Written |
|---|---|
| `audit.log` | always: one line per administrative action or entity edit — actor, operation, target, result and, over TCP, the client address |
| `error.log` | always: warnings, errors and alerts; managed units also enable the [syslog copy](user/daemon.md#environment-defaults) |
| `debug.log` | only when asked for, by `--debug-log` at start or by the daemon Owner with `POST /debug {"on": true}` at run time |

- Without `--log-dir`, a development daemon writes to `logs/` beside its database.
- Setup installs `/etc/logrotate.d/agent-bus`: daily, three archives, and a log over 10 MiB is rotated at the next daily run, compressed by copy and truncate so the daemon never reopens a file. It is the one rule for these logs; repeating setup writes the same file again.

### Config locations

- Current daemon configuration comes from command flags and environment; setup writes the flags into its unit.
- The per-program defaults are in [daemon source](../src/cmd/agent-busd/main.go).
- A general configuration-file format is not implemented.
- Per-record ACL editing is available through the [web face](../src/web/web-face/records.md#settings).

### Sample data

- `sudo /usr/local/bin/agent-bus-setup --samples` fills a running node with something to look at, from a galaxy far, far away and from Spaceballs, each name in its own realm; `sudo /usr/local/bin/agent-bus-setup --remove-samples` takes exactly that away again.
- Both write as the daemon Owner on the daemon account's socket; with `AGENT_BUS_ADDR` set to a development daemon's owner socket they need no root.

| Kind | Samples |
|---|---|
| Users | `luke@tatooine`, `leia@alderaan`, `han@corellia`, `yoda@dagobah`, `vader@empire`, `lone-starr@druidia`, `vespa@druidia`, `dark-helmet@spaceball`, `yogurt@vega` |
| Agents | `#r2d2@naboo` (Maintainer, configuration), `#c3po@tatooine`, `#k2so@scarif`, `#dot-matrix@druidia`, `#barf@druidia` |
| Services | `death-star@empire` (Maintainers, secret, configuration), `death-star-ii@endor` (inactive), `jedi-archives@coruscant`, `spaceball-one@spaceball`, `schwartz@vega` (secret) |
| Queues | `rebel-alerts@hoth`, `jabba-debts@tatooine`, `air-supply@druidia`, `falcon-repairs@corellia`, `droid-inbox@tatooine` (Deliver-To route to `#r2d2@naboo`) |
| PubSub | `imperial-news@empire`, `spaceballs-merch@spaceball`, `jedi-journal@dagobah` |
| Groups | `@rebels`, `@jedi`, `@empire`, `@spaceballs` |

| Rule | |
|---|---|
| Idempotent | a second `--samples` changes nothing and sends no message twice |
| Real data | a name that exists without its sample description — or a user with another person name — is left alone, on adding and on removing, and named |
| Removal | sample records are drained and unregistered; sample groups are emptied, which is how a group retires; sample users are deactivated, since the daemon never removes a user. `--samples` again brings them back |
