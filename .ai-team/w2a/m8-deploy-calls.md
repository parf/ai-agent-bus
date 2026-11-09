# M.8: installing, configuring and removing an agent by calling the runner

📌 **TL;DR:** M.8 makes install, configure and remove calls to `#runner/<person>@<realm>`,
so nobody needs a shell on the host. Configure is already built as the file calls;
this proposes the bodies of `install` and `remove`, and how the code arrives.
Recommended: code arrives as a **bundled package name** or a **git URL pinned to a
commit**; configuration stays write-only.

For the owner. Proposal only: nothing here is built or decided.

## What is already settled or built

| | Source |
|---|---|
| The runner is an agent; who may deploy is its ACL; no second door | [reaching the runner](../../Plans/R1.0-Release/runner-implementation.md#reaching-the-runner) |
| The runner never mints a credential: deploying runs code, never impersonates a name | same section |
| Configuration is write-only; the host is a write-only deployment target | same section |
| `read`, `write`, `create`, `delete` of the host's files, Owner and Maintainers only; an env `read` answers names and a digest, never values; `write` checks, replaces and applies as `reload` | [the host's files over the bus](../../docs/runner/operations.md#the-hosts-files-over-the-bus) |
| A message body is at most about 1 MiB | the daemon's request limit |

So **configure needs no new call**: `create`, `write` and `delete` are it. M.8
adds `install` and `remove`.

## How code arrives

| | A. Bundled name 🌟 | B. Git URL at a commit 🌟 | C. Archive in the body | D. From a bus record |
|---|---|---|---|---|
| Body | `install <agent>` | `install <agent> git <url> <commit>` | `install <agent> archive <sha256>` + base64 `tar.gz` after the first line | `install <agent> from <record>` |
| Where the bytes come from | the release's own agents, already on the host and covered by its manifest | the runner clones the URL and checks out exactly that commit; a branch or tag is refused | the message itself | nowhere yet: the daemon serves no content ([resources](../../docs/03-records-resource.md#what-you-get)) |
| Trust | the release's | the commit hash: what was reviewed is what runs | the stated digest | — |
| Limits | only what ships | the host needs `git` and a route to the URL; a private repository needs a key the host already has, never one sent in the call | about 700 KiB after base64 | needs a content store first |
| Fits | every bundled agent (Telegram, Slack, cron, …) | everything else | a small script, if wanted later | not in R1 |

**Recommendation: A and B.** A covers the bundled agents with nothing new to
trust; B covers the rest with a pin that makes "what runs" checkable. C is easy
to add later for small scripts; D waits for a content store.

## The calls

Same form as the built file calls: the verb and its words on the first line,
anything after it is content; the answer is `status`, or `error: …` with
nothing changed.

| Call | Does |
|---|---|
| `install <agent>` / `install <agent> git <url> <commit>` | places the package at `agent.d/<agent>`, checks its `agent.json` as `reload` would, then applies; refused if `<agent>` exists |
| `install --replace <agent> …` | the same over an existing package, whose `runner/` files, work, kept token and bus record stay; a changed package restarts its kept processes, as `reload` does today |
| `remove <agent>` | removes `agent.d/<agent>`, its `agents.json` entry and its instances' env and override files, then applies; its work directories, kept tokens and bus records stay, as `delete` of an instance keeps them |
| configure | the built `create`, `write`, `delete` |

| Rule | |
|---|---|
| Who | the runner's Owner and Maintainers, as for the file calls; anyone else is refused |
| Nothing comes back | no call answers an env value or a Secret; `read` stays as built |
| Atomic | a package is fetched and checked in a scratch directory under the runner's home, then moved into place; a failed check leaves the host as it was |
| Not | a way to mint or hand over a token: a new agent's record is created the way `reload` creates one today, by the runner's person |

## Questions for the owner

1. Code arrives as A and B? (recommended)
2. Does `remove` keep work, tokens and records, as `delete` does (recommended), or is a separate `purge` wanted?
3. Should `install` of an agent that already exists be refused unless `--replace` is given (recommended), or replace silently?
