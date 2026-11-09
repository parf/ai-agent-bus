#!/usr/bin/env bash
# J120 stub test for gate.sh.new: a red-then-green normal gate is never silent.
# Each case builds a small repository with the real src/smoke-quarantine.py and
# selector, and a stub smoke whose runs are scripted; no lock, no real smoke.
# Usage: gate-rerun.test.sh [gate script]   (default: gate.sh.new beside this file)
set -uo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
GATE=${1:-$HERE/gate.sh.new}
SRC=${J120_SRC:-/usr/local/src/ai-agent-bus}
SCRATCH=$(mktemp -d /usr/local/src/ai-agent-bus/tmp/ab-w1a-gate-rerun.XXXXXX)
trap 'rm -rf "$SCRATCH"' EXIT
pass=0 fail=0
ok() { if eval "$2"; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1"; fail=$((fail+1)); fi; }

# The first run's output: a Go test in a ⚠️ package and one in the CLI, a named
# title in a web shard, and a fast failure that names nothing.
FIRST=$'[go_unit]   FAIL go race: exit 1\n[go_unit]     --- FAIL: TestCoreRace (0.10s)\n[go_unit]     FAIL\tgithub.com/parf/ai-agent-bus/internal/core\t1.0s\n[go_unit]     --- FAIL: TestCLIRestart (5.00s)\n[go_unit]     FAIL\tgithub.com/parf/ai-agent-bus/cmd/x\t6.0s\n[web_ts]   FAIL a web title: exit 1\n[agent_slack] FAIL fast command agent_slack (nonzero exit)\npassed 10, failed 4'
RISKY=$'[go_unit]     --- FAIL: TestCoreRace (0.10s)\n[go_unit]     FAIL\tgithub.com/parf/ai-agent-bus/internal/core\t1.0s\npassed 10, failed 1'

repo() { # a fresh repository with main and branch feat; the stub smoke plays $@ in turn
  R=$SCRATCH/$1; shift
  mkdir -p "$R"/src/internal/core "$R"/src/cmd/x "$R"/Plans/issues/2026-11 "$R"/src/agents
  cp "$SRC"/src/smoke-quarantine.py "$SRC"/src/smoke-select.py "$SRC"/src/go.mod "$R"/src/
  cp "$SRC"/Plans/subsystems.md "$R"/Plans/
  printf '%s\n' 'ok_exit "a web title" "$rc"' >"$R"/src/smoke.sh
  : >"$R"/src/agents/smoke.sh
  printf 'package core\nimport "testing"\nfunc TestCoreRace(t *testing.T) {}\n' >"$R"/src/internal/core/core_test.go
  printf 'package x\nimport "testing"\nfunc TestCLIRestart(t *testing.T) {}\n' >"$R"/src/cmd/x/x_test.go
  printf '# Issues — Nov 1, 2026\n\n## Old\n\n- ⚪ J130 an open item.\n' >"$R"/Plans/issues/2026-11/2026-11-01.md
  cat >"$R"/src/smoke-map.json <<'MAP'
{
  "reverse": {},
  "checks": {
    "core_1": {
      "tags": "ABDEH"
    },
    "go_unit": {
      "tags": "ABCDEFGHM"
    },
    "web_ts": {
      "tags": "I"
    },
    "agent_slack": {
      "tags": "L"
    }
  },
  "fast": {}
}
MAP
  git -C "$R" init -q -b main && git -C "$R" add -A && git -C "$R" -c user.name=t -c user.email=t@t commit -q -m base
  git -C "$R" checkout -q -b feat && echo change >"$R"/feature.txt && git -C "$R" add feature.txt \
    && git -C "$R" -c user.name=t -c user.email=t@t commit -q -m feature && git -C "$R" checkout -q main
  : >"$R"/.calls
  local i=0
  for run in "$@"; do i=$((i+1)); printf '%s' "$run" >"$R/.run$i"; done
  cat >"$R"/stub-smoke.sh <<'STUB'
n=$(( $(wc -l <"$STUB_R/.calls") + 1 )); echo run >>"$STUB_R/.calls"
f=$STUB_R/.run$n
if [ -s "$f" ]; then cat "$f"; echo; exit 1; fi
echo 'passed 14, failed 0'
STUB
}
gate() { # body: the receipt on stdout
  (cd "$R" && STUB_R=$R GATE_REPO=$R GATE_NOLOCK=1 GATE_SMOKE="bash $R/stub-smoke.sh" \
    GIT_AUTHOR_NAME=gate GIT_AUTHOR_EMAIL=g@g GIT_COMMITTER_NAME=gate GIT_COMMITTER_EMAIL=g@g \
    bash "$GATE" <<<"{\"from\":\"#t\",\"body\":\"$1\"}")
}
calls() { wc -l <"$R/.calls"; }
qmap() { git -C "$R" show main:src/smoke-map.json | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin).get("quarantine")))'; }

