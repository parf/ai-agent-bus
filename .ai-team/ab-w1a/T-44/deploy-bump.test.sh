#!/usr/bin/env bash
# T-44: deploy-bump.sh over throwaway repositories. Run: bash deploy-bump.test.sh [script].
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
script=${1:-$here/deploy-bump.sh}
d=$(mktemp -d "/usr/local/src/ai-agent-bus/tmp/deploy-bump-test.XXXXXX")
trap 'rm -rf "$d"' EXIT
fail=0
bad() { echo "FAIL $1"; fail=1; }
V=src/internal/version/VERSION
packfiles() {
  mkdir -p "$R/src/agents"; echo 1.0.3 >"$R/src/agents/VERSION"
  printf '# Agent pack changelog\n\nintro\n\n## 1.0.3 — 2026-11-05\n\nold\n' >"$R/src/agents/CHANGELOG.md"
}
mkrepo() { # label [version]: VERSION commit, then two commits in the window
  R=$d/$1; mkdir -p "$R/src/internal/version"
  git -C "$R" init -q -b main; git -C "$R" config user.email t@t; git -C "$R" config user.name t
  echo "${2:-1.0.47}" >"$R/$V"
  printf '# Changelog 1.0\n\nintro\n\n## 1.0.47 — 2026-11-05\n\nold\n' >"$R/CHANGELOG.1.0.md"
  ((${PACK:-0})) && packfiles
  git -C "$R" add -A; git -C "$R" commit -qm "Version 1.0.47"
  echo a >"$R/a"; git -C "$R" add a; git -C "$R" commit -qm "First change lands"
  echo b >"$R/b"; git -C "$R" add b; git -C "$R" commit -qm "Second change lands"
}
run() { out=$(DEPLOY_REPO=$R bash "$script" "$@" 2>&1); rc=$?; }
untouched() { # label, head before: a refusal changes nothing and leaves nothing behind
  [[ $(git -C "$R" rev-parse HEAD) == "$2" ]] || bad "$1: HEAD moved"
  [[ -z $(git -C "$R" status --porcelain --untracked-files=all | grep -v "${3:-^$}") ]] || bad "$1: left changes: $(git -C "$R" status --porcelain --untracked-files=all | tr '\n' ' ')"
}

mkrepo ok
echo staged >"$R/other"; git -C "$R" add other
run --summary='Two changes land together'
[[ $rc == 0 && $out == "deploy-bump: 1.0.48 committed "*" for 2 commits since "* ]] || bad "ok: rc=$rc $out"
[[ $(cat "$R/$V") == 1.0.48 ]] || bad "ok: VERSION $(cat "$R/$V")"
[[ $(git -C "$R" log -1 --format=%s) == "Version 1.0.48" ]] || bad "ok: subject $(git -C "$R" log -1 --format=%s)"
[[ $(git -C "$R" show --name-only --format= HEAD | sort | tr '\n' ' ') == "CHANGELOG.1.0.md $V " ]] || bad "ok: the commit holds $(git -C "$R" show --name-only --format= HEAD | tr '\n' ' ')"
[[ $(git -C "$R" diff --cached --name-only) == other ]] || bad "ok: another staged file was swept in or unstaged"
want=$(printf '# Changelog 1.0\n\nintro\n\n## 1.0.48 — %s\n\nTwo changes land together\n\n## 1.0.47 — 2026-11-05\n\nold' "$(date +%F)")
[[ $(cat "$R/CHANGELOG.1.0.md") == "$want" ]] || bad "ok: changelog is $(cat "$R/CHANGELOG.1.0.md")"
[[ $(grep -c '^## ' "$R/CHANGELOG.1.0.md") == 2 ]] || bad "ok: not exactly one new entry"
head=$(git -C "$R" rev-parse HEAD)
run --summary='Again'
[[ $rc == 1 && $out == *"no commit since the last VERSION change"* ]] || bad "again: rc=$rc $out"
untouched again "$head" '^A  other$'

