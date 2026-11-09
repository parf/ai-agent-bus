# Mutants of ab-w1a/J111-map: each must fail smoke_select_test on an assertion, then the file is restored from HEAD.
import subprocess, os
W = '/usr/local/src/ai-agent-bus/tmp/ab-w1a-J111-map'
M = [('daemon API doc unowned', 'Plans/subsystems.md', ", `docs/09-daemon-api.md` |", " |"),
     ('subsystem table not shared', 'src/smoke-select.py', "'go.sum', 'Plans/subsystems.md')", "'go.sum')"),
     ('guard ignores the reader', 'src/smoke_select_test.py', "else modes[letter] == 'normal'", "else 'normal' in modes.values()"),
     ('docs-check reader not run', 'src/smoke_select_test.py', "'src/agents/agent_docs_test.py', 'src/agents/docs_order_test.py',", "'src/agents/agent_docs_test.py', 'src/agents/docs_order_test.py', 'src/agents/not_in_go_checks.py',"),
     ('stale name-only entry', 'src/smoke_select_test.py', "    NAMES_ONLY = {\n", "    NAMES_ONLY = {\n        ('src/gone.go', 'docs/gone.md'): 'gone',\n")]
env = {k: v for k, v in os.environ.items() if k not in ('AGENT_BUS_SESSION_FILE', 'AGENT_BUS_TOKEN')}
for name, f, a, b in M:
    p = os.path.join(W, f); t = open(p).read()
    assert t.count(a) == 1, name
    open(p, 'w').write(t.replace(a, b))
    try:
        r = subprocess.run(['python3', '-B', '-m', 'unittest', 'smoke_select_test'], cwd=W + '/src', capture_output=True, text=True, env=env, timeout=120)
        fails = [l for l in r.stderr.splitlines() if l.startswith(('FAIL:', 'ERROR:'))]
        print(f'{name}: rc={r.returncode}', fails, 'AssertionError' in r.stderr)
    finally:
        subprocess.run(['git', '-C', W, 'checkout', '-q', 'HEAD', '--', f], check=True)
print(subprocess.run(['git', '-C', W, 'status', '--short'], capture_output=True, text=True).stdout or 'clean')
