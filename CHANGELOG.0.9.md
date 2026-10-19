# Changelog 0.9

📌 **TL;DR:** Changes on the 0.9 development line, newest first, one or two
lines per version. 0.9 is an odd line: it is development and may be broken.
The previous line is [changelog 0.8](Plans/CHANGELOG.0.8.md#changelog-08).

## 0.9.5 — 2026-10-19

Say “no failed units” when systemd triage finds none; JSON still returns an empty unit list.

## 0.9.4 — 2026-10-19

Call visible agents by tag with `call --tagged`, bounded concurrent exchanges and one shared deadline; expose `ls --tag` while preserving single-call correlation flags.

## 0.9.3 — 2026-10-19

Manage all host unit types through systemd's delivered role tiers, with typed listing, timers, failed-unit triage and whole-journal logs. Discover standalone systemd instances by their package tag.

## 0.9.2 — 2026-10-19

Require agent.json descriptions for runner and standalone agents, and publish them on the bus. Expand standalone help and report bare-metal virtualization as a successful “none”.

## 0.9.1 — 2026-10-19

Publish launcher locks with their owner PID already inside. Stale cleanup removes only the observed dead owner, preserving a competing live replacement and fixed session names.

## 0.9.0 — 2026-10-19

0.9 begins: the 0.8 line closed at 0.8.243.