mkrepo nosummary; head=$(git -C "$R" rev-parse HEAD)
run
[[ $rc == 1 && $out == *"--summary='<one or two lines>' is required; the 2 commits since"* && $out == *"  Second change lands"* && $out == *"  First change lands"* ]] || bad "nosummary: rc=$rc $out"
[[ $out == *"Version 1.0.47"* ]] && bad "nosummary: the window includes the VERSION commit"
untouched nosummary "$head"
run --summary='  '
[[ $rc == 1 && $out == *"is required"* ]] || bad "blank summary: rc=$rc $out"
untouched blank "$head"

mkrepo long; head=$(git -C "$R" rev-parse HEAD)
run --summary="$(printf 'one\ntwo\nthree')"
[[ $rc == 1 && $out == *"more than two lines"* ]] || bad "long: rc=$rc $out"
untouched long "$head"
run --summary="$(printf 'one\ntwo')"
[[ $rc == 0 ]] || bad "two lines: rc=$rc $out"
for s in $'one\n\ntwo' $'one\n' $'\none'; do
  mkrepo blankline; head=$(git -C "$R" rev-parse HEAD)
  run --summary="$s"
  [[ $rc == 1 && ( $out == *"blank line"* || $out == *"more than two lines"* ) ]] || bad "blank line in $(printf %q "$s"): rc=$rc $out"
  untouched blankline "$head"
  rm -rf "$R"
done

mkrepo backslash
run --summary='Paths like C:\new\table and a literal \t stay as written'
[[ $rc == 0 ]] || bad "backslash: rc=$rc $out"
grep -qxF 'Paths like C:\new\table and a literal \t stay as written' "$R/CHANGELOG.1.0.md" || bad "backslash: the summary changed: $(sed -n 5,7p "$R/CHANGELOG.1.0.md")"

mkrepo branch; git -C "$R" checkout -qb side; head=$(git -C "$R" rev-parse HEAD)
run --summary=S
[[ $rc == 1 && $out == *"is not on main"* ]] || bad "branch: rc=$rc $out"
untouched branch "$head"

mkrepo dirtyv; head=$(git -C "$R" rev-parse HEAD); echo 9.9.9 >"$R/$V"
run --summary=S
[[ $rc == 1 && $out == *"has uncommitted changes"* ]] || bad "dirty VERSION: rc=$rc $out"
[[ $(cat "$R/$V") == 9.9.9 ]] || bad "dirty VERSION: the edit was overwritten"
mkrepo dirtycl; head=$(git -C "$R" rev-parse HEAD); echo x >>"$R/CHANGELOG.1.0.md"; git -C "$R" add CHANGELOG.1.0.md
run --summary=S
[[ $rc == 1 && $out == *"has uncommitted changes"* ]] || bad "staged changelog: rc=$rc $out"
untouched dirtycl "$head" '^M  CHANGELOG.1.0.md$'

mkrepo nocl 1.1.0; head=$(git -C "$R" rev-parse HEAD)
run --summary=S
[[ $rc == 1 && $out == *"no CHANGELOG.1.1.md for VERSION 1.1.0"* ]] || bad "no changelog: rc=$rc $out"
[[ -e $R/CHANGELOG.1.1.md ]] && bad "no changelog: a changelog was created"
untouched nocl "$head"

mkrepo badv 1.0; head=$(git -C "$R" rev-parse HEAD)
run --summary=S
[[ $rc == 1 && $out == *"VERSION is '1.0', not X.Y.Z"* ]] || bad "bad VERSION: rc=$rc $out"
untouched badv "$head"

mkrepo heading; printf '\n## 1.0.48 — x\n' >>"$R/CHANGELOG.1.0.md"; git -C "$R" commit -qam "A stray heading"; head=$(git -C "$R" rev-parse HEAD)
run --summary=S
[[ $rc == 1 && $out == *"already has a '## 1.0.48' heading"* ]] || bad "heading: rc=$rc $out"
untouched heading "$head"

