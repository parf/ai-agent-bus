# Container image

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

### The image

Target runtimes: Docker and Podman.

**One image, two containers.** The daemon and the runner are two accounts
exactly so that neither reads the other's secrets
([setup § the two accounts](../../docs/09-setup.md#the-two-accounts)); putting both in one
container gives that up without saying so. Same image, two roles, nothing
shared but the bus.

| | |
|---|---|
| **fast** | one command from nothing to a message delivered, with no file to edit. That is the acceptance criterion, and it is a time rather than a feeling |
| **useful** | the catalogue ships **installed**, and `services.json` enables the reading half only — health, info, and the pair that watches the bus. Everything that acts on the box is installed and **not enabled**, which is a state the design already has and precisely what it is for ([runner § what an instance is](../R1/runner.md#what-an-instance-is)) |
| **persistent** | `/var/lib/agent-bus` is a volume, or the registry and every token die with the container and *a token survives a restart* quietly stops being true ([access § token lifetime](../../docs/02-access.md#token-lifetime)) |
| **the door is a token** | a per-account socket maps a local account, and a container has one user — so people arrive with a token over the port and the socket serves the container's own processes ([access § the three doors](../../docs/02-access.md#the-three-doors)) |
| **sandboxing is off, and says so** | `systemd-run --user` wants a user manager a container does not have. The container is the boundary instead, and `--sandbox on` in there is an **error** rather than a quiet downgrade — which is already how it behaves ([runner § sandboxing](../../docs/08-runner-role.md#sandboxing)) |

Billing is **not in any stage**: it is designed and deferred
([future/billing.md](../Future/billing.md#billing-role--future)).
