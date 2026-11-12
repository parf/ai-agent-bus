# Federation

📌 **TL;DR:**
- **Problem:** agents on another agent-bus cannot be used from this one without running a second client per agent.
- **Proposal:** an **upstream mount**: this bus logs into a remote bus as one ordinary remote user and shows what that user can use under a local prefix, `#g:mail@a`.
- **Status:** owner design in progress, 2026-11-12 ([RD179](DECISIONS.md#recorded-decisions)); targets a 1.0.x release, but no build starts until the owner says it is ready; open choices are [Q407 and Q408](QUESTIONS.md#federation).

## Upstream mount

- On the remote bus, its owner registers a user and gives out its token. The remote needs nothing new: the mount is one more session of that user.
- On the local bus, a **mount record** owns a prefix, the remote address, and the token as its Secret. The prefix is a local alias the owner chooses, such as `realmo-prod`; one bus can hold many mounts, and the same remote can be mounted twice under different users and aliases. `g` below is an example.
- A **proxy** keeps one outbound connection to the remote as that user, so a local bus behind a firewall or NAT works.
- Remote access is the remote's existing ACL for that user. Locally, each mounted name is a local record with its own full [ACL](../../docs/02-access.md#acl), under the same rules as every other record; a local caller needs both the local grant and the remote user's access.
- The proxy queries the remote live with a short cache for listing, help and answers; locally only each mounted record's ACL and alias are stored.

- **The daemon's rules do not change.** To the local daemon a mounted record is a local record: ACL, locks, key-value authority, deadlines and errors follow the same rules as any other record.

## Names

- The local name is the prefix, `:`, then the full remote name unchanged: remote `#mail@a` and `#mail@b` become `#g:mail@a` and `#g:mail@b`, so remote realms never collide.
- The sigil stays first, so every kind maps the same way: `#realmo-prod:mail@a`, `realmo-prod:alice@a`, `@realmo-prod:ops@a`.
- `:` is refused in every local name today, so no local record can shadow a mounted one and no existing name changes.
- The 64-character name limit applies to the whole local name. When a remote name would exceed it, the mount record maps that long remote name to a short local name under the same prefix, for example `#realmo-prod:cs1`; an unmapped name that is too long is listed as hidden.

## One-way first

Local callers reach remote ones; remote callers never reach local ones.

| Operation | Through the mount |
|---|---|
| Listing | local `ls` includes the remote listing for that user, as `#g:*` |
| `help`, `image` and other asks | forwarded, answered by the remote agent |
| Calls with an answer | forwarded; the local deadline travels with the call |
| One-way messages | forwarded as plain agent calls with no answer |
| Key-value store of a remote record | `kv … '#g:x@a' …` acts on remote `#x@a`, under that user's authority there |
| Shared locks of a remote record | the local daemon applies its ordinary lock rules to the mounted record, so local callers are distinct holders; the remote sees the mount's user as the one holder |
| Big files | not included |

- **Remote down:** every `#g:*` operation fails at once with an upstream-unreachable error, and the listing shows the mount offline. Nothing waits for a deadline.
- **Remote callers** never reach the local bus in this form; the two-way form is [below](#two-way-proxy).

## Two-way proxy

Owner idea, 2026-11-12, not decided ([Q407](QUESTIONS.md#federation)): a transparent proxy in both directions.

- A local agent that wants remote callers opts in on the mount. The proxy user then registers a personal proxy agent on the remote, a 1-to-1 stand-in for that local agent.
- A remote call to that stand-in is carried down the mount's outbound connection and arrives locally as a call from `#g:<remote caller>`, so local ACLs grant remote callers with the same prefix.
- The answer returns the same way.
- New with two-way: a hop limit against mount loops (A mounts B, B mounts A); the remote naming of stand-ins under that user's allowed name space; and that remote user's right to register agents at all.

## Chaining

Retired on 2026-11-12: the mount replaces daemon-to-daemon chaining ([decision archive](../done/decisions-archive.md#federation)). This section keeps inbound links.
