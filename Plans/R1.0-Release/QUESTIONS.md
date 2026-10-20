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

## Unattended Claude sessions

- ❓ Q185 How `ab-claude` passes Claude Code's development-channels confirmation
  - Context: `ab-claude` loads the bus push channel with `--dangerously-load-development-channels server:agent-bus` (`src/launchers/launcher.ts:475`), and Claude Code asks "I am using this for local development" on every start; a session started unattended ([B.12](TODO.md#bundled-agents)) waits there
  - Options: package the MCP server as a plugin in our own marketplace and allowlist it in managed settings, then launch with `--channels plugin:agent-bus@…`; or the launcher answers that one prompt in its own terminal when the exact text appears; or drop channels for Claude and pull the inbox some other way
  - Impact: the plugin route is the supported one, if managed settings honour the allowlist for this account; the launcher answer is quick but breaks when Claude Code changes the text; without channels, an idle session gets no push

