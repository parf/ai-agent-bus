#!/usr/bin/env bash
# Test .ai-team/deploy-bump.sh.av1 on scratch git repos under tmp/; the live repo is never written.
# Run: .ai-team/ab-w1b/deploy-bump-av1-test.sh [path-of-script-under-test]   (default .ai-team/deploy-bump.sh.av1)
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
script=${1:-$here/../deploy-bump.sh.av1}
base=/usr/local/src/ai-agent-bus/tmp/av1-bump-test; rm -rf "$base"; mkdir -p "$base"
today=$(date -u +%F); fails=0
check() { if [[ $2 == "$3" ]]; then echo "ok   $1"; else echo "FAIL $1: got '$2', want '$3'"; fails=$((fails+1)); fi; }
mkrepo() { # dir; leaves a main with agents a (client bundled), b, standalone/c; window boundary commit made
  local d=$1; rm -rf "$d"; mkdir -p "$d"; cd "$d" || exit 1
  git init -q -b main . && git config user.email t@t && git config user.name t
  mkdir -p src/internal/version src/agents/a src/agents/b src/agents/standalone/c src/agents/shared src/clients/python/agent_bus
  echo 1.0.5 >src/internal/version/VERSION; echo 1.0.0 >src/agents/VERSION
  printf '# Changes\n' >CHANGELOG.1.0.md; printf '# Agents\n' >src/agents/CHANGELOG.md
  for n in a b standalone/c; do
    printf '{\n  "description": "Agent %s",\n  "version": "1.0.0",\n  "released": "2026-01-01",\n  "command": "./x.py"\n}\n' "$n" >src/agents/$n/agent.json
    echo "print(1)" >src/agents/$n/x.py
  done
  echo 'cp "$here/../../clients/python/agent_bus/"*.py x; cp "$here/../shared/metadata.py" y' >src/agents/a/package.sh
  echo "m=1" >src/agents/shared/metadata.py; echo "n=1" >src/agents/shared/names.ts; echo "t=1" >src/agents/shared/test-network.ts
  printf 'import metadata\n' >src/agents/b/x.py; printf 'import { n } from "../../shared/names.ts";\n' >src/agents/standalone/c/x.ts
  mkdir -p src/cmd/runner src/clients/tests; echo "go=1" >src/cmd/runner/main.go; echo "t=1" >src/agents/shared/test_sb.py; echo "k=1" >src/clients/tests/kv.py
  echo "s=1" >src/agents/shared/sa.py; echo "s=1" >src/agents/shared/sb.py; echo "s=1" >src/agents/shared/sn.py; echo "c=1" >src/clients/python/agent_bus/client.py
  cat >src/smoke-map.json <<'J'
{"checks":{"agent_a":{"tags":"L","paths":["src/agents/a/","src/agents/shared/sa.py"]},"agent_b":{"tags":"L","paths":["src/agents/b/","src/agents/shared/sb.py","src/cmd/runner/main.go","src/agents/shared/test_sb.py","src/clients/tests/kv.py","src/agents/shared/test-network.ts"]},"agent_c":{"tags":"L","paths":["src/agents/standalone/c/"]}}}
J
  git add -A && git commit -q -m seed && echo 1.0.5 >src/internal/version/VERSION && git commit -q -am "Version 1.0.5 boundary" --allow-empty
  git commit -q --allow-empty -m "boundary marker"
}
touch_commit() { local f=$1; echo "x$RANDOM" >>"$f"; git add "$f"; git commit -q -m "change $f"; }
ver() { python3 -c "import json,sys;d=json.load(open(sys.argv[1]));print(d.get('version','-'),d.get('released','-'))" "$1"; }
run() { DEPLOY_REPO=$PWD "$script" --summary='s' "$@" 2>&1; }
# a version change of the bus VERSION marks the window start
window() { echo 1.0.5 >src/internal/version/VERSION; git commit -q -am "Version 1.0.5" ; git commit -q -m "VERSION touch" --allow-empty; }

