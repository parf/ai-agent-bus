# Constitution check — Oct 20, 2026 · 978f0278..473e6fc3

📌 **TL;DR:** The last two days of commits (since 2026-10-18) were read against
[the constitution](../../../docs/constitution.md) and [current decisions](../../../docs/decisions.md#settled).
The core of "check once at the door" is faithful. Most breaks are a newer owner decision that never reached the
constitution (it is now stale or contradicts the code), plus five code paths that break a stated rule. Nothing was
changed; every item below is for the owner to accept or reject. Items marked *read* were checked by reading
every call site, *ran* by a probe, *not run* by reading only.
