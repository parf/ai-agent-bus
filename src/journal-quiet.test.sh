#!/bin/bash
# journal-quiet.sh against a stub journalctl, never the host journal: an entry for the pattern made while the command
# ran fails, one for another name or none passes with the command's own status, an unreadable journal fails.
set -u
here=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$here/../tmp"
work=$(mktemp -d "$here/../tmp/journal-quiet-test.XXXXXX")
trap 'rm -rf "$work"' EXIT
fail=0
bad() { echo "FAIL $*"; fail=1; }
mkdir "$work/bin"
# The stub: a cursor unless STUB_NO_CURSOR; after the cursor, the entries the command appended to $STUB_JOURNAL.
cat >"$work/bin/journalctl" <<'EOF'
#!/bin/bash
case " $* " in
  *" --show-cursor "*) [ -n "${STUB_NO_CURSOR:-}" ] || echo '-- cursor: s=stub'; [ -z "${STUB_CURSOR_FAILS:-}" ] || exit 125 ;;
  *" --after-cursor "*) [ -z "${STUB_READ_FAILS:-}" ] || { echo 'Failed to seek to cursor' >&2; exit 125; }; cat "$STUB_JOURNAL" 2>/dev/null ;;
esac
EOF
chmod +x "$work/bin/journalctl"
export PATH=$work/bin:$PATH STUB_JOURNAL=$work/journal
q() { : >"$STUB_JOURNAL"; out=$(bash "$here/journal-quiet.sh" "$@" 2>&1); rc=$?; }

q ab-db-fixture sh -c 'printf "    CONTAINER_NAME=ab-db-fixture-1-2-3\n    MESSAGE=ERROR: cannot insert multiple commands\n" >>"$STUB_JOURNAL"'
[ "$rc" = 3 ] && grep -q 'gained entries for ab-db-fixture' <<<"$out" || bad "a container's journal line passed: rc $rc $out"
q ab-db-fixture sh -c 'printf "    PODMAN_EVENT=create\n    PODMAN_NAME=ab-db-fixture-1-2-3\n" >>"$STUB_JOURNAL"'
[ "$rc" = 3 ] || bad "a podman event passed: rc $rc $out"
q ab-db-fixture sh -c 'printf "    CONTAINER_NAME=somebody-else\n" >>"$STUB_JOURNAL"; exit 7'
[ "$rc" = 7 ] && grep -q 'no journal entry for ab-db-fixture' <<<"$out" || bad "another name's line or the command's status was mishandled: rc $rc $out"
q ab-db-fixture true
[ "$rc" = 0 ] || bad "a silent run failed: rc $rc $out"
STUB_NO_CURSOR=1 q ab-db-fixture true
[ "$rc" = 2 ] || bad "an unreadable journal passed: rc $rc $out"
STUB_CURSOR_FAILS=1 q ab-db-fixture sh -c 'echo ran >"$STUB_JOURNAL.ran"'
[ "$rc" = 2 ] && [ ! -e "$STUB_JOURNAL.ran" ] || bad "a cursor read that printed a cursor and failed passed: rc $rc $out"
STUB_READ_FAILS=1 q ab-db-fixture true
[ "$rc" = 2 ] && grep -q 'after the run failed' <<<"$out" || bad "a journal that cannot be read after the run passed: rc $rc $out"
exit $fail
