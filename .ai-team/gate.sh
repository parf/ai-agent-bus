#!/usr/bin/env bash
# #ab-gate: the only lander on main (Plans/Team.v2/README.md#roles).
# Run by `agent-bus run '#ab-gate' --algo=json ...`, one message at a time.
# Body: <branch> [T-n] [stable -- line]
# Steps: rebase the branch on main in tmp/gate, take the smoke-gate lock, run
# src/smoke.sh --normal (one rerun on red), fast-forward main. Main moving
# underneath restarts the cycle once. Always exits 0: stdout is the receipt.
# A rerun is never silent (J120, TD7): on red then green the gate hands the first
# run to src/smoke-quarantine.py, which files a J-item per failing test and
# quarantines in src/smoke-map.json those it can name outside a ⚠️ subsystem; the
# gate commits that on top of the branch, re-reading main first, and the receipt
# starts with RERUN-GREEN and one OPEN line per test left unquarantined.
# A landing never bumps (TD6, Plans/Team.v2/DECISIONS.md): #ab-ceo raises PATCH
# once per deploy with .ai-team/deploy-bump.sh, so a body with "bump" is refused,
# and so is "-- line" without stable.
# A Markdown-only branch containing docs/ pages runs DOCS_CHECK plus smoke --fast,
# with no lock or normal gate (J121). Plans/ and root-only Markdown keep the docs
# path below when fast selection names no executable check. LICENSE.md and
# non-Markdown changes always run normal; the selector owns doc-reading checks.
# "stable" is the release landing (Plans/R1.0-Release: the 1.0.0 commit): the
# branch itself carries VERSION X.Y.0 above main's and its changelog; the gate
# raises nothing. It refuses unless the branch is exactly one commit with such a
# VERSION, the line names the reviewed tree and its evidence (it becomes the
# commit body), and the new line's changelog has its "## X.Y.0" heading. It
# runs the same normal gate and lands the commit with the subject exactly
# "STABLE - passed all tests and reviews".
# An agent-pack-only branch (J117) runs the same normal gate, which then runs only
# the checks that own a changed path and the pack contract (the selector's "agent
# pack only" rule, CLAUDE.md § verification). The gate reads that answer to skip
# the Go warm-up and to name those checks in the receipt; any selector error or
# other answer is an ordinary normal landing.
set -uo pipefail

REPO=${GATE_REPO:-/usr/local/src/ai-agent-bus}
SMOKE=${GATE_SMOKE:-src/smoke.sh --normal}
# The docs-only check: links and conflict markers (J29), then the two tests that
# read docs/ (the generated agents index, the page order).
DOCS_CHECK=${GATE_DOCS_CHECK:-python3 src/check-links.py && python3 src/check-markers.py}
DOCS_TESTS=${GATE_DOCS_TESTS:-cd src && python3 -B -m unittest discover -s agents -p agent_docs_test.py && python3 -B -m unittest discover -s agents -p docs_order_test.py}
WT=$REPO/tmp/gate
LOGS=$REPO/tmp/gate-logs
LOCK_REC=smoke-gate@parf.us
LOCK=slow
source "$(dirname "$(readlink -f "$0")")/gate-lock.sh"

env_json=$(cat)
from=$(jq -r '.from // "?"' <<<"$env_json")
body=$(jq -r '.body // ""' <<<"$env_json")

