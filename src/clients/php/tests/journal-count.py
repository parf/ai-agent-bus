"""Count only this shard's PHP processes, including native-library syslog.

Every PHP exec carries the unique syslog.ident ini override in its argv;
forks inherit it. Native syslog can retain identifier 'php', but journald's
trusted _CMDLINE still identifies the emitting process (T-14 gate evidence).
"""
import json
import sys

marker = sys.argv[1]
count = 0
for line in sys.stdin:
    if not line.strip():
        continue
    row = json.loads(line)
    command = row.get('_CMDLINE', '')
    if row.get('SYSLOG_IDENTIFIER') == marker or (
        isinstance(command, str) and 'syslog.ident=' + marker in command.split()
    ):
        count += 1
print(count)