echo "== red then green: names every shard, quarantines the safe tests, files every one, lands"
repo a "$FIRST"
out=$(gate "feat T-1"); echo "$out" | sed 's/^/     | /'
ok "the smoke ran twice" '[ "$(calls)" = 2 ]'
ok "the receipt starts with RERUN-GREEN naming each shard" '[[ $(head -n 1 <<<"$out") == "RERUN-GREEN go_unit, web_ts, agent_slack: quarantined 2 with a J-item" ]]'
ok "the ⚠️ test is OPEN as NEEDS LEAD" 'grep -qx "OPEN J131: go_unit / TestCoreRace not quarantined (⚠️ NEEDS LEAD)" <<<"$out"'
ok "the unnamed failure is OPEN as UNNAMED" 'grep -qx "OPEN J134: agent_slack not quarantined (UNNAMED)" <<<"$out"'
ok "exactly two OPEN lines" '[ "$(grep -c "^OPEN " <<<"$out")" = 2 ]'
ok "then the landed text" '[[ $(tail -n 1 <<<"$out") == "gate landed: T-1 feat — main "* ]] && [[ $out != *"green on rerun"* ]]'
ok "main holds the quarantine of the two safe tests only" '[ "$(qmap)" = "{\"go_unit\": {\"TestCLIRestart\": \"J132\"}, \"web_ts\": {\"a web title\": \"J133\"}}" ]'
ok "main is the feature plus one quarantine commit" '[ "$(git -C "$R" log --format=%s main | head -n 2 | paste -sd"|")" = "RERUN-GREEN go_unit, web_ts, agent_slack: quarantined 2 with a J-item|feature" ]'
ok "four J-items filed, J131 to J134" '[ "$(git -C "$R" show main:Plans/issues/$(date +%Y-%m)/$(date +%F).md | grep -oE "^- ⚪ J[0-9]+" | paste -sd" ")" = "- ⚪ J131 - ⚪ J132 - ⚪ J133 - ⚪ J134" ]'
ok "the first run is kept beside the log and cited" 'ls "$R"/tmp/gate-logs/*.run1 >/dev/null 2>&1 && git -C "$R" show main:Plans/issues/$(date +%Y-%m)/$(date +%F).md | grep -q "\.run1"'

echo "== green first: no RERUN-GREEN, nothing filed"
repo b
out=$(gate "feat T-2"); echo "$out" | sed 's/^/     | /'
ok "the smoke ran once" '[ "$(calls)" = 1 ]'
ok "the receipt is the landed text alone" '[ "$(wc -l <<<"$out")" = 1 ] && [[ $out == "gate landed: T-2 feat — main "* ]]'
ok "no RERUN-GREEN or OPEN anywhere" '! grep -qE "RERUN-GREEN|^OPEN " <<<"$out"'
ok "main is the feature alone, map without quarantine" '[ "$(git -C "$R" log -1 --format=%s main)" = feature ] && [ "$(qmap)" = null ]'
ok "no first-run file kept" '! ls "$R"/tmp/gate-logs/*.run1 >/dev/null 2>&1'

echo "== red twice: stays red, nothing lands or is filed"
repo c "$FIRST" "$FIRST"
out=$(gate "feat T-3"); echo "$out" | sed 's/^/     | /'
ok "the smoke ran twice" '[ "$(calls)" = 2 ]'
ok "the receipt is red, twice" '[[ $out == "gate red: T-3 feat — normal gate rc=1 twice"* ]] && ! grep -q RERUN-GREEN <<<"$out"'
ok "main did not move" '[ "$(git -C "$R" log -1 --format=%s main)" = base ]'

echo "== only a ⚠️ test: lands, files the J, skips nothing, says NEEDS LEAD"
repo d "$RISKY"
out=$(gate "feat T-4"); echo "$out" | sed 's/^/     | /'
ok "RERUN-GREEN says nothing quarantined" '[[ $(head -n 1 <<<"$out") == "RERUN-GREEN go_unit: nothing quarantined" ]]'
ok "OPEN NEEDS LEAD" 'grep -qx "OPEN J131: go_unit / TestCoreRace not quarantined (⚠️ NEEDS LEAD)" <<<"$out"'
ok "it landed" '[[ $(tail -n 1 <<<"$out") == "gate landed: T-4 feat"* ]]'
ok "no quarantine on main" '[ "$(qmap)" = null ]'
ok "the J is filed on main" 'git -C "$R" show main:Plans/issues/$(date +%Y-%m)/$(date +%F).md | grep -q "^- ⚪ J131 .*TestCoreRace"'

echo "== stable is refused while a quarantine stands"
repo e "$FIRST"
gate "feat T-5" >/dev/null
git -C "$R" checkout -q -b rel main && echo r >"$R"/rel.txt && git -C "$R" add rel.txt && git -C "$R" -c user.name=t -c user.email=t@t commit -q -m rel && git -C "$R" checkout -q main
out=$(gate "rel stable -- tree x"); echo "$out" | sed 's/^/     | /'
ok "refused, naming the quarantined tests" '[[ $out == "gate refused: rel rel — stable: src/smoke-map.json still quarantines go_unit/TestCLIRestart web_ts/a web title "* ]]'
ok "the smoke did not run again" '[ "$(calls)" = 2 ]'

echo; echo "passed $pass, failed $fail"
[ "$fail" = 0 ]
