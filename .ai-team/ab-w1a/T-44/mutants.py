# T-44 mutants: each must make its test fail on an assertion (a FAIL line), never on a crash.
import subprocess, os, shutil
env = {k: v for k, v in os.environ.items() if k not in ('AGENT_BUS_SESSION_FILE', 'AGENT_BUS_TOKEN')}
env['TREE'] = '/usr/local/src/ai-agent-bus'
G = [('bump accepted', '((bump)) && receipt refused "bump is retired', '((0)) && receipt refused "bump is retired'),
     ('line without stable accepted', "((!stable)) && [[ -n $note ]] && receipt refused", "false && receipt refused")]
D = [('main not required', 'refuse "$REPO is not on main"', 'true'),
     ('dirty file accepted', 'clean() { [[ -z $(g status --porcelain -- "$1") ]] || refuse', 'clean() { true || refuse'),
     ('missing changelog accepted', '[[ -f $REPO/$cl ]] || refuse', 'true || refuse'),
     ('empty window accepted', '((count > 0)) || refuse', '((1)) || refuse'),
     ('summary optional', 'if ((!have_summary)) || [[ -z ${summary//[[:space:]]/} ]]; then', 'if false; then'),
     ('long summary accepted', '|| refuse "$1 has more than two lines"', '|| true'),
     ('blank line accepted', '|| refuse "$1 has a blank line"', '|| true'),
     ('heading twice accepted', '! grep -qE', 'grep -qE'),
     ('failed commit not restored', '  restore\n  echo "deploy-bump failed', '  echo "deploy-bump failed'),
     ('summary through awk -v', 'DB_HEAD=$2 DB_SUMMARY=$3 awk \'\n    !done && /^## / { print ENVIRON["DB_HEAD"]; print ""; print ENVIRON["DB_SUMMARY"];', 'DB_HEAD=$2 DB_SUMMARY=$3 awk -v s="$3" \'\n    !done && /^## / { print ENVIRON["DB_HEAD"]; print ""; print s;'),
     ('commit sweeps the index', 'g commit -q -m "Version $next" -- "${files[@]}"', 'g add "${files[@]}" && g commit -q -m "Version $next"'),
     ('pack never bumped', 'if [[ -n $agents ]]; then', 'if false; then'),
     ('pack bumped without an agents change', 'agents=$(g diff --name-only "$last" HEAD -- src/agents/ ":(exclude)$AV" ":(exclude)$ACL")', 'agents=x'),
     ('pack version files count as a change', 'src/agents/ ":(exclude)$AV" ":(exclude)$ACL")\nanext', 'src/agents/)\nanext'),
     ('agents summary optional', 'if ((!have_asummary)) || [[ -z ${asummary//[[:space:]]/} ]]; then', 'if false; then'),
     ('agents summary unchecked', 'lines "the agents summary" "$asummary"', ':'),
     ('extra agents summary accepted', 'elif ((have_asummary)); then', 'elif false; then'),
     ('window-set pack VERSION bumped again', 'if [[ -n $agents && -n $(g diff --name-only "$last" HEAD -- "$AV") ]]; then', 'if false; then'),
     ('missing pack changelog accepted', '[[ -f $REPO/$ACL ]] || refuse', 'true || refuse')]
def run(name, src, a, b, test, args):
    base = open(src).read(); assert base.count(a) == 1, name
    os.makedirs('mut', exist_ok=True)
    for f in ('gate-lock.sh', 'gate.sh.new', 'deploy-bump.sh'): shutil.copy(f, 'mut/' + f)
    open('mut/' + src, 'w').write(base.replace(a, b))
    r = subprocess.run(['bash', test, *args], capture_output=True, text=True, env=env, timeout=200)
    fails = [l for l in r.stdout.splitlines() if l.startswith('FAIL')]
    print(f'{name}: rc={r.returncode} fails={len(fails)}', fails[:2])
for n, a, b in G: run(n, 'gate.sh.new', a, b, 'gate.test.sh', [os.path.abspath('mut')])
for n, a, b in D: run(n, 'deploy-bump.sh', a, b, 'deploy-bump.test.sh', [os.path.abspath('mut/deploy-bump.sh')])
shutil.rmtree('mut')
