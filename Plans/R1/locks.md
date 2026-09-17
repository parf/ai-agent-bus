# Shared locks

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Shared locks

**The daemon hands out named locks, and one holder has one at a time.** It is
the same authority that already decides which reader gets a message
([one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)), turned on something the bus
does not itself hold: a pool dividing a list of work, two hosts that must not
run one job twice.

| Verb | |
|---|---|
| **`lock`** | take *name*, for *ttl*. **Blocking**: the caller waits until it is granted or their own wait runs out |
| **`try-lock`** | the same, **non-blocking**: granted or refused, now |
| **`release`** | give it back before the ttl |

| | |
|---|---|
| **why the daemon, and not a service** | a pool already shares exactly one thing — the bus it talks to ([runner § one name on many hosts](runner.md#one-name-on-many-hosts)). Anything else able to order two writers is a second authority to install and keep agreeing with, and **a lock service could not itself be a pool** without consensus: two processes behind one name would both believe they had granted it |
| **and because it is small** | the three verbs, a deadline and a map. Enough services need it that leaving it out means each of them invents one, which is the case for building it in rather than against |
| **basic is the whole of it** | the verbs above, the ttl, the set. Fair queuing among waiters, reentrancy, locks that outlive their holder — none of that is here, and whoever needs one of them is describing a service of their own |
| **every lock has a ttl, and it is asked for** | holders die. Without a deadline one crash wedges a pool forever, so there is no lock without one and holding it longer means asking again. A `release` is the fast path, never the only one |
| **each grant carries a number that only goes up** | a ttl alone is not safety: a holder that stalled past its deadline still believes it holds the lock. The number is what lets whatever the lock guards refuse the older holder, and it never repeats — a daemon that restarted issues higher numbers than the run before it |
| **it is live state, and it is not dumped** | queues and stats survive a restart ([durability](../../docs/04-messaging.md#durability)); locks must not. A lock that outlived the daemon that granted it is a claim about processes nobody watched in the meantime. **A restart releases everything**, which is the honest answer and the reason the number above exists |
| **the holder is a principal** | the token says who ([access](../../docs/02-access.md#access)), so a listing can answer *who holds this* — and the daemon watches no connection, here as everywhere. The ttl is what ends a lock, not a socket closing |
| **it holds nothing** | a lock says who may act and stores no value. What the holders agree *about* lives wherever they keep it, which is a separate question ([1.2 § shared secrets, and a KV with locks](../R1.2/exploration.md#shared-secrets-and-a-kv-with-locks)) |

### A set of locks

**A set is a named group of locks, and it is how a shared service shares its
resources.** The service owns some countable thing — four GPUs, eight browser
sessions, the seats on a licence, a handful of outbound addresses — and
declares a set with one lock per resource. Whoever holds one of the set's locks
holds one of the things.

| | |
|---|---|
| **take a named one, or take any free one** | `gpu2` when it has to be that one; *any* when it does not, and the answer says **which** was given |
| **that answer is the point** | a counting semaphore says *you may proceed* and leaves two holders to pick the same GPU. A set says *you have `gpu2`*, which is the whole difference and the reason this is named locks rather than a number |
| **the set is a name, so it is granted like one** | who may take from it is the ordinary ACL question ([identity § acl](../../docs/02-access.md#acl)), asked once about the set rather than per resource |
| **empty behaves like a held lock** | `lock` waits for the first one returned, `try-lock` is refused now. Nothing new: it is the two forms above, asked of a set |
| **and it answers how many are free** | cheap, and the number a dashboard or a queue-depth alarm wants |

Everything else is unchanged — a ttl on every grant, a number that only goes
up, and nothing kept across a restart.

**From the daemon's side this is not a second mechanism.** A plain named lock
is a member of the **default set**, and the sets above are the same thing with
a name of their own. What a declared set adds is the one claim the default set
cannot make: **its members are interchangeable**. That is what *take any free
one* means, and why it is meaningless in the default set — `deploy@srv1` and
`migrate@srv1` are both in there and are not alternatives to each other.
