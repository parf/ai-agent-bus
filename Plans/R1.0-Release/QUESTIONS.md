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

## Runner env declarations

- ❓ Q190 How env.dist marks a key optional
  - Context: today an empty value (`KEY=`) declares a required key. A key that may stay empty has no spelling, so a package must put a placeholder value or be treated as unconfigured. Separately, the runner treats a package with a missing required key as a template even when `runner/<agent>/env` supplies it, against [configuration § where settings belong](../../docs/runner/configuration.md#where-settings-belong); that is a bug, fixed either way
  - Options: `KEY= # optional` on the same line marks it optional, and a bare `KEY=` stays required (owner's idea, recommended); or a separate `# optional: KEY` comment line; or every empty key optional and required ones listed in agent.json
  - Impact: the same-line marker keeps one file as the source and reads plainly in `--help-secret`; the comment is already free text there, so the marker must be exact (`# optional` as the whole comment or its first word)