# (c1) a shared file only A declares bumps A and not B
mkrepo $base/c1; echo 1.0.5 >src/internal/version/VERSION; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
touch_commit src/agents/shared/sa.py
out=$(run --agents-summary='a'); echo "$out" | tail -3
check "c1 A bumped to 1.0.1 and today (UTC)" "$(ver src/agents/a/agent.json)" "1.0.1 $today"
check "c1 B unchanged pair" "$(ver src/agents/b/agent.json)" "1.0.0 2026-01-01"
check "c1 version and released moved in one commit with the bus Version" "$(git show --stat --format=%s HEAD | grep -c 'agents/a/agent.json')" "1"
check "c1 the Version commit holds A's agent.json and not B's" "$(git show --name-only --format= HEAD | grep -c 'agents/b/agent.json')" "0"
# (c2) a shared file neither declares bumps neither
mkrepo $base/c2; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
touch_commit src/agents/shared/sn.py
out=$(run --agents-summary='n'); echo "$out" | tail -2
check "c2 A unchanged" "$(ver src/agents/a/agent.json)" "1.0.0 2026-01-01"
check "c2 B unchanged" "$(ver src/agents/b/agent.json)" "1.0.0 2026-01-01"
# (c3) the bundled client bumps exactly the agents that bundle it
mkrepo $base/c3; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
touch_commit src/clients/python/agent_bus/client.py
out=$(run); echo "$out" | tail -2
check "c3 A (bundles the client) bumped" "$(ver src/agents/a/agent.json)" "1.0.1 $today"
check "c3 B unchanged" "$(ver src/agents/b/agent.json)" "1.0.0 2026-01-01"
check "c3 standalone/c unchanged" "$(ver src/agents/standalone/c/agent.json)" "1.0.0 2026-01-01"
# (m2) a check that also lists a Go path, a client test or a shared test never bumps the agent for them
mkrepo $base/m2; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
check "m2 the plan right after seeding prints no bump" "$(DEPLOY_REPO=$PWD "$script" --plan | wc -l)" "0"
touch_commit src/cmd/runner/main.go; touch_commit src/clients/tests/kv.py; touch_commit src/agents/shared/test_sb.py
check "m2 --plan prints no bump for those paths" "$(DEPLOY_REPO=$PWD "$script" --plan | wc -l)" "0"
out=$(run --agents-summary='n'); echo "$out" | tail -1
check "m2 B unchanged" "$(ver src/agents/b/agent.json)" "1.0.0 2026-01-01"
# (i1) a shared file bumps exactly the agents that ship it: package.sh copies and Python/TS imports, beyond the declared paths
mkrepo $base/i1; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
touch_commit src/agents/shared/metadata.py
out=$(run --agents-summary='m'); echo "$out" | tail -1
check "i1 metadata.py: A (package.sh copy) bumped" "$(ver src/agents/a/agent.json)" "1.0.1 $today"
check "i1 metadata.py: B (python import) bumped" "$(ver src/agents/b/agent.json)" "1.0.1 $today"
check "i1 metadata.py: standalone/c (imports names.ts only) unchanged" "$(ver src/agents/standalone/c/agent.json)" "1.0.0 2026-01-01"
touch_commit src/agents/shared/names.ts
out=$(run --agents-summary='n'); echo "$out" | tail -1
check "i1 names.ts: standalone/c (TS import) bumped" "$(ver src/agents/standalone/c/agent.json)" "1.0.1 $today"
check "i1 names.ts: A not bumped again" "$(ver src/agents/a/agent.json)" "1.0.1 $today"
# (t1) test-network.ts is not a test file by pattern, so B (its check declares it) bumps; test_sb.py never does
mkrepo $base/t1; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
touch_commit src/agents/shared/test_sb.py
check "t1 test_sb.py (a test pattern) bumps nobody" "$(DEPLOY_REPO=$PWD "$script" --plan | wc -l)" "0"
touch_commit src/agents/shared/test-network.ts
check "t1 test-network.ts (declared, not a test pattern) bumps B" "$(DEPLOY_REPO=$PWD "$script" --plan | grep -c '^bump.b\b')" "1"
# (r1) a commit that changes released without version refuses, except the first stamp that adds it
mkrepo $base/r1; python3 - <<'P'
import json
p='src/agents/b/agent.json'; d=json.load(open(p)); d['released']='2026-02-02'; open(p,'w').write(json.dumps(d,indent=2)+'\n')
P
git commit -qam "released only"; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"; touch_commit src/agents/b/x.py
out=$(run --agents-summary='n'); check "r1 refuses a released-only commit, naming b" "$(echo "$out" | grep -c 'refused: agent b commit .* changed released without version')" "1"
mkrepo $base/r2; python3 - <<'P'
import json
p='src/agents/b/agent.json'; d=json.load(open(p)); del d['released']; open(p,'w').write(json.dumps(d,indent=2)+'\n')
P
git commit -qam "strip released"; python3 - <<'P'
import json
p='src/agents/b/agent.json'; d=json.load(open(p)); d['released']='2026-03-03'; open(p,'w').write(json.dumps(d,indent=2)+'\n')
P
git commit -qam "first stamp"; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"; touch_commit src/agents/b/x.py
out=$(run --agents-summary='n' --seed); check "r2 the first stamp is allowed" "$(echo "$out" | grep -c 'refused')" "0"
# (k1) after the seed, ADDING an agent's CHANGELOG.md, editing its help.golden.json and a test file leaves the plan empty
mkrepo $base/k1; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
for n in a b standalone/c; do echo "# $n" >src/agents/$n/CHANGELOG.md; echo '{}' >src/agents/$n/help.golden.json; echo "t" >src/agents/$n/x_test.py; done
git add -A; git commit -qm "changelogs, goldens and tests"; mkdir -p src/agents/b/docs; echo c >src/agents/b/docs/CHANGELOG.md; git add -A; git commit -qm "nested changelog"
echo x >>src/agents/a/help.golden.json; git commit -qam "golden edit"
check "k1 the plan is empty" "$(DEPLOY_REPO=$PWD "$script" --plan | wc -l)" "0"
touch_commit src/agents/b/x.py
check "k1 a real change still bumps B" "$(DEPLOY_REPO=$PWD "$script" --plan | grep -c '^bump.b\b')" "1"
# (p1) a shipped change landed before the previous Version commit was deployed with it: nothing bumps again, from the agent's own older bump commit
mkrepo $base/p1; touch_commit src/agents/shared/sa.py; touch_commit src/agents/b/x.py; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
check "p1 the plan since that deploy is empty" "$(DEPLOY_REPO=$PWD "$script" --plan | wc -l)" "0"
touch_commit src/agents/b/x.py
check "p1 a change after the deploy bumps B only" "$(DEPLOY_REPO=$PWD "$script" --plan | tr '\n' ' ' | sed 's/\t/ /g')" "bump b 1.0.0 1.0.1 "
# (x1) a previous deploy window that changed A's shipped files without A's version/released changing is refused by name, never silent
mkrepo $base/x1; echo "1.0.4 " >src/internal/version/VERSION; git commit -qam "Version 1.0.4"
touch_commit src/agents/shared/sa.py; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
check "x1 --plan refuses the missed agent, naming a and the range" "$(DEPLOY_REPO=$PWD "$script" --plan | grep -cE '^refuse.a.missed bump: .*[0-9a-f]{8}\.\.[0-9a-f]{8}')" "1"
touch_commit src/agents/b/x.py
out=$(run --agents-summary='m'); check "x1 the deploy itself refuses, changing nothing" "$(echo "$out" | grep -c 'refused: agent a missed bump')$(git status --porcelain | wc -l)" "10"
out=$(run --agents-summary='m' --accept-missed); echo "$out" | tail -2
check "x1 --accept-missed bumps A now" "$(ver src/agents/a/agent.json)" "1.0.1 $today"
# (x2) the seed window is not flagged: the same window changes the agent's version and released
mkrepo $base/x2; echo "1.0.4 " >src/internal/version/VERSION; git commit -qam "Version 1.0.4"
python3 - <<'P'
import json
p='src/agents/a/agent.json'; d=json.load(open(p)); d['version']='1.0.1'; d['released']='2026-02-02'; open(p,'w').write(json.dumps(d,indent=2)+'\n')
P
git commit -qam "seed-like bump of A"; touch_commit src/agents/shared/sa.py; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
check "x2 --plan prints nothing: the window holds A's version change" "$(DEPLOY_REPO=$PWD "$script" --plan | wc -l)" "0"
# (h1) deploy commits are recognised by subject only: a hand fix whose BODY says "Version 1.0.2" is not a baseline
mkrepo $base/h1; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
touch_commit src/agents/shared/sa.py
echo "hand" >>src/internal/version/VERSION; git commit -qam "Hand fix of VERSION" -m "Version 1.0.2"
check "h1 the change before the hand fix still bumps A" "$(DEPLOY_REPO=$PWD "$script" --plan | grep -c '^bump.a\b')" "1"
# (u1) --help and -h print usage naming every flag, exit 0 and change nothing
mkrepo $base/u1; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"; touch_commit src/agents/shared/sa.py
head=$(git rev-parse HEAD); for h in --help -h; do
  out=$(DEPLOY_REPO=$PWD "$script" $h); rc=$?
  check "u1 $h exits 0" "$rc" "0"
