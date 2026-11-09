#!/usr/bin/env bash
# J111: gate.sh end to end over a throwaway repository, with stub smoke, docs tests
# and agent-bus, and the real selector, subsystem table, smoke map, check-links and
# check-markers copied from TREE (default the main checkout).
# Run: [TREE=<checkout>] bash gate.test.sh [dir holding gate.sh and gate-lock.sh].
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
src=${1:-$here}
TREE=${TREE:-/usr/local/src/ai-agent-bus}
# The agent-pack-only rule comes with T-39's selector (main once it lands).
AGENTS_TREE=${AGENTS_TREE:-/usr/local/src/ai-agent-bus/tmp/ab-w1a-agent-pack}
gate=$src/gate.sh; [[ -f $gate ]] || gate=$src/gate.sh.new
d=$(mktemp -d "/usr/local/src/ai-agent-bus/tmp/gate-test.XXXXXX")
trap 'rm -rf "$d"' EXIT
mkdir -p "$d/bin" "$d/gate"
cp "$gate" "$d/gate/gate.sh"; cp "$src/gate-lock.sh" "$d/gate/gate-lock.sh"
cat >"$d/bin/agent-bus" <<'STUB'
#!/usr/bin/env bash
echo "$*" >>"$CALLS"
case $1 in
  lock)
    n=$(grep -c '^lock ' "$CALLS"); answer=$(sed -n "${n}p" "$PLAN"); [[ -n $answer ]] || answer=$(tail -n 1 "$PLAN")
    case $answer in
      grant) echo '{"holder":"parf"}' ;;
      self) echo '{"error":"you already hold it; use extend","kind":"busy"}' >&2; exit 1 ;;
      acl) echo '{"error":"not a maintainer","kind":"acl"}' >&2; exit 1 ;;
    esac ;;
  holders) echo 'smoke-gate@parf.us slow parf 11m left' ;;
  release) ;;
  *) exit 3 ;;
esac
STUB
printf '#!/usr/bin/env bash\necho smoke >>"$CALLS"\nexit 0\n' >"$d/bin/smoke-stub"
printf '#!/usr/bin/env bash\necho docs-check >>"$CALLS"\nexit "${DOCS_RC:-0}"\n' >"$d/bin/docs-stub"
printf '#!/usr/bin/env bash\necho docs-tests >>"$CALLS"\n' >"$d/bin/tests-stub"
printf '#!/usr/bin/env bash\necho "go $1" >>"$CALLS"\n' >"$d/bin/go"
chmod +x "$d/bin/"*
export PATH="$d/bin:$PATH" GATE_SMOKE="$d/bin/smoke-stub" GATE_DOCS_CHECK="$d/bin/docs-stub" GATE_DOCS_TESTS="$d/bin/tests-stub" GATE_LOCK_PAUSE=1
# Never reach the real bus: the gate would poll, and could take, the live smoke-gate lock.
[[ $(command -v agent-bus) == "$d/bin/agent-bus" ]] || { echo "FAIL the agent-bus stub is not first on PATH"; exit 1; }
fail=0
bad() { echo "FAIL $1"; fail=1; }
case_repo() { # label, then the branch's changes as path=content pairs; SELECTOR= replaces smoke-select.py on main
  local label=$1; shift
  R=$d/$label; mkdir -p "$R/src/internal/version" "$R/docs"
  git -C "$R" init -q -b main; git -C "$R" config user.email t@t; git -C "$R" config user.name t
  echo 1.0.40 >"$R/src/internal/version/VERSION"; printf '# c\n\n## 1.0.40 — x\n\nold\n' >"$R/CHANGELOG.1.0.md"
  echo a >"$R/docs/a.md"; echo 'package x' >"$R/src/x.go"; mkdir -p "$R/Plans" "$R/src/agents"
  local s; for s in Plans/subsystems.md src/smoke-map.json src/smoke-select.py src/check-links.py src/check-markers.py; do cp "$TREE/$s" "$R/$s"; done
  [[ -z ${SELECTOR:-} ]] || printf '%s\n' "$SELECTOR" >"$R/src/smoke-select.py"
  git -C "$R" add -A; git -C "$R" commit -qm base
  git -C "$R" checkout -qb b
  local kv; for kv in "$@"; do mkdir -p "$R/$(dirname "${kv%%=*}")"; echo "${kv#*=}" >"$R/${kv%%=*}"; done
  git -C "$R" add -A; git -C "$R" commit -qm change; git -C "$R" checkout -q main
  CALLS=$d/$label.calls PLAN=$d/$label.plan; : >"$CALLS"; echo grant >"$PLAN"; export CALLS PLAN
}
gate() { receipt=$(GATE_REPO=$R bash "$d/gate/gate.sh" <<<"{\"from\":\"#t\",\"body\":\"${BODY:-b T-1}\"}"); }
landed() { [[ $(git -C "$R" log -1 --format=%s main) == change ]]; }

