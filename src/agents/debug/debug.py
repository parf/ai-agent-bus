#!/usr/bin/env python3
"""debug: a script agent for checking what a runner hands its script.

The message body arrives as one argument (form args); its first word is the
command:

    hello <name>   answers "hello, <name>"
    env            answers every environment variable, one NAME=value a line

Anything else is answered with the commands it knows. It needs no bus
library and holds no credential: the runner does the bus talking.
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
        return "\n".join(f"{k}={v}" for k, v in sorted(os.environ.items()))
    return f"debug: no command {command!r}; send `hello <name>` or `env`"


if __name__ == "__main__":
    print(answer(" ".join(sys.argv[1:])))
