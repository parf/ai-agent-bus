#!/bin/bash
# K.17 SQLite backup/restore acceptance (docs/09-setup.md#backup-and-restore).
# One disposable real-systemd host is populated through the 0.7 CLI, backed up
# twice, and has the older backup restored over newer state; a second, fresh
# host installs the same archive and has the newer backup restored onto it,
# then takes a graceful restart and a bus-child crash. The hosts see only the
# archive, this fixture and the shared evidence directory.
# Usage: backup-restore.sh <0.7-archive> <new-output-directory>
# BACKUP_MUTANT=hot|keepwal breaks one procedure step to show its check fails.
set -euo pipefail
archive=$(realpath "${1:?0.7 release archive required}")
out=$(realpath -m "${2:?new evidence directory required}")
[ -f "$archive" ] && [ -f "$archive.sha256" ] || { echo "no archive or checksum: $archive" >&2; exit 1; }
[ ! -e "$out" ] || { echo "output directory must be new" >&2; exit 1; }
mkdir -m 700 -p "$out/package" "$out/fixture" "$out/shared" "$out/source" "$out/restore"
cp "$archive" "$archive.sha256" "$out/package/"
cp "$(dirname "$0")/backup-restore-container.sh" "$out/fixture/run.sh"

image=${FRESH_INSTALL_IMAGE:-localhost/agent-bus-fresh-install:arch-systemd}
podman build --pull=never -t "$image" -f "$(dirname "$0")/fresh-install.Containerfile" "$(dirname "$0")" >"$out/image-build.log"
names=()
cleanup() { for n in "${names[@]}"; do podman stop -t 10 "$n" >/dev/null 2>&1 || true; done; }
trap cleanup EXIT
host() {
  local phase=$1 name="agent-bus-backup-$1-$RANDOM-$$" state
  names+=("$name")
  podman run -d --rm --pull=never --privileged --systemd=always --network=none \
    --security-opt label=disable --name "$name" -e BACKUP_MUTANT="${BACKUP_MUTANT:-}" \
    -v "$out/package:/package:ro" -v "$out/fixture:/fixture:ro" \
    -v "$out/shared:/shared:rw" -v "$out/$phase:/evidence:rw" "$image" >/dev/null
  for _ in $(seq 1 100); do
    state=$(podman exec "$name" systemctl is-system-running 2>/dev/null || true)
    case "$state" in running|degraded) break ;; esac
    sleep .1
  done
  case ${state:-} in running|degraded) ;; *) echo "systemd did not start: ${state:-unknown}" >&2; return 1 ;; esac
  podman exec "$name" bash /fixture/run.sh "$phase" | tee "$out/$phase/result.log"
  local rc=${PIPESTATUS[0]}
  podman stop -t 10 "$name" >/dev/null 2>&1 || true
  return "$rc"
}
host source
host restore
