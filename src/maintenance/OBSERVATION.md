# Disk observations

`node-observe.py` reports allocated disk bytes without cleaning anything. Run it
as your ordinary user, by hand, a few times a day:

```sh
python3 src/maintenance/node-observe.py --repo /usr/local/src/ai-agent-bus
```

Each run adds a private, timestamped report under `REPO/tmp/b15/` (gitignored),
with `df` output and growth since the previous readable report. Keep these files
as the observation history. The first report has no growth baseline. No root,
installation, timer, cleanup command or bus lock is needed to observe.

The report covers the current user's Go build cache (`go env GOCACHE`), the
checkout's shared Rust target and `tmp/`, ABRT, system and daemon logs, daemon
and record backups, `/tmp`, and releases other than the current installed one.
Old release symlink aliases are excluded. GNU `du` uses allocated bytes and
stays on each input's filesystem; the categories overlap and must not be added.
The report directory itself is part of checkout `tmp/`.

Unreadable entries, changing files and command failures are labelled `partial`,
`timeout` or `unavailable`; partial byte counts are lower bounds. A missing path
is distinct from an empty directory. Growth is calculated only between complete
measurements of the same path. Reports capture a changing node, not an atomic
filesystem snapshot. Permission-limited results cannot establish retention bounds.
This script proposes no bound and changes none of the existing cleanup policy.
