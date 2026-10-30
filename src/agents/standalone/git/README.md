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

`git_agent.py` owns bounded parsing, repository caps and the legacy method
vectors. The keeper serves its own bus identity; `autopull.Runtime` routes only
authenticated command-channel deliveries to the event engine. Legacy calls run
in isolated token-free children with delivered roles. Separate bounded pools
keep Git work off the reader. `timers.py` only retires owned legacy units.

`autopull.attempt` serializes through the existing checkout lock, publishes a
safe result, then calls `after_attempt(repo, checkout, record, publish)` while
still locked. `state_directory()` exposes the state root; the script extension
owns an additional bounded budget. `publish` returns acceptance/no-op True or
False on a single refused/uncertain delivery, persisting a local failure note;
it never suppresses the hook. No hook runs without a lock and verified checkout.

## Verification

Disposable bare repositories and the private daemon/unit-controller fixture
check real Git results, channel routing, delivered roles, startup, bounded
concurrency, publication and timer retirement. SSH checks preserve configured-key
and account-default behaviour. Named owning mutants must fail assertions;
no live checkout is reset, cleaned or pushed by these checks.

## Authored help

`agent.json` owns methods, typed arguments and Configuration descriptions.
`env.dist` owns environment-key comments. This package declares `"help":"native"` so
its handler applies runtime Configuration scope and delivered roles to all four
help asks, using the shared manifest helper rather than a second method list. Bundled agents omit `version` and use the bus version.
Executable-grammar checks compare the methods with generated help.
