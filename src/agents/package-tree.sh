# Sourced by native agent packagers. Build public code in a sibling directory,
# then replace the installed tree: an obsolete module must not shadow a new one.
# Runtime state belongs in the runner's work directory, never in this tree.
# Python 3 is already the native agents' runtime and gives us the same
# non-nesting rename on Linux and macOS (mv -T is not portable).
package_rename() {
  python3 -c 'import os, sys; os.rename(sys.argv[1], sys.argv[2])' "$1" "$2"
}

package_cleanup() {
  package_rc=$?
  trap - EXIT HUP INT TERM
  if [ -n "$package_backup" ] && [ -d "$package_backup/previous" ] && [ ! -e "$package_target" ]; then
    package_rename "$package_backup/previous" "$package_target" || {
      echo "package rollback failed; previous tree kept in $package_backup/previous" >&2
      exit 1
    }
  fi
  [ -z "$package_stage" ] || rm -rf -- "$package_stage"
  [ -z "$package_backup" ] || rm -rf -- "$package_backup"
  exit "$package_rc"
}

package_begin() {
  case $1 in /*) package_target=${1%/} ;; *) package_target=$PWD/${1%/} ;; esac
  case ${package_target##*/} in ''|.|..) echo 'package destination must name an agent directory' >&2; return 1 ;; esac
  if [ -L "$package_target" ] || { [ -e "$package_target" ] && [ ! -d "$package_target" ]; }; then
    echo 'package destination must be a directory, never a symlink' >&2
    return 1
  fi
  mkdir -p "$(dirname "$package_target")"
  package_stage=$(mktemp -d "$package_target.new.XXXXXX")
  package_backup=
  trap package_cleanup EXIT
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM
  dest=$package_stage
}

package_commit() {
  # mktemp starts private; the completed package is public runner-readable code.
  chmod 0755 "$package_stage"
  if [ -e "$package_target" ]; then
    package_backup=$(mktemp -d "$package_target.old.XXXXXX")
    package_rename "$package_target" "$package_backup/previous"
  fi
  package_rename "$package_stage" "$package_target"
  package_stage=
  trap - EXIT HUP INT TERM
  [ -z "$package_backup" ] || rm -rf -- "$package_backup"
}
