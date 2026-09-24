# Future Web face rewrite

📌 **TL;DR:** Follow-up the development-environment plan leaves out on
purpose. Nothing here is scheduled.

| Topic | Note |
|---|---|
| Release packaging | build the face into each release (`bun build --compile` works: one stamped executable), link `/var/lib/agent-bus/web` to `current/web`, restart the unit on every switch, and stop it with a named reason on a rollback to a release without `web/` |
| Setup | `agent-bus-setup` creates the account and unit, with Go tests of the unit text, in place of `install-dev.sh` |
