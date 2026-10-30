# User guides: find the page for what you want to do

📌 **TL;DR:**
- **Problem:** you want to send a message, run a script or connect an AI session, and do not know which page to read.
- **Outcome:** one short list that points each task to its guide.
- **Start:** the [command line guide](cli.md), then try `agent-bus status` ([Try this first](#try-this-first)).

## Why you'd want it

- 🧭 **Too many pages to guess from.** Now each task has one guide, named by what you want done.
- ⏱️ **Reading theory before typing anything is slow.** Now these guides say what to type; the numbered docs explain why.

## What you get

| | Guide | For |
|---|---|---|
| 🧰 | **[Command line](cli.md)** | register a name, send a message, read your inbox, ask an agent what it can do. **Start here** |
| 🏃 | **[Running an agent](dedicated-runner.md)** | put a script, or a Claude Code, Codex or opencode session, behind a name others can call |
| 🔁 | **[Runner](../runner/README.md)** | install scripts that run when called, without a terminal open ([getting started](../runner/getting-started.md)) |
| 🤝 | **[AI Agents CLI integration](ai-cli.md)** | talk to a running Claude Code, Codex or opencode session from anywhere, and have two cross-review each other |
| 💾 | **[Backups](backup.md)** | export records encrypted to your key, move them, import them back |
| ⚙️ | **[The daemon](daemon.md)** | install it, see what it keeps, and tell whether it is healthy |

## Try this first

- Check that the bus is up and knows who you are:

```sh
agent-bus status
```

```text
{"up":"6h43m35s","realm":"parf.us","records":126,...,"you":"#ab-w2a",...}
```

- If `you` shows your name, go on with the [command line guide](cli.md#try-this-first).

## Technical details

- Why the bus works the way it does: the numbered docs, starting at the [overview](../00-overview.md).
- Installing a node: [setup](../07-setup.md#install).
