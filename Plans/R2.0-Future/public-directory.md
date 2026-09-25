# Public directory proposal

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## A public directory of people and their keys

**The want:** the daemon exports, publicly and **on by default**, the list of
its people and each one's public details — the way GitHub does — and above all
**a user-to-public-key endpoint reachable with `curl`**.

| | |
|---|---|
| why it is wanted | **one place decides who may log into a fleet.** This is a pattern already running: `sshd` is given an `AuthorizedKeysCommand`, the command asks a directory and falls back to `curl <keyserver>/<user>`, and the answer is the `authorized_keys` lines for that person. Add someone centrally and they have every box; remove them and they have none, at the next login — no file copied anywhere |
| and the smaller reason | enrolment already pulls keys this way, from `github.com/<login>.keys` ([identity § registration](../../docs/01-identity-and-roles.md#registration)). A bus is a realm of people with Ed25519 keys, which is the same thing GitHub is being used as — so a peer bus, or a person adding a colleague, fetches a key with the tool everyone already has and no account anywhere |
| the shape to copy | GitHub's: **one key per line, nothing to parse**. A `curl` into `ssh-keygen` or an `authorized_keys` file is the use, and a JSON envelope would only be in the way |
| what makes it a decision rather than a feature | **every other call carries a token** ([access](../../docs/02-access.md#access)) — the invariant is that authentication is always on. A public read surface is an *exception* to that, and it has to be written down as one rather than arriving quietly as a convenience |

The proposed responder is the main daemon. The current
[loopback and body trust boundary](../../docs/02-access.md#what-a-call-carries) does
not authorize a public endpoint: exposure, authentication exceptions and field
visibility require the [open decisions](QUESTIONS.md#open-questions), independently
of any future encryption work. This proposal has no assigned release.

Unresolved details: [questions](QUESTIONS.md#open-questions).
