# Design choices

📌 **TL;DR:** The reasons behind the bus's main boundaries: stable identities,
caller-controlled access, bounded messaging and separate execution processes.
Follow each link for the full behavior and limits.

## Settled

### Identity and names

- [Names](01-identity-and-authority.md#names) identify records independently of a running process, so an agent can stop and resume without changing its address.
- [Conditional registration](01-identity-and-authority.md#registration) arbitrates competing new claims. [Unregistration](01-identity-and-authority.md#unregistering) removes old group membership so a later holder does not inherit access.
- [Administrative and record authority](01-identity-and-authority.md#positions-and-scopes) are separate: administering people does not make someone every record's maintainer.
- [A Personal record's Allow may name `#bus`](03-records.md#personal-and-shared), its Maintainers may not (Q328).

### Access and credentials

- The daemon checks [identity and authority](constitution.md#authority-at-the-door) on incoming requests. Faces use that answer instead of maintaining independent permission rules.
- [Private values](constitution.md#-private-values) have their own record-level authority; permission to manage or call an agent does not automatically expose its secrets.
- [Message bodies are untrusted input](02-access.md#message-bodies-are-untrusted-input); authenticated routing and roles come from the envelope, not claims in the text.
- [Transfers](01-identity-and-authority.md#record-authority) preserve an Agent's credential. Rotate it when a previous holder must lose access.

### Records and configuration

- [Services](03-records-service.md#what-a-service-is) describe external systems; Agents answer through the bus. This separates discovery from execution.
- [Configuration and Secret](03-records-agent.md#configuring-a-template) have different presentation rules so settings can be inspected without exposing credentials.
- [KV](03-records-kv.md#rules) and [locks](01-identity-and-authority.md#shared-locks) belong to records, giving shared state the same management boundary as the work it supports.
- [Resources](03-records-resource.md#what-a-resource-is) describe data; their sources supply the contents when requested.

### Messaging and delivery

- [Taking a message and finishing work](04-messaging.md#receipts) are separate events. Receipts are optional reports from the receiver.
- [Timeouts](04-messaging.md#timeouts-and-uncertain-outcomes) leave uncertain outcomes; automatic repetition could duplicate an external effect.
- [Deliver-To and ACL](04-messaging.md#deliver-to) answer separate questions: who receives and who may publish. Forwarding moves a message; PubSub makes recipient copies.
- [Shared readers](04-messaging.md#several-readers-may-wait-when-they-say-so) must opt in so an accidental second reader is visible instead of silently taking another client's work.

### Daemon and storage

- [Queue checkpoints](04-messaging.md#durability) trade per-message persistence for throughput. An unclean stop can lose recent traffic or redeliver a consumed message.
- [Administrative changes](04-messaging.md#administrative-crash-recovery) commit before success: a crash must not silently lift an acknowledged restriction.
- The daemon runs a fixed signature verifier, but [never user-supplied programs](08-processes.md#nothing-the-daemon-runs-may-exec). Execution belongs outside the state-serving process.

### Runner and execution

- [Dedicated scripts](06-dedicated-runner.md#script-agents) use one foreground reader; [installed agents](runner/README.md) use the runner for supervision and configuration.
- [Agent metadata](constitution.md#agent-self-description) lives with the agent's package, where methods and settings can change together.
- [A package may be a linked checkout](runner/operations.md#install-a-custom-package): an owner develops an agent in its own git checkout and links it in, without a copy step.
- [Temporary controls and persisted enablement](runner/operations.md#controlling-one-agent) are separate so stopping work now does not silently change what starts later.
- [Host secrets](runner/security.md#secrets) stay under host access; bus-visible metadata need not disclose their values.

### Bundled agents

- [Helper subscriptions](agents/subscriptions.md) forward external events with explicit permissions and observable delivery limits.
- [When a subscription's source is checked](agents/subscriptions.md#source-check) is each helper's: at creation, or again before every send where revoking the author's reach must stop the copies (Q326).
- [Git](agents/git.md#who-may-use-it) and [systemd](agents/systemd.md#who-may-use-it) apply application permissions within each instance's configured limits.
- [Mail content](agents/imap.md#use-examples) stays visibly distinct from trusted instructions.

### Web, CLI, MCP and launchers

- The [web face](08-processes.md#the-web-face) acts as the visitor and has no privileged fallback.
- [Public node identity](05-discovery.md#what-a-node-says-about-itself) lets a visitor identify the node before signing in; its disclosure includes uptime and aggregate request counts.
- [Session names](06-launchers.md#session-names) remain reserved until explicitly released, so a dormant conversation does not lose its address to another session.
- [Takeover](06-launchers.md#one-live-session-per-name) is explicit, and [runtime control](06-launchers.md#runtime-isolation-and-recovery) requires credentials even on loopback.
- [MCP bigfile tools](03-records-kv.md#bigfiles) use one configured directory and require approval because a model-selected path can expose local data.

### Versioning and project process

- The [API contract](09-daemon-api.md#protocol-version) distinguishes client routes from administrative interfaces.
- Project and client-library terms are in the [project license](../LICENSE.md#polyform-strict-license-100) and [client license](../src/clients/LICENSE.md#mit-license).

### Backups

- [Record backups](administration.md#encrypted-record-backups) are encrypted to the exporting person's key. Import resolves conflicts before committing; backup contents and exclusions determine what can be recovered.
