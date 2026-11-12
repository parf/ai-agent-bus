# Federation

📌 **TL;DR:**
- **Problem:** agents on another agent-bus cannot be used from this one without running a second client per agent.
- **Proposal:** an **upstream mount**: this bus logs into a remote bus as one ordinary remote user and shows what that user can use under a local prefix, `#g:mail@a`.
- **Status:** owner design, 2026-11-12 ([RD179](DECISIONS.md#recorded-decisions)); not built, release **undecided**; open choices are [Q403–Q407](QUESTIONS.md#federation).

## Upstream mount

- On the remote bus, its owner registers a user and gives out its token. The remote needs nothing new: the mount is one more session of that user.
- On the local bus, a **mount record** owns a prefix (`g`), the remote address, and the token as its Secret.
- A **proxy** keeps one outbound connection to the remote as that user, so a local bus behind a firewall or NAT works.
- Remote access is the remote's existing ACL for that user. Local access is the mount record's own list ([Q405](QUESTIONS.md#federation)).
- The proxy queries the remote live with a short cache; it never copies remote records into the local store.

## Names

- The local name is the prefix, `:`, then the full remote name unchanged: remote `#mail@a` and `#mail@b` become `#g:mail@a` and `#g:mail@b`, so remote realms never collide.
- `:` is refused in every local name today, so no local record can shadow a mounted one and no existing name changes.
- The 64-character name limit applies to the whole local name; a longer remote name is not reachable through the mount. Aliasing is a possible later addition ([Q404](QUESTIONS.md#federation)).

## One-way first

Local callers reach remote ones; remote callers never reach local ones.

| Operation | Through the mount |
|---|---|
| Listing | local `ls` includes the remote listing for that user, as `#g:*` |
| `help`, `image` and other asks | forwarded, answered by the remote agent |
| Calls with an answer | forwarded; the local deadline travels with the call |
| One-way messages | forwarded as plain agent calls with no answer |
| Key-value store of a remote record | `kv … '#g:x@a' …` acts on remote `#x@a`, under that user's authority there |
| Shared locks of a remote record | as key-value; holder identity is [Q406](QUESTIONS.md#federation) |
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