note=""
[[ $body == *" -- "* ]] && { note=${body#* -- }; body=${body%% -- *}; }
read -r branch rest <<<"$body"
ticket="" bump=0 stable=0
for w in ${rest:-}; do
  [[ $w =~ ^T-[0-9]+$ ]] && ticket=$w
  [[ $w == bump ]] && bump=1
  [[ $w == stable ]] && stable=1
done
id=${ticket:-$branch}
mkdir -p "$LOGS"
log=$LOGS/$(date +%Y%m%d-%H%M%S)-${id//\//_}.log

receipt() { echo "gate ${1}: ${id} ${branch} — ${2}"; exit 0; }
g() { git -C "$WT" "$@"; }

[[ -n ${branch:-} ]] || { echo "gate refused: empty body; send '<branch> [T-n] [stable -- line]'"; exit 0; }
((bump)) && receipt refused "bump is retired: #ab-ceo bumps at deploy"
((!stable)) && [[ -n $note ]] && receipt refused "'-- line' goes only with stable: the changelog is written at deploy"
git -C "$REPO" rev-parse --verify -q "refs/heads/$branch" >/dev/null || receipt refused "no local branch '$branch'"
((stable)) && [[ -z $note ]] && receipt refused "stable needs '-- <reviewed tree and evidence>': the line becomes the commit body"

if [[ ! -d $WT ]]; then
  git -C "$REPO" worktree add --detach "$WT" main >>"$log" 2>&1 || receipt failed "cannot create $WT (log $log)"
fi

lock_held=0 runs=0
release() { ((lock_held)) && ${GATE_NOLOCK:+true} agent-bus release "$LOCK_REC" "$LOCK" >>"$log" 2>&1; lock_held=0; }
trap release EXIT

cycle() { # 0 landed, 1 red/refused (receipt printed), 2 main moved
  g reset -q --hard >>"$log" 2>&1; g clean -qfd >>"$log" 2>&1
  g checkout -q --detach "$branch" >>"$log" 2>&1 || receipt failed "cannot check out $branch (log $log)"
  local m # dependencies come from the main checkout, as in every worktree
  for m in src/mcp src/web src/clients/typescript; do
    [[ -e $WT/$m/node_modules || ! -d $REPO/$m/node_modules ]] || ln -s "$REPO/$m/node_modules" "$WT/$m/node_modules"
  done
  if ! g rebase -q main >>"$log" 2>&1; then
    local files; files=$(g diff --name-only --diff-filter=U | tr '\n' ' ')
    g rebase --abort >>"$log" 2>&1
    receipt red "rebase on main conflicts: ${files:-see log} — rebase and resend (log $log)"
  fi
  [[ -n $(g log --oneline main..HEAD) ]] || receipt refused "nothing to land: $branch is already in main"
  local base; base=$(git -C "$REPO" rev-parse main)
  if ((stable)); then # refused before the lock and the long run, not after it
    local mv bv mmaj mmin bmaj bmin cl
    mv=$(git -C "$REPO" show main:src/internal/version/VERSION | tr -d '[:space:]')
    bv=$(tr -d '[:space:]' <"$WT/src/internal/version/VERSION")
    local quarantine # a release ships with no check skipped
    quarantine=$(cd "$WT" && python3 -B src/smoke-quarantine.py list 2>&1) || receipt refused "stable: cannot read the quarantine: $(head -n 1 <<<"$quarantine")"
    [[ -z $quarantine ]] || receipt refused "stable: src/smoke-map.json still quarantines $(cut -f1,2 <<<"$quarantine" | tr '\t\n' '/ ')— land their fixes first"
    [[ $bv =~ ^([0-9]+)\.([0-9]+)\.0$ ]] || receipt refused "stable: the branch's VERSION is '$bv', not X.Y.0"
    bmaj=${BASH_REMATCH[1]} bmin=${BASH_REMATCH[2]}
    [[ $mv =~ ^([0-9]+)\.([0-9]+)\.[0-9]+$ ]] || receipt refused "stable: main's VERSION '$mv' is not X.Y.Z"
    mmaj=${BASH_REMATCH[1]} mmin=${BASH_REMATCH[2]}
    # integer components, major first: no weighted sum that a large minor could beat
    ((10#$bmaj > 10#$mmaj || (10#$bmaj == 10#$mmaj && 10#$bmin > 10#$mmin))) || receipt refused "stable: the branch's VERSION $bv is not a higher X.Y.0 than main's $mv"
    [[ $(g rev-list --count main..HEAD) == 1 ]] || receipt refused "stable: the branch must be exactly one commit above main"
    cl="CHANGELOG.${bv%.*}.md"
    # the version is literal (dots are not wildcards) and ends the word: space or end of line
    grep -qE "^## ${bv//./\\.}( |\$)" "$WT/$cl" 2>/dev/null || receipt refused "stable: $cl has no '## $bv' heading in the branch"
  fi

  # All changed paths, old and new names, must be Markdown under Plans/, docs/
  # or at the root, excluding the shipped LICENSE.md. A docs/ page needs the
  # selected doc-reading checks, at fast only. Plans/ and root-only Markdown
  # keep the existing selector fallback rule; any other answer runs normal.
  local docs_only=1 docs_fast=0 f sel
  while IFS= read -r f; do
    [[ $f =~ ^(Plans|docs)/.+\.md$ || ( $f =~ ^[^/]+\.md$ && $f != LICENSE.md ) ]] || { docs_only=0; break; }
    [[ $f != docs/* ]] || docs_fast=1
  done < <(g diff --name-only --no-renames main HEAD)
  if ((docs_only && !docs_fast)); then
    sel=$(cd "$WT" && python3 src/smoke-select.py --mode fast --out "$log.select" 2>&1) || docs_only=0
    rm -f "$log.select"
    printf '== selection\n%s\n' "$sel" >>"$log"
    grep -qx '  fallback: no executable checks for changed paths: all fast' <<<"$sel" || docs_only=0
  fi
  local t0=$SECONDS rc tries=1 check first=""
  if ((docs_only)); then
    if ((docs_fast)); then
      check="docs check + fast"
      echo "== docs only: $DOCS_CHECK, then src/smoke.sh --fast; no lock and no normal gate" >>"$log"
      (cd "$WT" && bash -c "$DOCS_CHECK" && src/smoke.sh --fast) >>"$log" 2>&1; rc=$?
    else
      check="docs check"
      echo "== docs only: $DOCS_CHECK, then $DOCS_TESTS; no lock and no normal gate" >>"$log"
      (cd "$WT" && bash -c "$DOCS_CHECK" && bash -c "$DOCS_TESTS") >>"$log" 2>&1; rc=$?
    fi
    local secs=$((SECONDS - t0))
    ((rc == 0)) || receipt red "docs-only check rc=$rc (${secs}s, log $log); tail: $(tail -n 8 "$log" | tr '\n' ' ' | cut -c1-600)"
  else
    check=normal
    # Agent pack only: the normal selection's own answer, never a guess from paths.
    local agents_only=0 asel
    if asel=$(cd "$WT" && python3 src/smoke-select.py --mode normal --out "$log.normal" 2>&1) \
      && grep -qx '  agent pack only: checks that own no changed path are skipped' <<<"$asel"; then
      agents_only=1
      check="agents only ($(awk -F '\t' '$2 != "skip" { print $1 }' "$log.normal" | sort | paste -sd' '))"
    fi
    rm -f "$log.normal"
    printf '== normal selection\n%s\n' "$asel" >>"$log"
    if ((!lock_held)) && [[ -z ${GATE_NOLOCK:-} ]]; then
      take_gate_lock "$LOCK_REC" "$LOCK" "$log"
      case $? in
        1) receipt failed "smoke-gate lock not granted in ${GATE_LOCK_BOUND}s: held by ${GATE_LOCK_HOLDER:-unknown}$([[ $GATE_LOCK_HOLDER == *' parf '* ]] && echo ' (another parf session)') (log $log)" ;;
        2) receipt failed "smoke-gate lock refused: ${GATE_LOCK_ERROR} (log $log)" ;;
      esac
      lock_held=1
    elif ((!lock_held)); then
      lock_held=1
    fi
    # Warm Go's build cache for the packages the fast shards test, untimed: a cold
    # cache costs ~12 s for go_unit alone and trips the 15 s fast ceiling (J66).
    # It compiles and links the test binaries and runs no test; failures are the
    # smoke run's to report.
    # An agent-pack-only run has no Go check to warm.
    ((agents_only)) || (cd "$WT/src" && export GOCACHE=${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/go-build} \
      && jq -r '[.. | objects | select(.argv? and .argv[0] == "go" and .argv[1] == "test") | .argv[] | select(startswith("./") and . != "./...")] | unique | .[]' smoke-map.json \
      | xargs -r go test -run '^$') >>"$log" 2>&1 || true
    first=$log.run$((++runs))
    (cd "$WT" && $SMOKE) 2>&1 | tee "$first" >>"$log"; rc=${PIPESTATUS[0]}
    if ((rc != 0)); then
      tries=2; echo "== rerun after rc=$rc" >>"$log"
      (cd "$WT" && $SMOKE) >>"$log" 2>&1; rc=$?
    fi
    local secs=$((SECONDS - t0))
    if ((rc != 0)); then
      release
      receipt red "normal gate rc=$rc twice (${secs}s, log $log); tail: $(tail -n 8 "$log" | tr '\n' ' ' | cut -c1-600)"
    fi
    release
    ((tries > 1)) || rm -f "$first"
    ((tries == 1 || !stable)) || receipt red "stable: green only on the rerun; a release needs a first-run green (log $log)"
  fi

  local ver="no bump" opened=""
  if ((tries > 1)); then
    # Re-read main right before the commit: the J numbers follow its highest one.
    [[ $(git -C "$REPO" rev-parse main) == "$base" ]] || return 2
    opened=$(cd "$WT" && python3 -B src/smoke-quarantine.py apply "$first" "$branch" 2>>"$log") \
      || receipt failed "quarantine helper failed after a green rerun (log $log, first run $first)"
    g add -- src/smoke-map.json Plans/issues >>"$log" 2>&1
    g commit -q -m "$(head -n 1 <<<"$opened")" -m "The gate's first normal run of $branch failed and its rerun passed (J120); first run: $first" >>"$log" 2>&1 \
      || receipt failed "quarantine commit failed (log $log)"
    (cd "$WT" && python3 -B src/smoke-quarantine.py check) >>"$log" 2>&1 \
      || receipt failed "the quarantine the gate wrote does not check (log $log)"
  fi
  if ((stable)); then
    # The commit lands as the release marker: the subject is fixed, the line is its body.
    g commit -q --amend -m "STABLE - passed all tests and reviews" -m "$note" >>"$log" 2>&1 \
      || receipt failed "stable commit message failed (log $log)"
    ver="STABLE $(tr -d '[:space:]' <"$WT/src/internal/version/VERSION")"
  fi

  [[ $(git -C "$REPO" rev-parse main) == "$base" ]] || return 2
  local tip; tip=$(g rev-parse HEAD)
  git -C "$REPO" merge -q --ff-only "$tip" >>"$log" 2>&1 || {
    [[ $(git -C "$REPO" rev-parse main) == "$base" ]] && receipt failed "ff-merge refused in the main checkout (dirty overlap?) (log $log)"
    return 2
  }
  printf 'LANDED range=%s..%s branch=%s topic=%s\n' "$base" "$tip" "$branch" "${ticket:-$(jq -r '.topic // ""' <<<"$env_json")}" >>"$log"
  [[ -z $opened ]] || printf '%s\n' "$opened"
  receipt landed "main ${tip:0:8}, ${ver}, ${check} ${secs}s; worker removes its worktree and branch"
}

echo "== $(date -Is) from $from: $branch ${ticket:-} ${note:+-- $note}" >>"$log"
cycle; [[ $? == 2 ]] && { echo "== main moved, second cycle" >>"$log"; cycle; }
receipt red "main kept moving during two cycles — resend (log $log)"
