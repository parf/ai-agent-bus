# Changelog 0.8

📌 **TL;DR:** Changes on the 0.8 feature line, newest first, one or two lines
per version. 0.8 is an even line: it starts from the stable 0.7.20. The
previous line is [changelog 0.7](Plans/CHANGELOG.0.7.md#changelog-07).

## 0.8.2 — 2026-09-23

Push stops when the daemon refuses the credential or the read, instead of
asking again every two seconds for as long as the session lives.

## 0.8.1 — 2026-09-23

`consume --topic` or `--tag` alone selects on that field only; a topic filter
no longer misses every tagged message on its topic.

## 0.8.0 — 2026-09-23

Opens the 0.8 line. A launcher prints a runtime adapter's "attached" status in
green, not in the red it keeps for failures.
