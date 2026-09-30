#!/usr/bin/env bash
# Run against your selected daemon as the record's Owner, Maintainer or Agent.
# Creates the Agent when absent; resets only the example/* names below.
set -euo pipefail

CLI=${AGENT_BUS_CLI:-agent-bus}
agent=${1:-'#kv-example'}
[[ $# -le 1 && $agent == \#* ]] || { echo 'Usage: examples/kv.sh [#agent-name]' >&2; exit 2; }
command -v "$CLI" >/dev/null || { echo "Missing CLI: $CLI" >&2; exit 2; }
passed=0

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
trace() {
  printf '  $' >&2
  printf ' %q' "$CLI" "$@" >&2
  printf '\n' >&2
}
run() { trace "$@"; "$CLI" "$@"; }
expect() {
  local want=$1 got
  shift
  if ! got=$(run "$@"); then fail "command refused: $got"; fi
  [[ $got == "$want" ]] || fail "expected [$want], got [$got]"
  passed=$((passed + 1))
}
reject() {
  local needle=$1 got
  shift
  trace "$@"
  if got=$("$CLI" "$@" 2>&1); then fail "expected refusal containing [$needle], got [$got]"; fi
  [[ $got == *"$needle"* ]] || fail "wrong refusal, wanted [$needle], got [$got]"
  passed=$((passed + 1))
}
kv() { run kv "$@"; }

# Lookup failures other than a missing record are fatal; never hide connection
# or permission errors by treating them as a reason to register.
if record=$("$CLI" ls "$agent" 2>&1); then
  [[ $record =~ \"kind\"[[:space:]]*:[[:space:]]*\"agent\" ]] || fail "$agent is not an Agent"
  printf 'Using Agent %s\n' "$agent"
else
  [[ $record == *'"error"'* && $record == *'no such name'* ]] || fail "$record"
  run register "$agent" --kind agent --descr 'KV example' >/dev/null
  printf 'Created Agent %s\n' "$agent"
fi
printf 'Testing KV on %s\n' "$agent"

# Keep reset and every section in the same critical section. Another run must
# not reset a key between a write and its comparison. Never force this lock.
lock_name=example/kv
run try-lock "$agent" "$lock_name" --ttl 2m >/dev/null
cleanup() {
  local status=$?
  trap - EXIT
  if ! "$CLI" release "$agent" "$lock_name" >/dev/null; then
    printf 'FAIL: could not release %s on %s\n' "$lock_name" "$agent" >&2
    if [[ $status == 0 ]]; then status=1; fi
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
section() {
  run extend "$agent" "$lock_name" --ttl 2m >/dev/null
  printf '\n%s\n' "$1"
}

section 'Reset example names'
# Fixed test names make reruns deterministic; each kind is a separate namespace.
for name in string int json shared absent; do
  for kind in string int json; do
    flags=()
    [[ $kind == string ]] || flags=("--$kind")
    kv delete "$agent" "example/$name" "${flags[@]}" >/dev/null
  done
done

section 'String, int and JSON: get/set/delete and conditional writes'
for kind in string int json; do
  flags=()
  case $kind in
    string) first='hello'; second='goodbye' ;;
    int) flags=(--int); first=7; second=-3 ;;
    json) flags=(--json); first='{"a":1}'; second='{"a":2}' ;;
  esac
  name="example/$kind"
  reject 'no value is stored' kv get "$agent" "$name" "${flags[@]}"
  reject 'no value is stored' kv set "$agent" "$name" "$first" "${flags[@]}" --replace
  expect "$agent $name set" kv set "$agent" "$name" "$first" "${flags[@]}" --add
  expect "$first" kv get "$agent" "$name" "${flags[@]}"
  reject 'already stored' kv set "$agent" "$name" "$second" "${flags[@]}" --add
  expect "$first" kv get "$agent" "$name" "${flags[@]}"
  expect "$agent $name set" kv set "$agent" "$name" "$second" "${flags[@]}" --replace
  expect "$second" kv get "$agent" "$name" "${flags[@]}"
  expect "$agent $name set" kv set "$agent" "$name" "$first" "${flags[@]}"
  expect "$first" kv get "$agent" "$name" "${flags[@]}"
  expect "$agent $name deleted" kv delete "$agent" "$name" "${flags[@]}"
  expect "$agent $name held no $kind value" kv delete "$agent" "$name" "${flags[@]}"
  reject 'no value is stored' kv get "$agent" "$name" "${flags[@]}"
done

section 'String: empty, stdin and arbitrary bytes'
expect "$agent example/string set" kv set "$agent" example/string ''
expect '' kv get "$agent" example/string
expect "$agent example/string set" kv set "$agent" example/string - < <(printf 'from stdin')
expect 'from stdin' kv get "$agent" example/string
# Bash variables cannot hold NULs: compare the bytes through od instead.
printf '\000\377\001\n' | kv set "$agent" example/string - >/dev/null
hex=$(kv get "$agent" example/string | od -An -tx1 | tr -d ' \n')
[[ $hex == 00ff010a0a ]] || fail "binary round trip: expected 00ff010a0a (including CLI newline), got $hex"
passed=$((passed + 1))

section 'Independent namespaces and integer increments'
expect "$agent example/shared set" kv set "$agent" example/shared text
expect "$agent example/shared set" kv set "$agent" example/shared 11 --int
expect "$agent example/shared set" kv set "$agent" example/shared '{"ok":true}' --json
expect text kv get "$agent" example/shared
expect 11 kv get "$agent" example/shared --int
expect '{"ok":true}' kv get "$agent" example/shared --json
expect 1 kv inc "$agent" example/int
expect 6 kv inc "$agent" example/int 5
expect 4 kv inc "$agent" example/int -2
expect 4 kv inc "$agent" example/int 0
expect 4 kv get "$agent" example/int --int

section 'All nine JSON operations: results and stored state'
json_step() {
  local operations=$1 results=$2 document=$3
  expect "$results" kv json "$agent" example/json "$operations"
  expect "$document" kv get "$agent" example/json --json
}
json_step '[{"op":"set","key":"label","value":"batch"}]' \
  '[{"op":"set","key":"label","changed":true}]' '{"label":"batch"}'
json_step '[{"op":"inc","key":"n","value":2},{"op":"inc","key":"n","value":-1}]' \
  '[{"op":"inc","key":"n","changed":true,"value":2},{"op":"inc","key":"n","changed":true,"value":1}]' '{"label":"batch","n":1}'
json_step '[{"op":"push","key":"q","value":"b"},{"op":"push","key":"q","value":"c"},{"op":"unshift","key":"q","value":"a"}]' \
  '[{"op":"push","key":"q","changed":true},{"op":"push","key":"q","changed":true},{"op":"unshift","key":"q","changed":true}]' '{"label":"batch","n":1,"q":["a","b","c"]}'
json_step '[{"op":"shift","key":"q"},{"op":"pop","key":"q"}]' \
  '[{"op":"shift","key":"q","changed":true,"value":"a"},{"op":"pop","key":"q","changed":true,"value":"c"}]' '{"label":"batch","n":1,"q":["b"]}'
json_step '[{"op":"add_to_set","key":"q","value":"b"},{"op":"add_to_set","key":"q","value":"d"},{"op":"push","key":"q","value":"d"}]' \
  '[{"op":"add_to_set","key":"q","changed":false},{"op":"add_to_set","key":"q","changed":true},{"op":"push","key":"q","changed":true}]' '{"label":"batch","n":1,"q":["b","d","d"]}'
json_step '[{"op":"remove_from_set","key":"q","value":"d"},{"op":"remove_from_set","key":"q","value":"missing"}]' \
  '[{"op":"remove_from_set","key":"q","changed":true},{"op":"remove_from_set","key":"q","changed":false}]' '{"label":"batch","n":1,"q":["b"]}'
json_step '[{"op":"unset","key":"label"},{"op":"unset","key":"missing"},{"op":"shift","key":"q"},{"op":"pop","key":"q"}]' \
  '[{"op":"unset","key":"label","changed":true},{"op":"unset","key":"missing","changed":false},{"op":"shift","key":"q","changed":true,"value":"b"},{"op":"pop","key":"q","changed":false}]' '{"n":1,"q":[]}'
json_step '[{"op":"shift","key":"missing"},{"op":"pop","key":"missing"},{"op":"remove_from_set","key":"missing","value":1}]' \
  '[{"op":"shift","key":"missing","changed":false},{"op":"pop","key":"missing","changed":false},{"op":"remove_from_set","key":"missing","changed":false}]' '{"n":1,"q":[]}'
json_step '[{"op":"add_to_set","key":"s","value":{"b":2,"a":1}},{"op":"add_to_set","key":"s","value":{"a":1.0,"b":2e0}},{"op":"remove_from_set","key":"s","value":{"a":1e0,"b":2.0}}]' \
  '[{"op":"add_to_set","key":"s","changed":true},{"op":"add_to_set","key":"s","changed":false},{"op":"remove_from_set","key":"s","changed":true}]' '{"n":1,"q":[],"s":[]}'

section 'Refused JSON edits are atomic'
reject 'different type' kv json "$agent" example/json '[{"op":"push","key":"q","value":"must roll back"},{"op":"push","key":"n","value":3}]'
expect '{"n":1,"q":[],"s":[]}' kv get "$agent" example/json --json
reject 'no operation' kv json "$agent" example/json '[{"op":"unset","key":"n"},{"op":"unknown","key":"q"}]'
expect '{"n":1,"q":[],"s":[]}' kv get "$agent" example/json --json
reject 'takes no value' kv json "$agent" example/json '[{"op":"shift","key":"q","value":1}]'
reject 'takes a value' kv json "$agent" example/json '[{"op":"set","key":"q"}]'
reject 'inc adds an integer' kv json "$agent" example/json '[{"op":"inc","key":"n","value":0.5}]'
reject 'a JSON value is an object' kv set "$agent" example/json '[1,2]' --json
expect '{"n":1,"q":[],"s":[]}' kv get "$agent" example/json --json

printf '\nPASS: %d comparisons; all KV verbs, set modes and JSON operations.\n' "$passed"