case_repo docs Plans/p.md=p Plans/x/new.md=n CLAUDE.md=c
gate
[[ $receipt == "gate landed:"*"docs check"* ]] || bad "docs: $receipt"
grep -qx docs-check "$CALLS" || bad "docs: the docs check did not run"
grep -qx docs-tests "$CALLS" || bad "docs: the docs tests did not run"
grep -qx smoke "$CALLS" && bad "docs: the normal gate ran for a docs-only branch"
grep -q '^lock ' "$CALLS" && bad "docs: a docs-only branch took the lock"
landed || bad "docs: main did not move"

# A docs/ page is never docs-only, owned or not; the subsystem table is the selector's
# input, and README.md a user page the selector gives the agent guide.
for c in "code src/x.go=y" "srcmd src/agents/README.md=r" "page docs/a.md=b" "guide docs/00-overview.md=g" \
  "api docs/09-daemon-api.md=a" "table Plans/subsystems.md=t" "readme README.md=r" "license LICENSE.md=l" "deep x/notes.md=n"; do
  read -r label kv <<<"$c"
  case_repo "$label" Plans/p.md=p $kv
  gate
  [[ $receipt == "gate landed:"*"normal"* ]] || bad "$label: $receipt"
  grep -qx smoke "$CALLS" || bad "$label: $kv did not force the normal gate"
  grep -qx docs-check "$CALLS" && bad "$label: the docs check stood in for normal"
  grep -q '^release ' "$CALLS" || bad "$label: the gate did not release its own lock"
done

case_repo red Plans/p.md=p
DOCS_RC=1 gate
[[ $receipt == "gate red:"*"docs-only check rc=1"* ]] || bad "red: $receipt"
landed && bad "red: a red docs check landed"
grep -qx docs-tests "$CALLS" && bad "red: the docs tests ran after a red docs check"

