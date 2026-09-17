## MVP

*Proposed.* Somebody other than the author can install it and use it safely on
a shared host.

**Build**

| | |
|---|---|
| identity | a token per principal, per-user sockets ([access](../../../docs/02-access.md#access)) |
| registration | manual record + GitHub ([identity § registration](../../../docs/01-identity-and-roles.md#registration)) |
| **user records** | a maintainer writes them and a person does not, which is what makes the fields trustworthy; the owner is always a maintainer and only the owner touches another one ([identity § who may write a record](../../../docs/01-identity-and-roles.md#users-and-profiles)) |
| **unique identifiers** | every identifying field normalised before it is written and unique across records — email, phone, `GithubUser`, IM handle ([identity § every identifying field is unique](../../../docs/01-identity-and-roles.md#users-and-profiles)) |
| tokens | persisted, previous kept, local never expires ([access § token lifetime](../../../docs/02-access.md#token-lifetime)) |
| encryption | 🚫 *struck* — the daemon issues the token a session key derives from, so end to end against it is not reachable in this stage's key mode; the bus is trusted on its own host ([access § encrypted sessions](../../../docs/02-access.md#trust-boundary)) |
| access | service ACL, then master ACL; a service may refuse master ([identity § acl](../../../docs/02-access.md#acl)) |
| messaging | TTL, `reply-to`, and **pub/sub topics** — a subscription is a `consume:<glob>` capability, which exists once there is an ACL ([messaging](../../../docs/04-messaging.md#messaging)) |
| services | calls grow up: `done` (finished processing) as well as `ack` (got it), caller deadlines, `reply-to` a third party, several workers behind one name, per-service call stats ([messaging](../../../docs/04-messaging.md#messaging)) |
| storage | the store and the dump behind their ports — a text file and JSON today, a database and Parquet as adapters ([setup § storage](../../../docs/09-setup.md#storage), [messaging § durability](../../../docs/04-messaging.md#durability)) |
| faces | the PoC MCP face grown up: generated docs, catalog filtered per caller; a dashboard people sign in to, showing the registry, stuck inboxes, exchanges, losses and refusals ([discovery § what it shows](../../../docs/05-discovery.md#what-it-shows)) |
| starting services | `agent-bus start <name> … <command>` — one command line publishes a service in the foreground, confined if it asks to be. The install lays out **both accounts and the directory tree**, because that is the arrangement and it is cheap; what waits is the runner program that would use the second one ([runner role](../../../docs/08-runner-role.md#foreground-runner)) |
| processes | the supervisor/children split ([processes](../../../docs/11-processes.md#processes-and-privileges)) |
| install | `npm install -g` + `sudo agent-bus-setup`: the programs it brings ([setup § the programs](../../../docs/09-setup.md#the-programs)), the two accounts and the tree they own ([setup § the two accounts](../../../docs/09-setup.md#the-two-accounts)) |

**Works at the end of MVP**

- Several users share one host, each seeing only the services they may.
- The bus restarts without losing queued messages.
- An agent asks the MCP face "what can I use?" and gets a filtered catalog.
- A service started with one command line is registered and reachable, and is
  confined when it asks to be.
- A person signs in to the dashboard with the credential they already have and
  sees the bus as they may see it — and a stranger sees only how to get one.

**Deliberately absent**: AUTH role, groups, chaining, peer sync, billing,
client libraries in other languages. The dashboard's groups, health, load
graphs, child liveness and record origin go with them — each waits on the
thing that would make it true rather than on the page
([discovery § what it shows](../../../docs/05-discovery.md#what-it-shows)).
