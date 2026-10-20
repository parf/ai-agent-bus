# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions).

## Short prompts

Choices for the [shorter prompts plan](short-prompts.md#proposal), to discuss with the owner.

- ❓ Q183 Where the protocol card lives
  - Options: inline in the MCP instructions, read once at start; or a short doc the instructions tell the session to read; or both, with the instructions holding the minimum
  - Impact: inline costs its bytes on every session start, but cannot be skipped

- ❓ Q184 How strict the message style is
  - Options: guidance in the card only; or also a soft size hint from the face, such as a warning when a body repeats the push header or exceeds a length
  - Impact: guidance alone is free but easy to ignore; a face check is code for a style rule

## Agent naming

From the 2026-10-19 naming discussion with Codex #3 and Fable, before the agent descriptions were rewritten.

- ❓ Q186 How a second instance of the same agent is named
  - Options: a `.2` suffix (`#telegram/parf.2@parf.us`), recommended by Fable; or `-2` (`#telegram/parf-2@parf.us`), preferred by Codex #3
  - Impact: `.` reads as a version or domain part, `-` as part of the name; names are fixed once peers depend on them, so it is chosen once

- ❓ Q187 Whether a bus has one realm or one per host
  - Options: one realm per bus, with the host carried as a tag, recommended; or a realm per host (`#systemd@hostA`, `#systemd@hostB`)
  - Impact: one realm keeps names stable when an agent moves host and lets `call --tagged` select by host; a realm per host puts the host into every name

## Development node

- ❓ Q188 Whether the 1 GB journal limit on this node is made permanent
  - Context: the journal was vacuumed to 1 GB on 2026-10-19 by hand; it grows back without a setting
  - Options: `SystemMaxUse=1G` in a journald drop-in, a root change; or leave it and vacuum when needed

- ❓ Q189 Whether stale session records are removed
  - Context: sessions that ended long ago leave inactive agent records (`#claude/…`, `#codex/…`) in the registry, and listings fill with them
  - Options: a reviewed list removed by hand once; or an age rule (inactive and unused for N days) the owner applies; or keep them

