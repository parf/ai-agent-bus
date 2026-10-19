#!/usr/bin/env python3
"""debug: a script agent for checking what a runner hands its script.

The message body arrives as one argument (form args); its first word is the
command:

    hello <name>   answers "hello, <name>"
    env            answers environment names with private values masked

Anything else is answered with the commands it knows. The runner generates
help from agent.json without executing this script; asked `--mcp`, it has no tools. It
needs no bus library and holds no credential: the runner does the bus
talking.
"""
import os
import sys


def answer(body: str) -> str:
    words = body.split()
    if not words:
        return "debug: send `hello <name>` or `env`"
    command, rest = words[0], words[1:]
    if command == "hello":
        return "hello, " + (" ".join(rest) if rest else "world")
    if command == "env":
        return "\n".join(f"{k}={v if k.startswith(('AGENT_BUS_', 'AB_ROLE_')) else '***'}" for k, v in sorted(os.environ.items()))
    return f"debug: no command {command!r}; send `hello <name>` or `env`"


if __name__ == "__main__":
    if os.environ.get("AGENT_BUS_ASK") == "mcp":
        print('{"tools":[]}')  # no MCP tools: the MCP shape for none
    else:
        print(answer(" ".join(sys.argv[1:])))
