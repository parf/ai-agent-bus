#!/usr/bin/env bash
# deploy-bump: one version bump per deploy (TD6, Plans/Team.v2/DECISIONS.md).
# #ab-ceo runs it in the main checkout before a deploy; a landing never bumps.
# Usage: .ai-team/deploy-bump.sh --summary='<one or two lines>' [--agents-summary='<one or two lines>'] [--seed]
# AV.1: every bundled agent has its own version and released (agent.json). Besides the bus and pack bumps below,
# each agent whose shipped files changed since the last deploy gets PATCH+1 and released=<today UTC>, written together into its agent.json inside the
# same commit, measured from the newest deploy commit (the `Version X.Y.Z` commit that changed src/internal/version/VERSION):
# what an agent shipped up to it is deployed already. An agent ships its folder (not CHANGELOG.md, help.golden.json or test files), the files its smoke-map checks list beside that
# folder (its agent_<name> paths), and src/clients/python/agent_bus/ when its package.sh bundles the client.
# --seed is the first per-agent bump: an agent.json without version gets 1.0.0 and without released the day;
# an agent that has both is bumped as usual. Without --seed, a bundled agent lacking either refuses, naming it.
# --ship=<agent dir, e.g. standalone/git> prints the paths that agent ships and exits.
# A bump the previous deploy window missed (shipped changes, no version/released change in that window) refuses by
# agent name with the commit range; --accept-missed bumps those agents now.
# --plan prints the per-agent plan (kind, agent, old, new; nothing for an agent not bumped) and exits, writing nothing.
# The window is every commit since the last one that changed the bus VERSION.
# It raises PATCH in src/internal/version/VERSION once and writes one entry
#   ## X.Y.Z — YYYY-MM-DD
#   <summary>
# on top of CHANGELOG.<X.Y>.md (resolved from VERSION, never created). When the
# window changed src/agents/ beyond the pack's own VERSION and CHANGELOG.md, it
# also raises the agent pack's PATCH in src/agents/VERSION and writes the same
# form of entry, from --agents-summary, on top of src/agents/CHANGELOG.md; then
# --agents-summary is required, and it is refused when the window has no agents
# change. A window that itself created or edited src/agents/VERSION (the pack's
# first version, or a hand bump) has its pack version already: no pack bump, and
# --agents-summary is refused. Everything goes into one commit, "Version X.Y.Z", holding those files
# alone. A missing summary refuses and prints the subjects to write it from.
# Refuses, changing nothing: not on main; a version file or changelog modified or
# staged; a version not X.Y.Z; a changelog missing or already holding the new
# heading; no commit since the last VERSION change; a summary empty, over two
# lines or with a blank line. Exit 0 committed, 1 refused, 2 the commit failed
# (every file restored).
set -uo pipefail
REPO=${DEPLOY_REPO:-/usr/local/src/ai-agent-bus}
V=src/internal/version/VERSION AV=src/agents/VERSION ACL=src/agents/CHANGELOG.md
refuse() { echo "deploy-bump refused: $1" >&2; exit 1; }
g() { git -C "$REPO" "$@"; }
usage="usage: deploy-bump.sh --summary='<one or two lines>' [--agents-summary='<one or two lines>'] [--seed] [--accept-missed]
       deploy-bump.sh --plan | --ship=<agent dir> | --help | -h"
help() {
  cat <<H
$usage

  --summary=<text>         required: one or two lines for the bus changelog entry
  --agents-summary=<text>  one or two lines for src/agents/CHANGELOG.md; required when the window changed src/agents/,
                           refused when it did not or when the window already sets src/agents/VERSION
  --seed                   the first per-agent bump: an agent.json without version gets 1.0.0 and without released the day;
                           without it, a bundled agent lacking either is refused by name
  --accept-missed          bump now an agent whose shipped files changed in the previous deploy window without its version
                           or released changing there (otherwise refused by name, with the commit range)
  --plan                   print the per-agent plan (kind, agent, old, new) and exit; writes nothing
  --ship=<agent dir>       print the paths that agent ships (for example standalone/git) and exit; writes nothing
  --help, -h               this text; changes nothing

One "Version X.Y.Z" commit per deploy on main. Exit 0 committed, 1 refused, 2 the commit failed (every file restored).
H
}

