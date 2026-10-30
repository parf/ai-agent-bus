# Runner backups

## Scope

Pending R1.1 work, moved from R1 by the owner on 2026-10-30.
The task IDs and acceptance remain in [TODO](TODO.md#backups). Encrypted
record export and import stay in [R1](../R1.0-Release/TODO.md#objective).

## Backing it up

**The backup is `runner/`, encrypted, and that is the whole of it.** It is a
runner verb (owner decision, [№Q33](../R1.0-Release/DECISIONS.md#recorded-decisions)): `agent-bus call '#runner/<instance>@<host>' backup`
answers with the archive, under the runner's ACL, and any timer may call it.
`agent.d` is a checkout: whatever is in it can be fetched again from where it
came from, and `agents.json` says from where. `runner/` cannot be fetched
again — it is every decision this host made, plus the record of what is
installed, and nothing else holds a copy.

**Use the `age` command-line tool**, encrypting the archive to the user's
Ed25519 SSH public key (`ssh-ed25519`). Backup creation needs only that public
key. Restore uses the matching private SSH key on the user's restore host;
neither the daemon nor the runner stores the user's private key.
[age's SSH key support](https://github.com/FiloSottile/age#ssh-keys) provides this
without a custom encryption format or a new key type of ours.

The archive contains secrets, so encryption is required. The user's key keeps
backup recovery outside the daemon's authority and preserves the
[separate secret domains](../../docs/07-setup.md#the-two-accounts).

Restoring is the archive backwards: fetch each origin at its commit, unpack
`runner/` over it. An agent whose origin is gone is a **named failure** and
not a quieter restore — a host that comes back with four agents out of five
and says nothing is worse than one that will not come back.

