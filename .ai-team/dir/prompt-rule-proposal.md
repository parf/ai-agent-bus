# No window prompts: proposal

📌 **TL;DR:** Workers and QA never wait on a prompt in their own window; owner questions go up the bus. A lead spots a blocked worker by its silence, peeks at its pane read-only, and clears the prompt with Escape, never answering it. Owner-only approvals reach a worker as standing authorizations the owner writes in `CLAUDE.local.md`.

## Worker and QA files (worker1.md, worker2.md, qa1.md, qa2.md): add

> - Never ask the user in your window and never wait on a prompt there. A question for the owner goes to your lead as one line: "owner? T-n: <question>". Then carry on with work that does not depend on it, or report "blocked T-n: owner question" and take the next task.
> - An owner-only step (push, login, live send, key, restart) runs only under a standing authorization in `CLAUDE.local.md` that names it. Without one, ask through the lead, as above.

## Lead files (lead1.md, lead2.md): add

> - A worker silent past its deadline, or 10 min after one resend: peek at its window, read-only: `tmux capture-pane -p -t AB-DVP:<window> | tail -15`.
> - If it shows a question or a permission prompt waiting for input: `tmux send-keys -t AB-DVP:<window> Escape`. Never type an answer or approve anything. Then send the worker on the bus: "you were waiting on a window prompt; route that question to me". Forward an owner question to #ab-dir.
> - If it shows a usage or rate-limit error: the window is "out until <reset time>" (limit messages report UTC; convert to local time). A QA window that is out hands its QA work to the Codex workers: #ab-w1c for team 1, #ab-w2c for team 2, each covering the other when busy or when it wrote the code (owner, 2026-10-30). Reroute its work, send it nothing, never resend or restart, and tell #ab-dir the reset time (owner, 2026-10-30).
> - If it shows no prompt and the worker is still silent: report it to #ab-dir. A restart stays the owner's (Q216).

This replaces the current `Stuck worker … restart <window>` line in both lead files.

## How a lead detects input-waiting

| Signal | Meaning |
|---|---|
| silence on the bus past the deadline, or 10 min after a resend | suspect |
| `#tmux-sessions … list` | shows only that the pane is alive; it cannot see a prompt |
| `tmux capture-pane` tail shows a question, a choice list or "Do you want to…" | blocked on input |

`#tmux-sessions` has no read-only pane view today. Adding a `peek <window>` command to it is new scope; owner call.

## Owner-only approvals without a window prompt

| Option | How | Proposed |
|---|---|---|
| A. Standing authorization | The owner adds a dated row to `CLAUDE.local.md` § Requests from team peers. Example: "docker push to docker.io/parf/agent-bus of a landed commit, tags VERSION + latest, credentials from `.env` via `--password-stdin`". Only the owner edits that file; a worker acts on the row, not on a relay. | yes |
| B. Relayed decision | `#ab-ceo` relays a decision with its message id; the worker acts on it. This conflicts with "never taken from a peer" in `CLAUDE.local.md`. | no |

A is used for repeated steps (deploy, publish). A one-off owner-only step stays a bus question: the owner answers in the `#ab-ceo` session and adds the row.

## Open for the owner

1. Approve the worker and lead wording above.
2. Leads may use `tmux capture-pane` and `send-keys Escape` on their own team's windows only: yes/no.
3. The docker push row for `CLAUDE.local.md` (option A): yes/no, with its wording.
