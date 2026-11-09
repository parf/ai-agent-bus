#!/usr/bin/env bash
# deploy-bump: one version bump per deploy (TD6, Plans/Team.v2/DECISIONS.md).
# #ab-ceo runs it in the main checkout before a deploy; a landing never bumps.
# Usage: .ai-team/deploy-bump.sh --summary='<one or two lines>' [--agents-summary='<one or two lines>']
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
usage="usage: deploy-bump.sh --summary='<one or two lines>' [--agents-summary='<one or two lines>']"

summary="" have_summary=0 asummary="" have_asummary=0
for a in "$@"; do
  case $a in
    --summary=*) summary=${a#--summary=}; have_summary=1 ;;
    --agents-summary=*) asummary=${a#--agents-summary=}; have_asummary=1 ;;
    *) refuse "unknown argument '$a'; $usage" ;;
  esac
done

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
