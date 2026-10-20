#!/bin/sh
# Usage: check-deploy.sh '#debug@node' (uses the caller's normal bus credentials).
set -eu
name=${1:?usage: check-deploy.sh '#debug@node'}
answer=$(agent-bus call "$name" --wait 20s selftest)
printf '%s\n' "$answer"
# An absent/malformed report is not a pass either.
printf '%s' "$answer" | python3 -c 'import json,sys
s=sys.stdin.read()
try:
 v=json.loads(s); s=v.get("body",s) if isinstance(v,dict) else s
except ValueError: pass
sys.exit(0 if "❌" not in s and s.count("✅")==6 else 1)'
