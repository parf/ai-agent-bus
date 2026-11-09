"""AS.1 5deb95e8/d42308ce runner mutants: each must compile and fail the pack protocol tests on an assertion; files restored after each."""
import subprocess, os, sys, pathlib
SRC = pathlib.Path('/usr/local/src/ai-agent-bus/tmp/ab-w1a-as1r/src')
F = 'cmd/agent-bus-runner/pack_protocol.go'
M = [
 ('daemon below the pack minimum accepted', F, 'if daemonMin < protocolMin || daemon > protocol {', 'if daemon > protocol {'),
 ('daemon above the pack maximum accepted', F, 'if daemonMin < protocolMin || daemon > protocol {', 'if daemonMin < protocolMin {'),
 ('unusable identity checks nothing', F, '\tdaemon, daemonMin, err := daemonRange(identity)\n\tif err != nil {\n\t\treturn "pack protocol unsupported: " + err.Error()', '\tdaemon, daemonMin, err := daemonRange(identity)\n\tif err != nil {\n\t\treturn ""'),
 ('a package with the pack marker is not checked', F, '\tif _, err := os.Stat(filepath.Join(support, "VERSION")); err == nil {', '\tif _, err := os.Stat(filepath.Join(support, "VERSION")); err == nil && false {'),
 ('every package counts as the pack', F, '\tif _, err := os.Stat(filepath.Join(support, "VERSION")); err == nil {', '\tif true {'),
 ('missing declaration loads', F, '\t\t\treturn 0, 0, errors.New("protocol.json: missing")', '\t\t\treturn 1, 1, nil'),
 ('a 4xx identity answer checks nothing', F, '\tif code >= 400 && code < 500 {\n\t\treturn nil, &identityRefused{code}\n\t}\n', ''),
 ('a null identity field reads as zero', F, ' || string(raw) == "null" ||', ' ||'),
 ('load never checks', 'cmd/agent-bus-runner/main.go', '\tr.checkPackProtocols(list)\n', ''),
]
env = {k: v for k, v in os.environ.items() if k not in ('AGENT_BUS_SESSION_FILE', 'AGENT_BUS_TOKEN')}
env['GOMAXPROCS'] = '6'
bad = 0
for name, f, old, new in M:
    p = SRC / f; src = p.read_text()
    assert src.count(old) == 1, name
    p.write_text(src.replace(old, new))
    try:
        r = subprocess.run(['go', 'test', '-count=1', '-p', '4', '-run', 'Pack|TestOnlyPack|TestAnUnusable|TestADaemonProtocol|TestAMissing|TestAFourHundred|TestANullField', './cmd/agent-bus-runner'], cwd=SRC, env=env, capture_output=True, text=True, timeout=300)
        out = r.stdout + r.stderr
        build = '[build failed]' in out or 'setup failed' in out
        failed = [l.strip() for l in out.splitlines() if l.strip().startswith('--- FAIL')]
        killed = r.returncode != 0 and not build and failed
        print(('KILLED ' if killed else 'SURVIVED ') + name + (' (BUILD FAILED)' if build else '') + ' :: ' + '; '.join(failed[:3]))
        bad += not killed
    finally:
        p.write_text(src)
sys.exit(1 if bad else 0)