# The selector fails closed: an error, or an answer without the no-checks line, runs normal.
# selcrash prints the docs-only line and then fails: the exit status alone must force normal.
for c in "selerr|import sys; sys.exit(3)" "selodd|print('SELECTION LANDING RUN')" \
  "selcrash|print('  fallback: no executable checks for changed paths: all fast'); raise SystemExit(1)"; do
  SELECTOR=${c#*|} case_repo "${c%%|*}" Plans/p.md=p
  gate
  [[ $receipt == "gate landed:"*"normal"* ]] || bad "${c%%|*}: $receipt"
  grep -qx smoke "$CALLS" || bad "${c%%|*}: a selector that said nothing usable did not force normal"
  grep -qx docs-check "$CALLS" && bad "${c%%|*}: the docs check stood in for normal"
done

# The shape rule stands on its own: a selector that always answers docs-only still
# sends a docs/ page, LICENSE.md, Markdown under src/ or elsewhere to normal.
n=0
for kv in docs/a.md=b LICENSE.md=l src/agents/README.md=r x/notes.md=n; do
  SELECTOR="print('  fallback: no executable checks for changed paths: all fast')" case_repo selyes$((++n)) Plans/p.md=p "$kv"
  gate
  [[ $receipt == "gate landed:"*"normal"* ]] || bad "selyes $kv: $receipt"
  grep -qx docs-check "$CALLS" && bad "selyes $kv: the docs check stood in for normal"
done
SELECTOR="print('  fallback: no executable checks for changed paths: all fast')" case_repo selyesok Plans/p.md=p
gate
[[ $receipt == "gate landed:"*"docs check"* ]] || bad "selyesok: the stub selector did not reach the docs path: $receipt"

# The real docs check (check-links and check-markers): a plan line that is a conflict
# marker (J29) is refused; the same branch without it lands.
case_repo marker CLAUDE.md=c 'Plans/p.md=||||||| parent of 1234567 (x)'
GATE_DOCS_CHECK= gate
[[ $receipt == "gate red:"*"docs-only check rc=1"* ]] || bad "marker: $receipt"
landed && bad "marker: a conflict marker landed"
grep -rqs 'Plans/p.md' "$R/tmp/gate-logs" || bad "marker: check-markers did not name the file"
grep -qx docs-tests "$CALLS" && bad "marker: the docs tests ran after a red docs check"
case_repo clean CLAUDE.md=c 'Plans/p.md=a plain line'
GATE_DOCS_CHECK= gate
[[ $receipt == "gate landed:"*"docs check"* ]] || bad "clean: $receipt"
grep -qx docs-tests "$CALLS" || bad "clean: the docs tests did not run"
grep -q '^lock ' "$CALLS" && bad "clean: a docs-only branch took the lock"

case_repo wait src/x.go='package y'
printf 'self\nself\ngrant\n' >"$PLAN"
gate
[[ $receipt == "gate landed:"* ]] || bad "wait: $receipt"
[[ $(grep -c '^lock ' "$CALLS") == 3 ]] || bad "wait: $(grep -c '^lock ' "$CALLS") lock calls, want 3"

case_repo held src/x.go='package y'
echo self >"$PLAN"
GATE_LOCK_BOUND=2 gate
[[ $receipt == "gate failed:"*"not granted in 2s: held by smoke-gate@parf.us slow parf 11m left (another parf session)"* ]] || bad "held: $receipt"
grep -qx smoke "$CALLS" && bad "held: the normal gate ran without the lock"
grep -q '^release ' "$CALLS" && bad "held: the gate released a hold it never took"

case_repo acl src/x.go='package y'
echo acl >"$PLAN"
gate
[[ $receipt == "gate failed:"*"lock refused:"*'"kind":"acl"'* ]] || bad "acl: $receipt"
[[ $(grep -c '^lock ' "$CALLS") == 1 ]] || bad "acl: a non-busy refusal was retried"

# T-44: a landing never bumps (TD6). "bump" and a "-- line" without stable are refused
# before the rebase, the lock and the run; a landing leaves VERSION and the changelog alone.
for c in "bump|b T-1 bump|bump is retired: #ab-ceo bumps at deploy" "line|b T-1 -- a changelog line|'-- line' goes only with stable: the changelog is written at deploy"; do
  IFS='|' read -r label body want <<<"$c"
  case_repo "$label" src/x.go='package y'
  BODY=$body gate
  [[ $receipt == "gate refused: T-1 b — $want" ]] || bad "$label: $receipt"
  landed && bad "$label: a refused body landed"
  [[ -s $CALLS ]] && bad "$label: the gate ran something: $(tr '\n' ' ' <"$CALLS")"
done
case_repo plain src/x.go='package y'
gate
[[ $receipt == "gate landed:"*", no bump, normal"* ]] || bad "plain: $receipt"
[[ $(git -C "$R" show main:src/internal/version/VERSION) == 1.0.40 ]] || bad "plain: VERSION moved"
git -C "$R" diff --quiet main~1 main -- src/internal/version/VERSION CHANGELOG.1.0.md || bad "plain: the landing touched VERSION or the changelog"
case_repo stable src/internal/version/VERSION=1.1.0 'CHANGELOG.1.1.md=## 1.1.0 — x'
BODY='b T-1 stable -- reviewed tree and evidence' gate
[[ $receipt == "gate landed:"*"STABLE 1.1.0"* ]] || bad "stable: $receipt"
[[ $(git -C "$R" log -1 --format=%s main) == "STABLE - passed all tests and reviews" ]] || bad "stable: subject $(git -C "$R" log -1 --format=%s main)"

# J117: an agent-pack-only branch lands on the normal gate, whose selection runs only the
# owning checks and the pack contract; the gate skips the Go warm-up and names those checks.
for c in "one|agent_pack agent_slack|src/agents/slack/slack.ts=x" \
  "two|agent_pack agent_slack agent_telegram|src/agents/slack/slack.ts=x src/agents/telegram/telegram.ts=x" \
  "shared|agent_discord agent_pack agent_slack agent_telegram|src/agents/shared/router.ts=x"; do
  IFS='|' read -r label want kvs <<<"$c"
  TREE=$AGENTS_TREE case_repo "agents-$label" $kvs
  gate
  [[ $receipt == "gate landed:"*", no bump, agents only ($want) "* ]] || bad "agents-$label: $receipt"
  grep -qx smoke "$CALLS" || bad "agents-$label: the normal gate did not run"
  grep -q '^go ' "$CALLS" && bad "agents-$label: the Go warm-up ran for an agent-only branch"
  grep -q '^lock ' "$CALLS" || bad "agents-$label: the normal gate ran without the lock"
done
TREE=$AGENTS_TREE case_repo agents-setup src/agents/slack/slack.ts=x src/cmd/agent-bus-setup/package_install.go=x
gate
[[ $receipt == "gate landed:"*", no bump, normal "* ]] || bad "agents-setup: $receipt"
grep -q '^go test' "$CALLS" || bad "agents-setup: a branch beyond the pack skipped the Go warm-up"
# The answer is the selector's alone: an error, even after the line, or another answer is plain normal.
for c in "aerr|print('  agent pack only: checks that own no changed path are skipped'); raise SystemExit(1)" \
  "aodd|print('  agent pack only')"; do
  SELECTOR=${c#*|} TREE=$AGENTS_TREE case_repo "${c%%|*}" src/agents/slack/slack.ts=x
  gate
  [[ $receipt == "gate landed:"*", no bump, normal "* ]] || bad "${c%%|*}: $receipt"
  grep -q '^go test' "$CALLS" || bad "${c%%|*}: the Go warm-up was skipped without the selector's answer"
done

((fail)) && exit 1
echo "gate: docs-only lands on the docs check alone, code runs normal, the lock waits only on busy: green"
