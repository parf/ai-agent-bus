# Upgrade and recovery

📌 **TL;DR:** 0.5.69 upgrades a populated packaged node through one verified
release switch and restores the prior release plus its whole matching state on
failure or interruption.

## Result

`agent-bus-setup --upgrade` is separate from first installation. Ordinary setup
still repairs the same release and now refuses to select a different one.
Upgrade accepts no owner, address, user, key or executable overrides: installed
authority, mappings and operator configuration stay authoritative.

The new package and current installed release are verified before downtime.
The new release is staged unselected, free space is checked, and a durable
root-owned recovery journal is written before stopping the daemon. After a
clean stop, the entire daemon home is copied as one backup with ownership and
modes retained. One atomic `current` rename selects the new release. Success
requires its public identity plus the real systemd cgroup to show the supervisor,
bus and sandbox-bound web process from that release, and requires the unit file
to retain its original digest.

A failed start stops the new unit, restores both the old `current` target and
the backed-up daemon tree, starts the old release and verifies its identity and
processes. If setup or the host stops mid-transaction, the same new archive's
`--recover` command performs that rollback. Recovery refuses a unit changed
after the journal was written rather than combining unknown configuration with
the saved state.

Setup also waits for the daemon account's credential socket before first-user
provisioning. This removes the active-unit/socket-readiness race without using
the anonymous shared listener.

## Checks

A network-isolated disposable host with real systemd 260.1 upgrades a populated
0.5.68 archive to 0.5.69 without a checkout or `/rd`. The final measured 6/6 run:

- preserves the token store and SSH authorization bytes, a queued envelope,
  registry and ACL behavior, durable `root` and `nobody` account mappings, and
  the base unit plus an operator drop-in;
- removes `agent-bus-web` from the new package and proves validation fails
  before the old daemon stops or `current` changes;
- kills setup after the new release is selected, observes the durable journal,
  and restores 0.5.68 with the documented `--recover` command;
- installs a manifest-valid new daemon that corrupts both the token and snapshot
  before failing, then proves automatic rollback restores matching credentials
  and registry state rather than either half alone; and
- completes the good upgrade, consumes the pre-upgrade message, exercises the
  pre-upgrade peer ACL through its mapped socket, verifies all six installed
  program versions, and finds only the intended supervisor, bus and web
  executable/bind paths in the live cgroup.

Nine focused unit mutations and four full real-systemd package mutations fail
independently. They remove the different-release guard, old-release validation,
safe `current` target check, private journal mode, whole-tree restore, delegated
cgroup traversal, credential-socket type check, explicit unit address and state
symlink copy; or remove the journal, operator-configuration check, state half of
rollback and systemd failed-limit reset. The fast repository smoke is 488/0.
The frozen slow repository smoke is 609/0; all 18 candidate hashes and both
acceptance-script hashes remained unchanged through it.

The first exploratory container assumed `ssh-keygen`; the fixture now uses
fixed public-key lines. The second exposed `sudo` as a missing prerequisite in
the disposable image. The third corrected a socket fixture from uid spelling
to the documented account spelling. The fourth found that process inspection
must descend below `DelegateSubgroup=supervisor`. The fifth replaced a byte
comparison of the live snapshot — whose clean/start metadata legitimately
changes — with semantic registry, queue and corruption checks. A sixth replaced
an unavailable `cmp` utility with a shell comparison. A seventh exposed a real
rollback bug: repeated failed starts exhausted systemd's burst limit, so rollback
now resets that unit's failed limiter before starting the verified release.
None receives acceptance credit. A separate 9/9 fresh-install rerun proves the
new first-user socket-readiness control on the same real-systemd host.

## Limits

The backup is local recovery data, not the future runner backup mechanism or an
off-host disaster-recovery system. Installed unit migration is intentionally
outside this slice: 0.5.69 preserves the base unit and drop-ins exactly. A later
release that needs a unit schema change must define and accept that migration
rather than silently overwriting operator configuration.
