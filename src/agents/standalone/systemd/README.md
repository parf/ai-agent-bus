# Systemd agent

📌 **TL;DR:** B.2 manages one declared set of host units through fixed systemctl
commands. It runs under its own root unit, using the shared standalone starter;
the bus supplies caller roles and the agent enforces them before invoking systemctl.

## Calls

The B.2 format is text `list`, `status <unit>`, or `<verb> <unit>`.
JSON uses exactly `{"method":"list"}` or
`{"method":"status","unit":"nginx.service"}`; substitute another supported verb.
JSON answers contain stdout, stderr and exit status; failures include `error`.
Text failures begin with `error:`. Commands have a 30-second bound.

| Delivered role | Methods |
|---|---|
| none | list, status |
| operate | list, status, start, restart, reload |
| admin | all above, stop, enable, disable |

Only delivered `AB_ROLE_OPERATE` and `AB_ROLE_ADMIN` grant these tiers.
Body fields cannot grant a role. Every tier, including admin, is restricted to
the configured units. Names must be exact, fully qualified names; options,
paths, globs, escapes and extra arguments are refused before systemctl runs.
An inactive unit's nonzero status is returned as an error with systemctl's output.

## Install

Create a root-owned JSON file, not writable by group or others, containing
`{"units":["nginx.service"]}`. The list must be nonempty and contain no duplicates.
The file is validated both during installation and for every call.

With the caller's administrative bus credentials available to the installer:

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN python3 src/agents/standalone/systemd/install.py install \
  --name '#systemd/web@host' --user root --units-file /etc/agent-bus/web-units.json \
  --allow '#operator@host(operate),#administrator@host(admin)'
```

Use `remove --name '#systemd/web@host'` with current administrative credentials
to remove the unit, token file and record. The scope file remains administrator-owned.
The [shared starter](../starter.md#standalone-starter) owns credentials and lifecycle.
The scope path is fixed installer configuration, never a caller argument.

## Verification

The slow `agent_systemd` shard checks role tiers, exact scope, body forgery,
argument refusal, installation idempotence and removal against a private bus.
Its unit controller uses real root and nobody processes; its systemctl stand-in
writes a root-owned state file. Losing root privileges makes every acting verb
return a permission error while list and status still work. This is an isolated
privilege check, not a live systemd deployment; it never changes host units.
Mutants remove the role or scope check, trust a body role, or hide a systemctl
failure; each must fail its named assertion. NoNewPrivileges alone does not
remove an already-root process's credentials.
