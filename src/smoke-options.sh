# Shared by smoke.sh and its mode-contract check; no work is started here.
smoke_options() {
  FAST=${FAST:-0}; SLOW=${SLOW:-0}; SHARD=""
  while [ $# -gt 0 ]; do
    case $1 in
      --fast) FAST=1; SLOW=0 ;;
      --slow) FAST=0; SLOW=1 ;;
      --shard) SHARD=${2:-}; shift ;;
      *) echo "usage: smoke.sh [--fast | --slow] [--shard NAME]" >&2; return 2 ;;
    esac
    shift
  done
}
