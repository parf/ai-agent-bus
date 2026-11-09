#!/usr/bin/env bash
# Start the AB-DVP team through #tmux-sessions and open two Kitty tabs: CEO and the team.
set -euo pipefail

team_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repo_dir=$(git -C "$team_dir" rev-parse --show-toplevel)
config=$team_dir/ab-dvp-v2-config.json
session=AB-DVP
sessions_agent='#tmux-sessions/ab-dvp@parf.us'
run=0
roles=0
stop=0
status=0
kitty_socket=${KITTY_LISTEN_ON:-}

usage() {
    cat <<'EOF'
Usage: .ai-team/start-agents.sh [--status | --stop | --roles] [--to KITTY_SOCKET]

Without arguments, start the whole team and CEO. Use --status to inspect without starting.

On start:
  1. start missing windows through #tmux-sessions/ab-dvp (never by hand);
  2. open two Kitty tabs: CEO (the #ab-ceo session, unless it is already live)
     and AB-DVP (the team's tmux session; switch windows inside it);
  3. return at once; a detached follow-up waits up to 120 s for every #ab- name
     to have a reader (and sends --roles), logging to tmp/start-agents.log.
An unavailable Kitty socket starts a detached Kitty with remote control
(unix:$XDG_RUNTIME_DIR/ab-dvp-kitty-$UID); the tabs open inside it.
Open tabs are kept; live windows are never restarted.

  --run              Start the team and open the tabs (also the default)
  --status           Show help, Kitty availability and team status only
  --stop             Stop every team window through #tmux-sessions (SIGTERM, 10 s to
                     save); the CEO tab is not a team window and keeps running
  --roles            Send every team window a message to re-read its role file,
                     Plans/Team.v2/roles/common.md and its checkpoint; sent,
                     not waited for (after --run when both)
  --to KITTY_SOCKET  Target a Kitty remote-control socket
  -h, --help         Show this help and current team status
EOF
}

while (($#)); do
    case "$1" in
        --run) run=1; shift ;;
        --roles) roles=1; shift ;;
        --stop) stop=1; shift ;;
        --status) status=1; shift ;;
        --to)
            if (($# < 2)) || [[ -z $2 ]]; then
                echo 'start-agents: --to requires a Kitty socket' >&2
                exit 64
            fi
            kitty_socket=$2
            shift 2
            ;;
        -h|--help) usage; exit 0 ;;
        *) echo "start-agents: unknown option: $1" >&2; usage >&2; exit 64 ;;
    esac
done

if (( ! run && ! stop && ! roles && ! status )); then run=1; fi

if (( run && stop )); then echo 'start-agents: --run and --stop together' >&2; exit 64; fi

for executable in agent-bus jq tmux; do
    if ! command -v "$executable" >/dev/null 2>&1; then
        echo "start-agents: missing command: $executable" >&2
        exit 69
    fi
done

# The owner's own credential: a session's AGENT_BUS_* would act as that agent.
owner() { env -u AGENT_BUS_TOKEN -u AGENT_BUS_ADDR -u AGENT_BUS_NAME -u AGENT_BUS_SESSION_FILE agent-bus "$@"; }
body() { jq -r '.body // empty' 2>/dev/null; }

mapfile -t windows < <(jq -r '.windows[].name' "$config")
mapfile -t names < <(jq -r '.windows[].env.AGENT_BUS_NAME // empty' "$config")
expected=("${names[@]}" '#ab-gate' '#ab-ceo')

live_names() { owner ls --tag ab-dvp --json | jq -r '.[] | select((.readers // 0) > 0) | .name' 2>/dev/null || true; }
missing_names() {
    local live; live=$(live_names)
    for name in "${expected[@]}"; do
        grep -qxF -- "$name" <<< "$live" || grep -q -- "^$name@" <<< "$live" || printf '%s\n' "$name"
    done
}

# The role file a window was started with: its prompt file, or the file its first prompt names.
role_file() {
    jq -r --arg n "$1" '.windows[] | select(.env.AGENT_BUS_NAME == $n) | .command' "$config" |
        grep -oE '/usr/local/src/ai-agent-bus/Plans/Team.v2/roles/[a-z0-9]+\.md' | head -1
}
role_message() {
    printf 'Read your role again now: %s, then Plans/Team.v2/roles/common.md (shared rules, adopted 2026-11-05) and your checkpoint if you have one. Follow them from now on. No reply is needed. Do not start a task because of this message.' "$1"
}
tell_roles() {
    local live name file failed=0 out
    live=$(live_names)
    for name in "${names[@]}"; do
        file=$(role_file "$name")
        [[ -n $file ]] || { printf 'start-agents: no role file for %s in the config.\n' "$name" >&2; failed=1; continue; }
        # Fire and forget: the message waits in the name's inbox until the window reads it.
        if out=$(owner send --topic=role --tag="ab-dvp-role-${name#\#}" "$name" "$(role_message "$file")" 2>&1); then
            grep -qxF -- "$name" <<< "$live" && printf 'told %s\n' "$name" || printf 'queued for %s (no reader now)\n' "$name"
        else
            printf 'start-agents: could not send to %s: %s\n' "$name" "$out" >&2; failed=1
        fi
    done
    return "$failed"
}

