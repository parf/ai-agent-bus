# What a failed `go vet` or `go test` run says, for smoke.sh and its contract check.
# Reads go's whole output on stdin. First what failed wherever it sits: each failing
# test and package, its file:line lines, a panic or timeout, the first race report
# whole. A run that fails before any test ran (`FAIL ./... [setup failed]`, a build
# error) names no test, and the filter alone would print just that line and lose go's
# own words (J25), so then go's lines are printed too, minus the passing packages.
go_failure_text() {
  local out shown
  out=$(cat)
  shown=$(printf '%s\n' "$out" | awk '
    /^WARNING: DATA RACE/ { race = (races++ == 0) }
    race { print; if (/^==================/) race = 0; next }
    /^[[:space:]]*--- FAIL|^FAIL|^panic:|_test\.go:[0-9]+:|race detected|^running tests:|^[[:space:]]+Test[[:alnum:]_\/]* \(/ { print }' |
    head -80)
  printf '%s\n' "$shown" | sed 's/^/    /'
  printf '%s\n' "$shown" | grep -Eq -- '--- FAIL|^panic:|_test\.go:[0-9]+:|DATA RACE|^running tests:' ||
    printf '%s\n' "$out" | grep -Ev '^ok[[:space:]]|no test files' | head -60 | sed 's/^/    go: /'
}
