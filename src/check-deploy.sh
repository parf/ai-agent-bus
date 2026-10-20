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
answer=$(agent-bus call "$name" --wait 20s selftest)
# Print only the report. An absent/malformed report is not a pass either.
printf '%s' "$answer" | python3 -c 'import json,sys
s=sys.stdin.read()
try:
 v=json.loads(s)
 s=v.get("body", "") if isinstance(v,dict) else ""
except ValueError: pass
if not isinstance(s,str): s=""
if s: print(s.rstrip())
sys.exit(0 if "❌" not in s and s.count("✅")==6 else 1)'
