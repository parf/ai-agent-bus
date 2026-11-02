#!/bin/bash
# What the runner's sandbox lets an agent see and do. The driver sends one word
# and its argument as the message body: view <path outside>, net <port>, limits.
set -- $1
case "${1:-}" in
  view)
    here=$(cd "$(dirname "$0")" && pwd)
    ( : >"$here/escape" ) 2>/dev/null && echo "package: writable" || echo "package: read-only"
    ( : >"$AGENT_BUS_WORK/kept" ) 2>/dev/null && echo "work: writable" || echo "work: read-only"
    ( : >"${2:-/nonexistent/outside}" ) 2>/dev/null && echo "outside: writable" || echo "outside: denied"
    ;;
  net)
    ( exec 3<>"/dev/tcp/127.0.0.1/${2:-0}" ) 2>/dev/null && echo "network: reached" || echo "network: denied"
    ;;
  limits)
    cg=$(sed 's/^0:://' /proc/self/cgroup)
    for f in cpu.max memory.max pids.max; do echo "$f: $(cat "/sys/fs/cgroup$cg/$f" 2>/dev/null || echo unreadable)"; done
    ;;
  burn)
    # Four seconds of spinning, then the CPU time this process and its children took (clock ticks).
    end=$((SECONDS + 4)); while [ $SECONDS -lt $end ]; do :; done
    read -r -a st < /proc/self/stat; echo "cpu_ticks: $(( st[13] + st[14] ))"
    ;;
  *) echo "unsupported probe" ;;
esac