summary="" have_summary=0 asummary="" have_asummary=0 seed=0 plan_only=0 ship= accept_missed=0
for a in "$@"; do
  case $a in
    --help|-h) help; exit 0 ;;
    --summary=*) summary=${a#--summary=}; have_summary=1 ;;
    --agents-summary=*) asummary=${a#--agents-summary=}; have_asummary=1 ;;
    --seed) seed=1 ;;
    --plan) plan_only=1 ;;
    --accept-missed) accept_missed=1 ;;
    --ship=*) ship=${a#--ship=} ;;
    *) refuse "unknown argument '$a'; $usage" ;;
  esac
done


read -r -d '' AV_PY <<'PYEOF' || true
import datetime, json, os, re, subprocess, sys
from pathlib import Path
repo, mode, seed, today = Path(sys.argv[1]), sys.argv[2], sys.argv[3] == '1', sys.argv[4]
AG = repo / 'src/agents'
VER = re.compile(r'^(\s*"version"\s*:\s*)"[^"]*"', re.M)
REL = re.compile(r'^(\s*"released"\s*:\s*)"[^"]*"', re.M)
def git(*a):
    return subprocess.run(['git', '-C', str(repo), *a], capture_output=True, text=True, check=True).stdout
def manifests():
    return sorted(p for p in AG.rglob('agent.json') if 'node_modules' not in p.parts)
TESTFILE = re.compile(r'(^test_.*\.py|_test\.py|\.test\.ts|_test\.go)$')
SHARED = AG / 'shared'
def imported_shared(directory):
    """Shared modules an agent folder ships: its package.sh's ../shared/X copies, the shared modules its Python and
    TS sources import (a module the folder itself defines is its own), then what those modules import in turn."""
    found, local = set(), {p.stem for p in directory.glob('*.py')}
    texts = []
    package = directory / 'package.sh'
    if package.is_file():
        texts.append(package.read_text())
        found |= {m for m in re.findall(r'shared/([A-Za-z0-9_.-]+\.(?:py|ts))', package.read_text())}
    for source in list(directory.rglob('*.py')) + list(directory.rglob('*.ts')):
        if 'node_modules' in source.parts or TESTFILE.search(source.name):
            continue
        text = source.read_text(errors='replace')
        if source.suffix == '.py':
            for name in re.findall(r'^\s*(?:from|import)\s+([A-Za-z_][A-Za-z0-9_]*)', text, re.M):
                if name not in local and (SHARED / f'{name}.py').is_file():
                    found.add(f'{name}.py')
        else:
            found |= {m for m in re.findall(r'["\'](?:\.\./)+shared/([A-Za-z0-9_.-]+\.ts)["\']', text)}
    pending = list(found)
    while pending:
        name = pending.pop()
        path = SHARED / name
        if not path.is_file():
            continue
        text = path.read_text(errors='replace')
        more = set()
        if name.endswith('.py'):
            more = {f'{m}.py' for m in re.findall(r'^\s*(?:from|import)\s+([A-Za-z_][A-Za-z0-9_]*)', text, re.M) if (SHARED / f'{m}.py').is_file()}
        else:
            more = {m for m in re.findall(r'from\s+["\']\./([A-Za-z0-9_.-]+\.ts)["\']', text) if (SHARED / m).is_file()}
        for m in more - found:
            found.add(m); pending.append(m)
    return {f'src/agents/shared/{n}' for n in found if (SHARED / n).is_file()}
def shipped(rel):
    folder = f'src/agents/{rel}/'
    paths = [folder]
    smoke = json.loads((repo / 'src/smoke-map.json').read_text())
    for check in smoke.get('checks', {}).values():
        listed = check.get('paths', []) if isinstance(check, dict) else []
        if folder in listed:
            for x in listed:
                # Declared files only when they ship from src/agents/: shared files and standalone/install.py;
                # never a Go, client-test or other agent's path a check also lists. Test files never ship.
                if x.startswith('src/agents/') and not x.endswith('/') and not TESTFILE.search(Path(x).name) \
                        and (x.startswith('src/agents/shared/') or x == 'src/agents/standalone/install.py'):
                    paths.append(x)
    paths += sorted(x for x in imported_shared(AG / rel) if not TESTFILE.search(Path(x).name))
    package = AG / rel / 'package.sh'
    if package.is_file() and 'clients/python/agent_bus' in package.read_text():
        paths.append('src/clients/python/agent_bus/')
    return folder, sorted(set(paths))
