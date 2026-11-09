# T-44 on top of J111: gate.sh.new = ../J111/gate.sh.new with "bump" retired (TD6).
t = open('../J111/gate.sh.new').read()
def rep(a, b):
    global t
    assert t.count(a) == 1, a[:70]; t = t.replace(a, b)
rep("""# Body: <branch> [T-n] [bump | stable] [-- line]
# Steps: rebase the branch on main in tmp/gate, take the smoke-gate lock, run
# src/smoke.sh --normal (one rerun on red), bump PATCH + changelog into the tip
# commit only when the lead marked it "bump", fast-forward main. Main moving
# underneath restarts the cycle once. Always exits 0: stdout is the receipt.
# The changelog is CHANGELOG.<major>.<minor>.md of main's VERSION.""",
"""# Body: <branch> [T-n] [stable -- line]
# Steps: rebase the branch on main in tmp/gate, take the smoke-gate lock, run
# src/smoke.sh --normal (one rerun on red), fast-forward main. Main moving
# underneath restarts the cycle once. Always exits 0: stdout is the receipt.
# A landing never bumps (TD6, Plans/Team.v2/DECISIONS.md): #ab-ceo raises PATCH
# once per deploy with .ai-team/deploy-bump.sh, so a body with "bump" is refused,
# and so is "-- line" without stable.""")
rep("""# "STABLE - passed all tests and reviews". It cannot be combined with "bump".""",
    """# "STABLE - passed all tests and reviews".""")
rep("""[[ -n ${branch:-} ]] || { echo "gate refused: empty body; send '<branch> [T-n] [bump | stable] [-- line]'"; exit 0; }""",
"""[[ -n ${branch:-} ]] || { echo "gate refused: empty body; send '<branch> [T-n] [stable -- line]'"; exit 0; }
((bump)) && receipt refused "bump is retired: #ab-ceo bumps at deploy"
((!stable)) && [[ -n $note ]] && receipt refused "'-- line' goes only with stable: the changelog is written at deploy\"""")
rep("""((bump && stable)) && receipt refused "stable does not take bump: the branch carries its own VERSION and changelog"
""", "")
i = t.index('  local ver="no bump"\n  if ((bump)); then'); j = t.index('  elif ((stable)); then', i)
t = t[:i] + '  local ver="no bump"\n  if ((stable)); then' + t[j + len('  elif ((stable)); then'):]
open('gate.sh.new', 'w').write(t)