kitty_args=(@ --to "$kitty_socket")
kitty_ready=0
if (( ! run )); then usage; printf '\n'; fi
if ! command -v kitty >/dev/null 2>&1; then
    echo 'Kitty: not installed.'
elif [[ -z $kitty_socket ]]; then
    echo 'Kitty: no socket configured (KITTY_LISTEN_ON is empty); --run opens a detached Kitty.'
elif kitty "${kitty_args[@]}" ls >/dev/null 2>&1; then
    printf 'Kitty: remote control ready (%s).\n' "$kitty_socket"
    kitty_ready=1
else
    printf 'Kitty: socket unavailable (%s).\n' "$kitty_socket"
fi

echo 'Windows (#tmux-sessions):'
owner call "$sessions_agent" --wait 30s list | body | sed 's/^/  /'
mapfile -t missing < <(missing_names)
printf 'Team names: %d with a reader, %d missing%s\n' \
    "$(( ${#expected[@]} - ${#missing[@]} ))" "${#missing[@]}" \
    "$( ((${#missing[@]})) && printf ': %s' "${missing[*]}")"
if (( stop )); then
    echo 'Stopping every team window through #tmux-sessions:'
    owner call "$sessions_agent" --wait 120s stop | body | sed 's/^/  /'
    echo 'CEO keeps running; close its tab to stop it.'
    exit 0
fi
if (( ! run )); then
    (( roles )) && { tell_roles; exit $?; }
    exit 0
fi

if (( ! kitty_ready )); then
    if [[ ${AB_DVP_KITTY_BOOTSTRAPPED:-0} == 1 ]]; then
        echo 'start-agents: new Kitty did not provide a working socket; check remote control.' >&2
        exit 69
    fi
    if [[ -z ${DISPLAY:-} && -z ${WAYLAND_DISPLAY:-} ]]; then
        echo 'start-agents: no graphical display; run from a desktop session or pass --to.' >&2
        exit 69
    fi
    new_socket="unix:${XDG_RUNTIME_DIR:-/tmp}/ab-dvp-kitty-$UID"
    echo 'Opening detached Kitty; the team tabs open inside it.'
    exec env -u KITTY_LISTEN_ON AB_DVP_KITTY_BOOTSTRAPPED=1 kitty --detach \
        --listen-on "$new_socket" -o allow_remote_control=socket-only \
        --directory "$repo_dir" "$team_dir/start-agents.sh" --run --to "$new_socket"
fi

echo 'Starting missing windows through #tmux-sessions:'
owner call "$sessions_agent" --wait 60s start | body | sed 's/^/  /'

open_titles=$(kitty "${kitty_args[@]}" ls | jq -r '.[].tabs[].title')
has_tab() { grep -qxF -- "$1" <<< "$open_titles"; }

# Tab 1: the CEO session, unless #ab-ceo is already live somewhere.
if grep -qxF -- '#ab-ceo' <<< "$(live_names)"; then
    echo 'Keeping CEO: #ab-ceo is already live.'
elif has_tab CEO; then
    echo 'Keeping tab CEO: already open.'
else
    # A launch from a Claude Code tool shell inherits CLAUDE_CODE_CHILD_SESSION, which turns transcripts off.
    kitty "${kitty_args[@]}" launch --type=tab --tab-title CEO --title CEO --cwd "$team_dir/ceo" \
        env -u CLAUDE_CODE_CHILD_SESSION -u AGENT_BUS_TOKEN AGENT_BUS_NAME='#ab-ceo' AB_TAB_TITLE=CEO \
        ab-claude -2 --tag ab-dvp --descr 'CEO: owner contact for AB-DVP' \
        --model claude-opus-5-5 \
        --append-system-prompt "$(cat "$repo_dir/Plans/Team.v2/roles/ceo.md")" \
        "Start: read $team_dir/ceo/handoff.md; the team was started by .ai-team/start-agents.sh (check readers, do not start it again); then report to the owner." >/dev/null
    echo 'Opened tab CEO.'
fi

# Tab 2: the whole AB-DVP tmux session; switch windows inside it.
if has_tab "$session"; then
    printf 'Keeping tab %s: already open.\n' "$session"
else
    kitty "${kitty_args[@]}" launch --type=tab --tab-title "$session" --title "$session" \
        --cwd "$repo_dir" tmux attach -t "=$session" >/dev/null
    printf 'Opened tab %s.\n' "$session"
fi

# Wait for readers in a detached follow-up, so --run returns at once.
log=$repo_dir/tmp/start-agents.log
mkdir -p "$repo_dir/tmp"
follow_up() {
    local deadline=$((SECONDS + 120))
    while :; do
        mapfile -t missing < <(missing_names)
        if ((${#missing[@]} == 0)); then
            echo "$(date '+%F %T') all team names have a reader."
            (( roles )) && tell_roles
            return
        fi
        ((SECONDS >= deadline)) && break
        sleep 5
    done
    echo "$(date '+%F %T') no reader after 120 s: ${missing[*]}"
    (( roles )) && tell_roles
}
follow_up >>"$log" 2>&1 </dev/null &
disown
printf 'Readers are checked in the background (120 s); see %s or run .ai-team/start-agents.sh.\n' "$log"
exit 0
