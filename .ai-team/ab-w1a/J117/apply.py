# J117 on top of the installed gate: gate.sh.new = .ai-team/gate.sh with the agent-pack-only path.
t = open('../../gate.sh').read()
def rep(a, b):
    global t
    assert t.count(a) == 1, a[:70]; t = t.replace(a, b)
rep("""# "STABLE - passed all tests and reviews".""",
"""# "STABLE - passed all tests and reviews".
# An agent-pack-only branch (J117) runs the same normal gate, which then runs only
# the checks that own a changed path and the pack contract (the selector's "agent
# pack only" rule, CLAUDE.md § verification). The gate reads that answer to skip
# the Go warm-up and to name those checks in the receipt; any selector error or
# other answer is an ordinary normal landing.""")
rep("""  else
    check=normal
""", """  else
    check=normal
    # Agent pack only: the normal selection's own answer, never a guess from paths.
    local agents_only=0 asel
    if asel=$(cd "$WT" && python3 src/smoke-select.py --mode normal --out "$log.normal" 2>&1) \\
      && grep -qx '  agent pack only: checks that own no changed path are skipped' <<<"$asel"; then
      agents_only=1
      check="agents only ($(awk -F '\\t' '$2 != "skip" { print $1 }' "$log.normal" | sort | paste -sd' '))"
    fi
    rm -f "$log.normal"
    printf '== normal selection\\n%s\\n' "$asel" >>"$log"
""")
rep("""    # Warm Go's build cache for the packages the fast shards test, untimed: a cold""",
"""    # Warm Go's build cache for the packages the fast shards test, untimed: a cold""")
i = t.index("    (cd \"$WT/src\" && export GOCACHE=")
j = t.index("| xargs -r go test -run '^$') >>\"$log\" 2>&1 || true\n", i) + len("| xargs -r go test -run '^$') >>\"$log\" 2>&1 || true\n")
block = t[i:j]
t = t[:i] + "    # An agent-pack-only run has no Go check to warm.\n    ((agents_only)) || " + block.lstrip() + t[j:]
open('gate.sh.new', 'w').write(t)