mkrepo hook; head=$(git -C "$R" rev-parse HEAD)
printf '#!/bin/sh\nexit 1\n' >"$R/.git/hooks/pre-commit"; chmod +x "$R/.git/hooks/pre-commit"
run --summary=S
[[ $rc == 2 && $out == *"the commit was refused"*"restored"* ]] || bad "hook: rc=$rc $out"
[[ $(cat "$R/$V") == 1.0.47 ]] || bad "hook: VERSION not restored"
untouched hook "$head"

mkrepo arg; head=$(git -C "$R" rev-parse HEAD)
run --summary=S --bump
[[ $rc == 1 && $out == *"unknown argument '--bump'"* ]] || bad "arg: rc=$rc $out"
untouched arg "$head"

# The agent pack: bumped with its own entry only when the window changed src/agents/
# beyond the pack's VERSION and CHANGELOG.md.
mkpack() { # the pack's version files in the base commit (before the window), then agent changes in it
  local f; for f in "$@"; do mkdir -p "$R/$(dirname "$f")"; echo x >"$R/$f"; git -C "$R" add "$f"; git -C "$R" commit -qm "Agent change in $f"; done
}
PACK=1 mkrepo pack; mkpack src/agents/slack/slack.ts
run --summary='Bus side' --agents-summary='Slack answers faster'
[[ $rc == 0 && $out == "deploy-bump: 1.0.48 and agent pack 1.0.4 committed "* ]] || bad "pack: rc=$rc $out"
[[ $(cat "$R/src/agents/VERSION") == 1.0.4 && $(cat "$R/$V") == 1.0.48 ]] || bad "pack: versions $(cat "$R/$V") $(cat "$R/src/agents/VERSION")"
[[ $(git -C "$R" show --name-only --format= HEAD | sort | tr '\n' ' ') == "CHANGELOG.1.0.md src/agents/CHANGELOG.md src/agents/VERSION $V " ]] || bad "pack: the commit holds $(git -C "$R" show --name-only --format= HEAD | tr '\n' ' ')"
want=$(printf '# Agent pack changelog\n\nintro\n\n## 1.0.4 — %s\n\nSlack answers faster\n\n## 1.0.3 — 2026-11-05\n\nold' "$(date +%F)")
[[ $(cat "$R/src/agents/CHANGELOG.md") == "$want" ]] || bad "pack: agents changelog is $(cat "$R/src/agents/CHANGELOG.md")"
grep -qx 'Bus side' "$R/CHANGELOG.1.0.md" || bad "pack: the bus entry is missing"
[[ $(git -C "$R" log -1 --format=%s) == "Version 1.0.48" ]] || bad "pack: subject $(git -C "$R" log -1 --format=%s)"

PACK=1 mkrepo packnosum; mkpack src/agents/slack/slack.ts; head=$(git -C "$R" rev-parse HEAD)
run --summary='Bus side'
[[ $rc == 1 && $out == *"--agents-summary='<one or two lines>' is required"* && $out == *"  Agent change in src/agents/slack/slack.ts"* ]] || bad "packnosum: rc=$rc $out"
untouched packnosum "$head"
for s in $'one\n\ntwo' $'a\nb\nc' '  '; do
  run --summary='Bus side' --agents-summary="$s"
  [[ $rc == 1 && ( $out == *"agents summary has"* || $out == *"is required"* ) ]] || bad "packbad $(printf %q "$s"): rc=$rc $out"
  untouched packbad "$head"
done

PACK=1 mkrepo nopack; mkpack; head=$(git -C "$R" rev-parse HEAD)
run --summary='Bus side' --agents-summary='Nothing changed there'
[[ $rc == 1 && $out == *"--agents-summary given, but no commit since"*"changed src/agents/"* ]] || bad "nopack extra: rc=$rc $out"
untouched nopack "$head"
run --summary='Bus side'
[[ $rc == 0 && $out != *"agent pack"* ]] || bad "nopack: rc=$rc $out"
[[ $(cat "$R/src/agents/VERSION") == 1.0.3 ]] || bad "nopack: the pack VERSION moved"
[[ $(git -C "$R" show --name-only --format= HEAD | sort | tr '\n' ' ') == "CHANGELOG.1.0.md $V " ]] || bad "nopack: the commit holds $(git -C "$R" show --name-only --format= HEAD | tr '\n' ' ')"

