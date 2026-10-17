#!/usr/bin/env python3
"""debug: a script agent for checking what a runner hands its script.

The message body arrives as one argument (form args); its first word is the
command:

    hello <name>   answers "hello, <name>"
    env            answers every environment variable, one NAME=value a line

Anything else is answered with the commands it knows. Asked `--help`
(AGENT_BUS_ASK=help, with no input) it describes itself for an AI reader
(docs/constitution.md#agent-self-description); any other ask is no input. It
needs no bus library and holds no credential: the runner does the bus
talking.
"""
import os
import sys


HELP = """# debug: what a runner hands its script

A script agent for checking a runner: it shows what an agent it serves is
given. Call it with text; the first word is the command.

- hello <name>
  Answers "hello, <name>" ("hello, world" without one): the runner ran the
  script and carried the answer back.
  Example: agent-bus call '<this agent>' 'hello smoke'
- env
  Answers every environment variable the script was given, one NAME=value a
  line: the envelope (AGENT_BUS_FROM, AGENT_BUS_TOPIC, ...), the agent's env
  layers and the held roles (AB_ROLE_*). It never holds a token. Whoever this
  agent's ACL admits sees all of it, its env layers' values included."""


def answer(body: str) -> str:
    words = body.split()
    if not words:
        return "debug: send `hello <name>` or `env`"
    command, rest = words[0], words[1:]
    if command == "hello":
        return "hello, " + (" ".join(rest) if rest else "world")
    if command == "env":
        return "\n".join(f"{k}={v}" for k, v in sorted(os.environ.items()))
    return f"debug: no command {command!r}; send `hello <name>` or `env`"


if __name__ == "__main__":
    if os.environ.get("AGENT_BUS_ASK") == "help":
        print(HELP)
    else:
        print(answer(" ".join(sys.argv[1:])))
