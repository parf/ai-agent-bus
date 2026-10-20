# Debug implementation

📌 **TL;DR:** Debug serves itself with the Python bus client and its own agent
credential, inside the existing runner sandbox. It exposes bounded diagnostics;
private environment values and the credential are masked. See the
[user guide](../../../docs/agents/debug.md#check-the-bus-in-a-minute).

## Package

`package.sh <agent.d/debug>` keeps `debug.py` as the entry point and includes
its client, shared metadata renderer and version. No-argument startup serves;
explicit arguments run a single method for compatible test fixtures.
The manifest owns all help. No additional privileges or external network access.

## Verification

The normal Go shard runs `test_doctor.py`, including cap, deadline, credential,
selftest-failure and package checks. The runner smoke exercises the live package.
Doctor uses only fixed bus calls and reports refused/unavailable checks.
