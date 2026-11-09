# T-50 schedule in core — owner design decisions (working notes, #ab-ceo)

Hand to #ab-dir as one hand-off only when the owner says so (owner 14:14: "CTO only advises; give him nothing more yet").
Draft branch: ab-w1b 59b56516 (paused).

## Settled with the owner

- #cron and #at become bus core; any agent schedules its own; all current functionality kept; web Crons/Ats (left nav) like KV with tag filter/count for KV, Crons, Ats; sections on agent pages (owner ~14:00)
- One `schedule` object; `at` = one-shot schedule; Crons and Ats are two views (owner 14:08)
- No migration: no live crons/ats exist (owner)
- Floor 1 s ("more often: write it yourself"); entry cap ~10k (scope open) (owner 14:05)
- Missed runs: queues carry delivery; only daemon downtime needs a rule (owner 14:05)
- Daemon downtime: an `at` past due fires once at start with `late_by`; a cron fires one coalesced catch-up with `missed: N`, then normal schedule (owner 14:2x: "ДА")
- Sender `#bus`; topic always `schedule` (mandatory, fixed); tag free except daemon-event reserved names; daemon stamps `scheduled_by`, `schedule_id`; reply goes to the creator; creator's may-send checked at every firing (owner: "да, пиши так - topic обязательно"; "#2 agree")
  - owner note: event copies reach only explicit subscribers, and nobody can subscribe a name they have no rights over
- Targets: own inbox or anything the creator may already send to (owner "да конечно")
- Body required; empty refused at creation ("empty events - путь к ошибкам")
- Lifecycle/authority like KV (owner "lf")
- Times stored in UTC only; convert only for display (owner)

- Refused firing (CTO-1): skip, count, notice to creator; after X consecutive refusals the timer is DISABLED (not paused) (owner: "после X событий выключал (disable) timer"); X is a plan value
- Optional max lateness, default off (CTO-2) (owner "да - good idea")
- User docs describe the schedule as core bus functionality; receivers key on topic `schedule` + `scheduled_by`, not #bus alone (CTO-7) (owner "да - конечно")
- Optional sixth field for seconds (owner "да")
- Controls: enable (default) / disable, run now, firing history, until, max_runs (owner "yes - and enable(default)/disable - если мы будем выключать сломанные")

## Open

- Alerts/warnings: owner asked to think them through (#ab-ceo proposal pending)

- 3/CTO-4: short-lived names — (a) schedule dies with the name, warn at creation (#ab-ceo pick) or (b) belongs to the person
- 8: cap scope — 10k/node + 1k/name, or 10k per agent
- 10: tags on schedules
- CTO-6: storage is ⚠️ A/C: two readers, migration test, restart-persistence fixture test
- Release (R1.1 has no open scope, Q136)
