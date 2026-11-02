#!/bin/sh
# The simplest agent: answer help, then tell the time. The message arrives as
# the first argument; whatever the script prints is the answer.
# region: simple
case "${AGENT_BUS_ASK:-}" in
  "") ;;                                         # an ordinary request: carry on
  help) echo "time [iso|epoch] - the current UTC time"; exit 0 ;;
  *) echo "unsupported: $AGENT_BUS_ASK"; exit 0 ;;
esac
case "${1:-iso}" in
  iso) date -u +%Y-%m-%dT%H:%M:%SZ ;;
  epoch) date -u +%s ;;
  *) echo "unsupported format; ask help" ;;      # the body is data, never echoed back
esac
# endregion
