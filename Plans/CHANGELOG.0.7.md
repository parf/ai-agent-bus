# Changelog 0.7

📌 **TL;DR:** Archived changes **0.7.0–0.7.20**, 2026-09-22–2026-09-23, grouped by area. The line closed at **0.7.20**. Earlier: [0.6](CHANGELOG.0.6.md#changelog-06); next: [0.8](CHANGELOG.0.8.md#changelog-08).

## Grouped changes

| Area | Versions | Summary |
|---|---|---|
| Store and durable identity | 0.7.1, 0.7.3, 0.7.6, 0.7.19 | Replaced JSON snapshots/token files with one exclusively held SQLite database. Management writes committed before acknowledgement; internal IDs were durable and never reused. Credentials bound User/Agent IDs, transfer rebound them and removal deleted them atomically. A never-written database was not reported as an unclean stop. |
| Names and ownership | 0.7.4–0.7.5, 0.7.7 | Made realms optional but identity-significant, required `#` on Agent names and User ownership of records, and allowed Personal on every kind. Email, GitHub and case-insensitive Twitter/X identities became unique and syntax-checked; invalid stored records were ignored and reported rather than deleted. |
| Records and lifecycle | 0.7.8–0.7.10, 0.7.17, 0.7.20 | Active/inactive replaced paused/banned/disabled states; an inactive entity granted nothing and reserved its name. Schema 4 replaced the separate groups table with Group records, nested members had to exist and account mappings belonged to actors. The daemon-account socket followed its durable Owner. Secrets became validated ENV files; malformed secrets/configuration were ignored on load. |
| Routing and atomic lists | 0.7.11–0.7.12, 0.7.20 | Added atomic list deltas and one-slot forwarding on agents/queues. PubSub could fan out through groups, queues and further topics with a ten-step bound; destination ACLs judged the source, and a publication nobody received was refused. Final review rechecked delivery authority at handoff and released readers after policy changes; 0.8.189 later replaced that timing with admission-only checks. |
| CLI, web and diagnostics | 0.7.2, 0.7.14, 0.7.16–0.7.18 | Added audit/error/debug logs, actor-aware management/group commands, list errors and route observations. Forms followed the new model; failed routes and persistence refusals reached the error log, while an ordinary unknown-name send remained a caller refusal. |
| Installation and close-out | 0.7.13, 0.7.15, 0.7.17–0.7.20 | Added explicit destructive `--reinstall`, setting aside old state/unit drop-ins before installing fresh; ordinary setup reused a running node's database. Reinstall handled missing units/logrotate directories. Final review tightened actor, credential-restore and route-source rules before the stable close. |
