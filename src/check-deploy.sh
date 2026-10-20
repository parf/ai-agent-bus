#!/bin/sh
# An explicit target overrides the local host's usual debug instance.
set -eu
name=${1:-}
if [ -z "$name" ]; then
  realm=$(agent-bus status 2>/dev/null | python3 -c 'import json,sys
try:
 name=json.load(sys.stdin).get("you", "")
 print(name.rsplit("@",1)[1] if "@" in name else "")
except (ValueError,TypeError,AttributeError): pass' || true)
  [ -n "$realm" ] || realm=$(hostname -f)
  name="#debug@$realm"
fi
# Capture errors separately so successful delayed-ack chatter stays hidden.
python3 - "$name" <<'PYTHON'
import json,re,subprocess,sys
answer=subprocess.run(['agent-bus','call',sys.argv[1],'--wait','20s','selftest'],capture_output=True,text=True)
if answer.returncode:
    lines=answer.stderr.splitlines()
    print(lines[0] if lines else 'selftest call failed',file=sys.stderr)
    sys.exit(1)
s=answer.stdout
try:
    v=json.loads(s)
    s=v.get('body','') if isinstance(v,dict) else ''
except ValueError: pass
if not isinstance(s,str): s=''
if s: print(s.rstrip())
total=re.search(r'^Checks: ([1-9][0-9]*)$',s,re.MULTILINE)
passed=sum(line.startswith('✅ ') for line in s.splitlines())
sys.exit(0 if total and '❌' not in s and passed==int(total[1]) else 1)
PYTHON
