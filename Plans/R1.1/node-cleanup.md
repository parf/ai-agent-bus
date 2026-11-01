# Node cleanup

📌 **TL;DR:** B.15 moved to R1.1 on 2026-11-01 (Q299). Observation does not authorize a timer, install or deletion.

## Lasting bounds

[RD110](../R1.0-Release/DECISIONS.md#recorded-decisions) requires stale session records to be removed from a list the owner sees first, with a daemon backup beforehand, and lasting bounds on build caches, crash dumps, old releases, logs, backups and test leftovers.

The [report-only script](../../src/maintenance/OBSERVATION.md) runs manually as an ordinary user; timestamped samples stay in ignored `tmp/b15/`. The first sample was captured on 2026-10-30. Repeated observations precede choosing growth bounds. [B.15 acceptance](TODO.md#node-cleanup) remains unfinished.
