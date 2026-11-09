#!/usr/bin/env bash
# Opens #ab-ceo in its own kitty window and returns; #ab-ceo starts the AB-DVP team through the bus.
# A launch from inside a Claude Code tool shell inherits CLAUDE_CODE_CHILD_SESSION, which turns transcript saving off: drop it.
dir=/usr/local/src/ai-agent-bus
kitty --title CEO --directory "$dir/local/ceo" \
  env -u CLAUDE_CODE_CHILD_SESSION AGENT_BUS_NAME='#ab-ceo' AB_TAB_TITLE=CEO \
  ab-claude -2 --tag ab-dvp --descr 'CEO: owner contact for AB-DVP' \
  --model claude-opus-5-5 \
  --append-system-prompt "$(cat "$dir/Plans/Team.v2/roles/ceo.md")" \
  "Start: read /usr/local/src/ai-agent-bus/.ai-team/ceo/handoff.md first; the team is already up (check readers, do not start it again); then report to the owner." \
  >/dev/null 2>&1 </dev/null &
disown
exit 0
