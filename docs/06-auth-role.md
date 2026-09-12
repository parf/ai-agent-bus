# AUTH role

Optional child process of `agent-busd` (`auth: on`). Turn it on when an
organisation wants central identities, groups and hourly derived keys; leave it
off and everything in [access](02-access.md) still works.

Nodes with `auth: on` are the AUTH replicas — run **2+**. A laptop node never
holds `master_secret`.

## Bundle

```
bundle { gen, prev_gen, created_at, payload, payload_hash }
signature = Ed25519(offline_signing_key, gen | prev_gen | created_at | payload_hash)
```

**Payload**: principals, pubkeys, groups, ACL/role expressions, **admin keys**
(the SSH pubkeys allowed to administer; `authorized_keys` is regenerated from
them each generation), identity-source config, optional `next_signing_pubkey`
for rotation. Nothing in it is secret — the pubkeys came from a public
directory.

**Not in the payload**: service and topic definitions, ownership (those are
live records, [services § service and template](03-services-and-topics.md#service-and-template)),
service health/stats, queues, sealed private configs, `master_secret`.

Enforced by replicas **and** services: valid signature; `gen > current`
(identical gen + hash = no-op). Gaps (`prev_gen != current`): **newer
generation wins**, gap logged.

## Where it runs

- The AUTH role is a **child process of `agent-busd`**, with no network
  listener of its own. Only that child holds `master_secret` and
  verifies/serves the bundle; the bus reaches it over a unix socket
  (sshd/Postfix-style privilege separation) —
  [processes](11-processes.md).
- **Signing key is offline** (admin machine): `agent-bus auth sign --gen N`.
  Never on a server → any replica can be master, failover is a pointer flip, a
  compromised replica can serve stale-but-valid config — but see
  [overview § trade offs](00-overview.md#trade-offs): it does hold `master_secret`.
- `master_secret`: **out-of-band file** on each AUTH replica, never in the
  bundle or the repo.

## Topology

- **The bundle lives in a git repo over SSH.** Workflow: edit →
  `agent-bus auth sign --gen N` → `git push`. Replicas **pull on start** and on
  poll (30–60 s); master pushes. Git history is the audit trail. Optional fast
  path: `GET /bundle?since=<gen>` from the master, 304 if unchanged.
- **Default: master/slave.** Reads (key issuance, lookups) from any replica;
  writes only via master; master down → reads continue; promotion is a manual
  flag flip.
- Every response carries `gen`; services refetch when they see a newer one.

## Reference deployment

A local `agent-busd` with `auth: on` as master + a **git remote as backup**: a
GitHub repo (private suggested, not required) or the user's own SSH account on
another server. The same repo also receives unsigned **registry snapshots** in a
separate directory — which is also how peers sync
([services § registry sync](03-services-and-topics.md#registry-sync)). Backup
and peer sync, never authority: the remote cannot forge (no signing key) and
holds no `master_secret`. Lose the box → clone, drop in the `master_secret`
file, start. Works offline; the remote is the off-site copy, not a dependency.

## Consistency window

Revoked access is honoured up to poll interval + one epoch. Optional
`revoked_users` list in the bundle, checked on every session start, gives
immediate effect on *new* sessions; live sessions are not torn down.

## SSH admin

`agent-busd` runs as the dedicated `agent-busd` user: nologin shell, no sudo,
home 0700. Admins SSH in with their own keys; identity is bound to the key; the
forced command is **`agent-bus-admin`**, the same program an operator runs on
the console ([setup § the programs](09-setup.md#the-programs)), so
there is one grammar and one set of rules rather than two.

- `authorized_keys` holds **every** user's key, each behind the forced command
  its holder is entitled to: `restrict,command="/…/agent-bus-admin <admin-name>"`
  for an operator, `restrict,command="/…/agent-bus-token"` for everybody else
  (optionally `from=`). Regenerated from the bundle each generation. The
  `token` verb is the same either way
  ([setup § the programs](09-setup.md#the-programs)) — an operator's
  line adds verbs, it does not change that one.
- `sshd_config`: `Match User agent-busd` → `ForceCommand`, `PermitTTY no`,
  `AllowTcpForwarding no`, `AllowAgentForwarding no`, `X11Forwarding no`,
  `PermitUserEnvironment no`, `PasswordAuthentication no`;
  `ExposeAuthInfo yes` to log the key fingerprint.
- `agent-bus-admin` parses `$SSH_ORIGINAL_COMMAND` against a fixed verb
  grammar (`bundle show|history`, `user list`, `service list`, `status`,
  `replica-sync`, `token`); a bundle arriving on stdin is still verified
  (signature + gen) — SSH gates *who may talk*, the signature gates *what
  config is real*; append-only audit log `ts admin fp verb gen result`.
- **Break-glass is root on the box** — it can always edit `authorized_keys` or
  the bundle pointer by hand. No separate offline admin key.
- Master→slave sync may itself run over SSH with a `replica-sync` forced command.
- Test the lockdown: `ssh agent-busd@host bash`, `-L`, `-A`, `-t` must all fail.

❓ **`authorized_keys` is regenerated from the bundle each generation**, which
would drop the key `agent-bus-setup` installed for issuing tokens
([access § getting a token](02-access.md#getting-a-token)) the moment AUTH is
switched on. *Settled by:* owner.