def deploy_commits():
    """The deploy commits, newest first: the `Version X.Y.Z` subject that changes src/internal/version/VERSION. An agent
    has shipped everything up to the newest, so it is bumped only for shipped files changed since, never since its own
    last bump."""
    # The subject alone: a body line that reads "Version 1.0.2" never makes a deploy commit.
    out = git('log', '--format=%H %s', '--', 'src/internal/version/VERSION')
    return [line.split(' ', 1)[0] for line in out.splitlines() if re.fullmatch(r'\S+ Version [0-9]+\.[0-9]+\.[0-9]+', line)]
def shipped_names(paths, a, b):
    return [n for n in git('diff', '--name-only', a, b, '--', *paths).split('\n')
            if n and Path(n).name not in ('CHANGELOG.md', 'help.golden.json') and not TESTFILE.search(Path(n).name)]
def plan():
    deploys = deploy_commits()
    baseline = deploys[0] if deploys else ''
    previous = deploys[1] if len(deploys) > 1 else ''
    accept = os.environ.get('AV_ACCEPT_MISSED') == '1'
    for path in manifests():
        rel = str(path.parent.relative_to(AG))
        data = json.loads(path.read_text())
        have_v, have_r = 'version' in data, 'released' in data
        if not (have_v and have_r) and not seed:
            print(f'refuse\t{rel}\tlacks {"version" if not have_v else "released"}; --seed writes the first pair')
            continue
        if not have_v:
            print(f'seed\t{rel}\t-\t1.0.0'); continue
        if not have_r:
            print(f'stamp\t{rel}\t{data["version"]}\t{data["version"]}'); continue
        rp = str(path.relative_to(repo))
        # The pair moves together: the last bump is the newest commit that changed either line.
        last = git('log', '-1', '--format=%H', '-G^\\s*"(version|released)"\\s*:', '--', rp).strip()
        if not last:
            print(f'refuse\t{rel}\tno commit changed its version'); continue
        if not baseline:
            print(f'refuse\t{rel}\tno deploy commit (Version X.Y.Z) to measure from'); continue
        shown = git('show', '--format=', '-U0', last, '--', rp)
        changed = lambda key: re.search(r'^[+-]\s*"' + key + r'"\s*:', shown, re.M) is not None
        # A reformat of either line counts as a change here, so it is a bump: the pair is only ever moved by the bump.
        first_stamp = re.search(r'^\+\s*"released"\s*:', shown, re.M) and not re.search(r'^-\s*"released"\s*:', shown, re.M)
        if changed('version') and not changed('released'):
            print(f'refuse\t{rel}\tcommit {last[:8]} changed version without released: the pair must move together'); continue
        if changed('released') and not changed('version') and not first_stamp:
            print(f'refuse\t{rel}\tcommit {last[:8]} changed released without version: the pair must move together'); continue
        folder, paths = shipped(rel)
        # Bookkeeping and fixtures never ship: an agent's own CHANGELOG.md (added or edited, at any depth), its reviewed
        # help.golden.json and every test file by pattern.
        names = shipped_names(paths, baseline, 'HEAD')
        # A bump must never be missed silently: shipped changes in the previous deploy window without a version or
        # released change in that same window (an older script, a hand-made Version commit) were lost to the baseline.
        missed = bool(previous and shipped_names(paths, previous, baseline) and not git(
            'log', '-1', '--format=%H', '-E', '-G^\\s*"(version|released)"\\s*:', f'{previous}..{baseline}', '--', rp).strip())
        if missed and not accept:
            print(f'refuse\t{rel}\tmissed bump: shipped files changed in {previous[:8]}..{baseline[:8]} with no version or released change; --accept-missed bumps it now'); continue
        if missed:
            names = names or ['(missed in the previous deploy window)']
        if names:
            major, minor, patch = data['version'].split('.')
            print(f'bump\t{rel}\t{data["version"]}\t{major}.{minor}.{int(patch) + 1}')
