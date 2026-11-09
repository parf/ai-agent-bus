# Development team

📌 **TL;DR:** Run `./.ai-team/start-agents.sh` from the repository to start the whole team and CEO. Existing live windows are kept.

## Commands

```sh
.ai-team/start-agents.sh             # start all missing team windows and CEO
.ai-team/start-agents.sh --status    # inspect without starting
.ai-team/start-agents.sh --roles     # re-read roles and checkpoints
.ai-team/start-agents.sh --stop      # stop the team; CEO stays running
```

Run from a graphical session with Kitty, or select its remote-control socket with `--to`. The team uses the configured `#tmux-sessions/ab-dvp@parf.us` instance.

## Files

- [Launcher configuration](ab-dvp-v2-config.json) names windows, commands and their working directories.
- Each working directory keeps its handoff and checkpoint notes in Git.
- [Team roles](../Plans/Team.v2/README.md#roles) define responsibilities.
- Gate and deployment scripts live here alongside the launcher.
- Secrets, logs, caches, compiled probes and generated test output are ignored.
