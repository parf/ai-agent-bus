#!/bin/bash
set -euo pipefail
export AGENT_BUS_NAME=$EXAMPLES_OWNER AGENT_BUS_TOKEN=$EXAMPLES_OWNER_TOKEN
file=${1:-${DEMO_FILE:?set DEMO_FILE or pass a path}}
body=$(python3 -c 'import json,sys; print(json.dumps({"path":sys.argv[1]}))' "$file")
for attempt in 1 2; do
  answer=$(agent-bus call '#stats-rust@localhost' --topic digest "$body")
  printf '%s' "$answer" | python3 -c '
import hashlib,json,pathlib,sys
value=json.loads(json.load(sys.stdin)["body"])
if "error" in value: raise SystemExit(value["error"])
expected=hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest()
assert value["sha256"]==expected and value["java"]["sha256"]==expected
assert value["java_verified"] is True
if sys.argv[2]=="2": assert value["cached"] is True, "second digest was not deduplicated"
print(json.dumps({k:value[k] for k in ("worker","path","bytes","sha256","cached","java_verified")}))
' "$file" "$attempt"
done
# An outside path answers alike whether or not it exists.
there=$(agent-bus call '#stats-rust@localhost' --topic stats '{"path":"/etc/passwd"}')
missing=$(agent-bus call '#stats-rust@localhost' --topic stats "{\"path\":\"/etc/passwd-missing-$$\"}")
python3 -c '
import json,sys
there,missing=(json.loads(json.loads(a)["body"]).get("error") for a in sys.argv[1:])
assert isinstance(there,str) and there==missing, (there,missing)
' "$there" "$missing"