def edit(path, new):
    text = Path(path).read_text()
    if VER.search(text):
        text = VER.sub(lambda m: m[1] + json.dumps(new), text, 1)
    elif new != '-':
        text = re.sub(r'^(\s*"description"\s*:.*\n)', lambda m: m[1] + f'  "version": {json.dumps(new)},\n', text, 1, re.M)
    line = f'"released": "{today}"'
    if REL.search(text):
        text = REL.sub(lambda m: m[1] + json.dumps(today), text, 1)
    else:
        text = re.sub(r'^(\s*"version"\s*:.*\n)', lambda m: m[1] + f'  {line},\n', text, 1, re.M)
    json.loads(text)
    sys.stdout.write(text)
if mode == 'plan':
    plan()
elif mode == 'ship':
    print('\n'.join(shipped(sys.argv[5])[1]))
else:
    edit(sys.argv[5], sys.argv[6])
PYEOF

clean() { [[ -z $(g status --porcelain -- "$1") ]] || refuse "$1 has uncommitted changes"; }
bumped() { # version-file label: prints PATCH+1; refuses anything but X.Y.Z
  local cur; cur=$(tr -d '[:space:]' <"$REPO/$1" 2>/dev/null)
  [[ $cur =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]] || refuse "$2 is '$cur', not X.Y.Z"
  echo "${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.$((10#${BASH_REMATCH[3]} + 1))"
}
fresh() { ! grep -qE "^## ${2//./\\.}( |$)" "$REPO/$1" || refuse "$1 already has a '## $2' heading"; }
lines() { # what text: one or two lines, none blank
  [[ $2 != *$'\n'*$'\n'* ]] || refuse "$1 has more than two lines"
  [[ $2 != *$'\n'* || ( -n ${2%%$'\n'*} && -n ${2#*$'\n'} ) ]] || refuse "$1 has a blank line"
}

if [[ -n $ship ]]; then
  python3 -I -c "$AV_PY" "$REPO" ship 0 "$(date -u +%F)" "$ship"; exit $?
fi
if ((plan_only)); then
  AV_ACCEPT_MISSED=$accept_missed python3 -I -c "$AV_PY" "$REPO" plan "$seed" "$(date -u +%F)"; exit $?
fi
[[ $(g symbolic-ref -q --short HEAD) == main ]] || refuse "$REPO is not on main"
clean "$V"
next=$(bumped "$V" VERSION) || exit 1
cl=CHANGELOG.${next%.*}.md
[[ -f $REPO/$cl ]] || refuse "no $cl for VERSION $(tr -d '[:space:]' <"$REPO/$V"): start a new line's changelog by hand"
clean "$cl"
last=$(g log -1 --format=%H -- "$V")
[[ -n $last ]] || refuse "no commit has changed $V"
count=$(g rev-list --count "$last..HEAD")
((count > 0)) || refuse "no commit since the last VERSION change (${last:0:8})"
fresh "$cl" "$next"
# The pack is bumped only for a change to the agents themselves.
agents=$(g diff --name-only "$last" HEAD -- src/agents/ ":(exclude)$AV" ":(exclude)$ACL")
anext=""
if [[ -n $agents && -n $(g diff --name-only "$last" HEAD -- "$AV") ]]; then
  ((!have_asummary)) || refuse "--agents-summary given, but the window already sets $AV ($(tr -d '[:space:]' <"$REPO/$AV")): the pack is not bumped again"
elif [[ -n $agents ]]; then
  [[ -f $REPO/$AV ]] || refuse "no $AV for the agents change"
  [[ -f $REPO/$ACL ]] || refuse "no $ACL for the agents change: start it by hand"
  clean "$AV"; clean "$ACL"
  anext=$(bumped "$AV" "$AV") || exit 1
  fresh "$ACL" "$anext"
elif ((have_asummary)); then
  refuse "--agents-summary given, but no commit since ${last:0:8} changed src/agents/"
fi
# Each bundled agent's own bump, planned before anything is written.
utoday=$(date -u +%F)
plan=$(AV_ACCEPT_MISSED=$accept_missed python3 -I -c "$AV_PY" "$REPO" plan "$seed" "$utoday") || refuse "cannot plan the per-agent bumps"
agent_rel=() agent_new=() agent_note=()
while IFS=$'\t' read -r kind rel old new; do
  [[ -n $kind ]] || continue
  case $kind in
    refuse) refuse "agent $rel $old" ;;
    *) clean "src/agents/$rel/agent.json"; agent_rel+=("$rel"); agent_new+=("$new"); agent_note+=("$rel $kind ${old/#-/none}→$new") ;;
  esac
done <<<"$plan"
if ((!have_summary)) || [[ -z ${summary//[[:space:]]/} ]]; then
  echo "deploy-bump refused: --summary='<one or two lines>' is required; the $count commits since ${last:0:8}:" >&2
  g log --format='  %s' "$last..HEAD" >&2
  exit 1
fi
lines "the summary" "$summary"
if [[ -n $anext ]]; then
  if ((!have_asummary)) || [[ -z ${asummary//[[:space:]]/} ]]; then
    echo "deploy-bump refused: --agents-summary='<one or two lines>' is required: the window changed src/agents/; its commits:" >&2
    g log --format='  %s' "$last..HEAD" -- src/agents/ ":(exclude)$AV" ":(exclude)$ACL" >&2
    exit 1
  fi
  lines "the agents summary" "$asummary"
fi

# Compute every file first, then rename each into place.
files=("$V" "$cl") tmps=()
[[ -n $anext ]] && files+=("$AV" "$ACL")
nbase=${#files[@]}
for rel in "${agent_rel[@]}"; do files+=("src/agents/$rel/agent.json"); done
trap 'rm -f "${tmps[@]}"' EXIT
for f in "${files[@]}"; do
  t=$(mktemp "$REPO/$f.XXXXXX") || refuse "cannot write temporary files"
  tmps+=("$t")
done
entry() { # changelog heading text out; the text goes through the environment: awk -v would turn "\n" into a newline
  DB_HEAD=$2 DB_SUMMARY=$3 awk '
    !done && /^## / { print ENVIRON["DB_HEAD"]; print ""; print ENVIRON["DB_SUMMARY"]; print ""; done=1 } { print }
    END { if (!done) { print ""; print ENVIRON["DB_HEAD"]; print ""; print ENVIRON["DB_SUMMARY"] } }' "$REPO/$1" >"$4"
}
today=$(date +%F)
{ printf '%s\n' "$next" >"${tmps[0]}" && entry "$cl" "## $next — $today" "$summary" "${tmps[1]}"; } || refuse "cannot build the bus entry"
if [[ -n $anext ]]; then
  { printf '%s\n' "$anext" >"${tmps[2]}" && entry "$ACL" "## $anext — $today" "$asummary" "${tmps[3]}"; } || refuse "cannot build the agents entry"
fi
for i in "${!agent_rel[@]}"; do
  python3 -I -c "$AV_PY" "$REPO" edit "$seed" "$utoday" "$REPO/${files[nbase+i]}" "${agent_new[i]}" >"${tmps[nbase+i]}" || refuse "cannot write ${files[nbase+i]}"
done
restore() { g checkout -q HEAD -- "${files[@]}"; }
for i in "${!files[@]}"; do
  chmod --reference="$REPO/${files[i]}" "${tmps[i]}"
  mv "${tmps[i]}" "$REPO/${files[i]}" || { restore; refuse "cannot rename ${files[i]} into place"; }
done
if ! g commit -q -m "Version $next" -- "${files[@]}"; then
  restore
  echo "deploy-bump failed: the commit was refused; ${files[*]} are restored" >&2
  exit 2
fi
echo "deploy-bump: $next${anext:+ and agent pack $anext} committed $(g rev-parse --short HEAD) for $count commits since ${last:0:8}"
((${#agent_note[@]})) && printf 'deploy-bump agent: %s\n' "${agent_note[@]}"