PACK=1 mkrepo packnocl; mkpack src/agents/slack/slack.ts; git -C "$R" rm -q src/agents/CHANGELOG.md; git -C "$R" commit -qm "Drop it"; head=$(git -C "$R" rev-parse HEAD)
run --summary='Bus side' --agents-summary='S'
[[ $rc == 1 && $out == *"no src/agents/CHANGELOG.md for the agents change"* ]] || bad "packnocl: rc=$rc $out"
[[ -e $R/src/agents/CHANGELOG.md ]] && bad "packnocl: the pack changelog was created"
untouched packnocl "$head"

PACK=1 mkrepo packdirty; mkpack src/agents/slack/slack.ts; echo 9 >"$R/src/agents/VERSION"; head=$(git -C "$R" rev-parse HEAD)
run --summary='Bus side' --agents-summary='S'
[[ $rc == 1 && $out == *"src/agents/VERSION has uncommitted changes"* ]] || bad "packdirty: rc=$rc $out"
[[ $(cat "$R/$V") == 1.0.47 ]] || bad "packdirty: the bus VERSION moved"

PACK=1 mkrepo packhook; mkpack src/agents/slack/slack.ts; head=$(git -C "$R" rev-parse HEAD)
printf '#!/bin/sh\nexit 1\n' >"$R/.git/hooks/pre-commit"; chmod +x "$R/.git/hooks/pre-commit"
run --summary='Bus side' --agents-summary='S'
[[ $rc == 2 ]] || bad "packhook: rc=$rc $out"
untouched packhook "$head"

# A window that creates src/agents/VERSION (the pack's first landing) or edits it by hand
# already sets the pack's version: no pack bump, and --agents-summary is refused.
mkrepo packnew; packfiles; git -C "$R" add src/agents; git -C "$R" commit -qm "The pack has a version"; mkpack src/agents/slack/slack.ts; head=$(git -C "$R" rev-parse HEAD)
run --summary='Bus side' --agents-summary='S'
[[ $rc == 1 && $out == *"the window already sets src/agents/VERSION (1.0.3): the pack is not bumped again"* ]] || bad "packnew extra: rc=$rc $out"
untouched packnew "$head"
run --summary='Bus side'
[[ $rc == 0 && $out != *"agent pack"* ]] || bad "packnew: rc=$rc $out"
[[ $(cat "$R/src/agents/VERSION") == 1.0.3 ]] || bad "packnew: the new pack VERSION moved to $(cat "$R/src/agents/VERSION")"
[[ $(git -C "$R" show --name-only --format= HEAD | sort | tr '\n' ' ') == "CHANGELOG.1.0.md $V " ]] || bad "packnew: the commit holds $(git -C "$R" show --name-only --format= HEAD | tr '\n' ' ')"
PACK=1 mkrepo packhand; mkpack; head0=$(git -C "$R" rev-parse HEAD)
run --summary='Bus side'  # a deploy: the pack's version files are now behind the window
echo y >"$R/src/agents/a.py"; git -C "$R" add src/agents/a.py; git -C "$R" commit -qm "Agent change"
echo 1.1.0 >"$R/src/agents/VERSION"; git -C "$R" commit -qam "The pack moves to 1.1.0 by hand"
run --summary='Bus again'
[[ $rc == 0 && $(cat "$R/src/agents/VERSION") == 1.1.0 ]] || bad "packhand: rc=$rc $out; pack $(cat "$R/src/agents/VERSION")"

# A window that only edits the pack's own changelog is no agents change.
PACK=1 mkrepo packcl; echo typo >>"$R/src/agents/CHANGELOG.md"; git -C "$R" commit -qam "A pack changelog typo"
run --summary='Bus side'
[[ $rc == 0 && $out != *"agent pack"* && $(cat "$R/src/agents/VERSION") == 1.0.3 ]] || bad "packcl: rc=$rc $out"

((fail)) && exit 1
echo "deploy-bump: one PATCH and one entry per window, every refusal changes nothing: green"
