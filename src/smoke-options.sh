# Shared by smoke.sh and its mode-contract check; no work is started here.
smoke_options() {
  MODE=${MODE:-fast}; TARGETS=${TARGETS:-}; SHARD=""
  while [ $# -gt 0 ]; do
    case $1 in
      --fast) MODE=fast; TARGETS="" ;;
      --normal|--slow)
        MODE=${1#--}; TARGETS=""
        if [ $# -gt 1 ] && [[ $2 != --* ]]; then TARGETS=$2; shift; fi ;;
      --shard) SHARD=${2:-}; shift ;;
      *) echo "usage: smoke.sh [--fast | --normal [A,F] | --slow [L|all]] [--shard NAME]" >&2; return 2 ;;
    esac
    shift
  done
}
