# Git agent

📌 **TL;DR:** Git uses the standalone starter and fixed command vectors; repository caps and caller tiers are intersected for each operation.
Implementation and verification notes stay here; use the [user guide](../../../../docs/agents/git.md)
for setup, calls and settings.

## Calls

See the [calls guide](../../../../docs/agents/git.md#calls).

## Repositories and access levels

See the [repositories and access levels guide](../../../../docs/agents/git.md#repositories-and-access-levels).

## Roles and boundaries

See the [roles and boundaries guide](../../../../docs/agents/git.md#who-may-use-it).

## Install and identity

See the [install and identity guide](../../../../docs/agents/git.md#installing-it).

## Autopull

See the [autopull guide](../../../../docs/agents/git.md#autopull).

## Implementation

git_agent.py resolves revisions to commit ids and paths inside declared roots
before fixed argv construction. Per-repository locks cover calls and timer
updates. Commands run in a private process group, killed and waited on at
timeout, including SSH children. Configured-key mode disables other identities
and account SSH config; server host trust still applies. timers.py owns the
oneshot/timer lifecycle and atomic state outside the repository.

## Verification


The slow `agent_git` shard uses disposable local repositories, a private SSH
server/upstream, and the shared starter's private unit controller. It checks
actual delivered roles, install idempotence/removal, destructive-operation
results, fixed identity, configured-key authentication and default-key fallback.
Mutants remove option/scope/tier checks, trust body roles/author, ignore the key,
or restore more than the named file. No live repository is reset, cleaned or
pushed by the checks; live deployment verification uses list/status/log/rev/diff/autopull only.
A real private systemd-user timer pulls a fixture upstream commit, reports dirty-tree refusal without discarding edits, and is removed afterwards.

## Authored help

`agent.json` owns methods, typed arguments and Configuration descriptions.
`env.dist` owns environment-key comments. This package declares `"help":"native"` so
its handler applies runtime Configuration scope and delivered roles to all four
help asks, using the shared manifest helper rather than a second method list. Bundled agents omit `version` and use the bus version.
Executable-grammar checks compare the methods with generated help.
