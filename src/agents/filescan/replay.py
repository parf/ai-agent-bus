#!/usr/bin/env python3
"""Replay a recorded FileScan session with no FileScan running: `replay.py fixtures/filescan-session.jsonl`.

Each line is one step: {"step", "call", "answer", "sent", "rag_calls"}. `sent` are the messages a queue or agent
subscriber received, `rag_calls` what an `as rag` subscriber was sent, and `answer` what the call replied. A consumer
under development reads the same JSON it will get live; `changes()` yields (change, path, message) in arrival order.
"""
import json
import sys


def load(path):
    with open(path,encoding='utf-8') as handle:
        return [json.loads(line) for line in handle if line.strip()]


def changes(steps):
    """Every change message a plain subscriber received, in order."""
    for step in steps:
        for message in step['sent']:
            body=message['body']
            yield body['change'],body['path'],body


def main(argv):
    steps=load(argv[1] if len(argv)>1 else 'fixtures/filescan-session.jsonl')
    for kind,path,body in changes(steps):
        print(f"{kind:8} {path}"+(f"  {body['size']} bytes" if 'size' in body else ''))
    return 0


if __name__=='__main__':sys.exit(main(sys.argv))
