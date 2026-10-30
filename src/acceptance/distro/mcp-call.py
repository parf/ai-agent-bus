"""One MCP tool call through a face started as a command, over stdio.

Usage: python3 mcp-call.py <tool> <json-arguments> <command...>
Prints the tool result's text; exits non-zero on an error result or no answer
within 20 s.
"""
import json
import select
import subprocess
import sys
import time

tool, arguments, command = sys.argv[1], json.loads(sys.argv[2]), sys.argv[3:]
face = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
deadline = time.monotonic() + 20


def send(message):
    face.stdin.write(json.dumps(message) + '\n')
    face.stdin.flush()


def answer(want):
    while time.monotonic() < deadline:
        ready, _, _ = select.select([face.stdout], [], [], deadline - time.monotonic())
        line = face.stdout.readline() if ready else ''
        if not line:
            break
        message = json.loads(line)
        if message.get('id') == want:
            return message
    face.kill()
    sys.exit(f'no answer to request {want}')


try:
    send({'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {
        'protocolVersion': '2025-06-18', 'capabilities': {},
        'clientInfo': {'name': 'distro-acceptance', 'version': '1'}}})
    answer(1)
    send({'jsonrpc': '2.0', 'method': 'notifications/initialized'})
    send({'jsonrpc': '2.0', 'id': 2, 'method': 'tools/call', 'params': {'name': tool, 'arguments': arguments}})
    result = answer(2)
finally:
    face.kill()
if 'error' in result:
    sys.exit(f'{tool}: {result["error"]}')
text = ''.join(part.get('text', '') for part in result['result'].get('content', []))
print(text)
sys.exit(1 if result['result'].get('isError') else 0)
