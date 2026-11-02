#!/bin/sh
# The same time service with what a good per-message agent does: answer help
# and status, read its settings from Configuration and its key from Secret,
# keep a little state in its work folder, and never echo the body or a secret.
# region: advanced
case "${AGENT_BUS_ASK:-}" in
  "") ;;
  help) echo "time [now|iso|epoch] - the current UTC time, now in the configured format; history - the last five reports"; exit 0 ;;
  status)
    echo "reports: $(cat "$AGENT_BUS_WORK/reports" 2>/dev/null || echo 0); format: $(printf '%s' "${AGENT_BUS_CONFIG:-}" | sed -n 's/.*"format":"\([a-z]*\)".*/\1/p' | grep . || echo iso)"
    exit 0 ;;
  *) echo "unsupported: $AGENT_BUS_ASK"; exit 0 ;;
esac
format=$(printf '%s' "${AGENT_BUS_CONFIG:-}" | sed -n 's/.*"format":"\([a-z]*\)".*/\1/p' | grep . || echo iso)
word=${1:-now}; [ "$word" = now ] && word=$format
case "$word" in
  history) cat "$AGENT_BUS_WORK/history" 2>/dev/null || echo "no reports yet"; exit 0 ;;
  iso) now=$(date -u +%Y-%m-%dT%H:%M:%SZ) ;;
  epoch) now=$(date -u +%s) ;;
  *) echo "unsupported format; ask help"; exit 0 ;;
esac
# Work folder: lasting state. The stamp proves the Secret was used, never shows it.
stamp=$(printf '%s%s' "${TIME_STAMP_KEY:-}" "$now" | sha256sum | cut -c1-8)
echo $(( $(cat "$AGENT_BUS_WORK/reports" 2>/dev/null || echo 0) + 1 )) >"$AGENT_BUS_WORK/reports"
{ echo "$now"; head -n 4 "$AGENT_BUS_WORK/history" 2>/dev/null || true; } >"$AGENT_BUS_WORK/history.new" && mv "$AGENT_BUS_WORK/history.new" "$AGENT_BUS_WORK/history"
echo "$now stamp=$stamp"
# endregion
