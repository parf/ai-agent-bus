# Keep a failed Bun run whole: its diff and source location precede (fail),
# and a fixed context window can cut either one off. Passing runs stay terse.
bun_test_text() {
  local rc=$1
  if [ "$rc" != 0 ]; then
    cat
  else
    grep -E '^\(fail\)| pass$| fail$|^error:|^FILE_TIMING'
  fi | sed 's/^/  /'
}