done
out=$(DEPLOY_REPO=$PWD "$script" --help); for flag in --summary --agents-summary --seed --accept-missed --plan --ship --help -h; do [[ $(echo "$out" | grep -c -- "$flag") -ge 1 ]] && echo "ok   u1 usage names $flag" || { echo "FAIL u1 usage names $flag"; fails=$((fails+1)); }; done
check "u1 help changed nothing" "$(git rev-parse HEAD)$(git status --porcelain | wc -l)" "${head}0"
# (d1) a version change without released in the same commit refuses by agent name
mkrepo $base/d1; python3 - <<'P'
import json
p='src/agents/b/agent.json'; d=json.load(open(p)); d['version']='1.1.0'; open(p,'w').write(json.dumps(d,indent=2)+'\n')
P
git commit -qam "hand minor bump"; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"; touch_commit src/agents/b/x.py
out=$(run --agents-summary='n'); check "d1 refuses the drifted pair, naming b" "$(echo "$out" | grep -c 'refused: agent b commit .* changed version without released')" "1"
check "d1 and changes nothing" "$(git status --porcelain | wc -l)" "0"
# (b/own folder) a change in B's own folder bumps B only; a CHANGELOG.md edit alone bumps nobody
mkrepo $base/b1; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"
touch_commit src/agents/standalone/c/x.py; touch_commit src/agents/b/CHANGELOG.md
out=$(run --agents-summary='c'); echo "$out" | tail -2
check "b1 standalone/c bumped" "$(ver src/agents/standalone/c/agent.json)" "1.0.1 $today"
check "b1 B (CHANGELOG.md only) unchanged" "$(ver src/agents/b/agent.json)" "1.0.0 2026-01-01"
# the next deploy bumps from the agent's own last bump commit, not from the window: a second deploy with nothing new leaves it
touch_commit src/agents/shared/sb.py
out=$(run --agents-summary='b'); echo "$out" | tail -2
check "b1 second deploy: B bumped once" "$(ver src/agents/b/agent.json)" "1.0.1 $today"
check "b1 second deploy: standalone/c not bumped again" "$(ver src/agents/standalone/c/agent.json)" "1.0.1 $today"
# (seed) an agent lacking the pair refuses without --seed, is seeded with it; one with a version only gets released
mkrepo $base/s1; python3 - <<'P'
import json
p='src/agents/b/agent.json'; d=json.load(open(p)); del d['version']; del d['released']; open(p,'w').write(json.dumps(d,indent=2)+'\n')
p='src/agents/a/agent.json'; d=json.load(open(p)); del d['released']; d['version']='0.4.2'; open(p,'w').write(json.dumps(d,indent=2)+'\n')
P
git commit -qam "strip"; echo "1.0.5 " >src/internal/version/VERSION; git commit -qam "Version 1.0.5"; touch_commit src/agents/shared/sn.py
out=$(run --agents-summary='n'); check "s1 refuses without --seed, naming an agent and its missing field" "$(echo "$out" | grep -cE 'refused: agent (a lacks released|b lacks version)')" "1"
check "s1 the refusal changed nothing" "$(git status --porcelain | wc -l)" "0"
out=$(run --agents-summary='n' --seed); echo "$out" | tail -3
check "s1 seeded B" "$(ver src/agents/b/agent.json)" "1.0.0 $today"
check "s1 A keeps 0.4.2, gains released" "$(ver src/agents/a/agent.json)" "0.4.2 $today"
check "s1 standalone/c complete and unchanged" "$(ver src/agents/standalone/c/agent.json)" "1.0.0 2026-01-01"
cd "$base/.." && rm -rf "$base"
((fails == 0)) && echo "ALL OK" || { echo "$fails FAILED"; exit 1; }
