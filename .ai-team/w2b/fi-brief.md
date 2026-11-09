# find-issues reviewer brief (read-only)

Repo /usr/local/src/ai-agent-bus, range `30215f6..main` (use `main`, local refs only; NEVER git fetch/pull/push, never edit tracked files, never touch the live daemon/db/web).
Read first: .claude/skills/find-issues/SKILL.md (sections Reviewers + Format), .claude/skills/review-checklist.md (every section), Plans/subsystems.md (your row), the CLAUDE.md "Design boundaries" sections that own your paths.
Method: `git log -p --no-merges 30215f6..main -- <your paths>` (and read files at main). Verify every suspicion before reporting: read the line, run a focused test/probe in a scratch dir under /usr/local/src/ai-agent-bus/tmp/fi-<you>/ (Go: GOFLAGS as usual, never `go build ./cmd/X` inside src/; Rust only CARGO_TARGET_DIR=/usr/local/src/ai-agent-bus/tmp/cargo-target). No gates, no mutants, no lock.
Exclude: anything Plans/issues/ files of the last 5 days already hold (grep there by file/symbol first, open and closed, incl. Plans/issues/done/), bugs added and fixed inside the range, pending-by-plan items, Plans/done history.
One cause = one item. At most 10 items, worst first. A claim of absence names every implementation checked.
Output Markdown only, exact item form:
- 🔴|🟠|⚪|🔵 <LETTER><n> `path:line` <claim and consequence, one line ≤120 chars>
  - Evidence: <strongest proof run/read>
  - Impact: <who hits it, how often>
  > 💡👾 <fix, one line>
Use your assigned letter(s) for ids (n from 1). Then one line `Checked and sound: <names>`. Nothing else, no preamble. If nothing verified: just the Checked line.
