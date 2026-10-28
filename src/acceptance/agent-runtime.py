#!/usr/bin/env python3
"""Exercise an exported agent tree with no checkout on its import path."""
import json
import os
from pathlib import Path
import subprocess
import sys


def check(root):
    root = Path(root).resolve()
    # Negative fixtures remove these imports on purpose. Refuse before a child
    # interpreter raises an import exception that the host crash reporter sees.
    for relative in ('agents/shared/metadata.py', 'agents/shared/envfile.py', 'agents/shared/provider_http.py', 'agents/ai/support/agents/shared/provider_http.py', 'agents/voyage/support/agents/shared/provider_http.py', 'agents/qdrant/support/agents/shared/provider_http.py', 'agents/standalone/tmux-sessions/config.json.dist', 'clients/python/agent_bus/client.py',
                     'agents/standalone/logwatch/logwatch.py'):
        if not (root / relative).is_file():
            raise ValueError('missing runtime artifact: ' + relative)
    env = {k: v for k, v in os.environ.items() if not k.startswith(('AGENT_BUS_', 'AB_ROLE_', 'PYTHON'))}
    env.update(PYTHONDONTWRITEBYTECODE='1', AB_ROLE_OWNER='1')
    version = (root / 'internal/version/VERSION').read_text().strip()
    assert 'PolyForm Noncommercial License 1.0.0' in (root / 'agents/LICENSE.md').read_text()
    assert 'MIT License' in (root / 'clients/LICENSE.md').read_text()
    assert (root / 'agents/NOTICE.md').is_file()

    def run(argv, cwd, extra=None, status=0):
        p = subprocess.run(argv, cwd=cwd, env=env | (extra or {}), capture_output=True, text=True, timeout=15)
        assert p.returncode == status, f"{argv[0]} runtime failed: {p.stderr}"
        return p.stdout, p.stderr

    for agent in ('systemd', 'server-info', 'git', 'logwatch', 'tmux-sessions'):
        directory = root / 'agents/standalone' / agent
        script = directory / {'git': 'git_agent.py', 'logwatch': 'logwatch.py', 'systemd': 'systemd.py', 'server-info': 'server-info.py', 'tmux-sessions': 'sessions.py'}[agent]
        if agent == 'logwatch':
            # Load the whole native service, including the shipped Python client,
            # then ask its real handler without opening a bus connection.
            code = "import runpy,sys,types; m=runpy.run_path(sys.argv[1],run_name='runtime_check'); print(m['Agent'].handle(None,types.SimpleNamespace(ask='help-json')))"
            output, _ = run(['python3', '-c', code, str(script)], directory)
        else:
            extra = {'AGENT_BUS_ASK': 'help-json'}
            if agent == 'git':
                # Ask for the full grammar with an explicit commit-cap fixture;
                # native help correctly omits writes when no repos are configured.
                extra['AGENT_BUS_CONFIG'] = json.dumps({'repos':{'fixture':{'path':str(root),'access':'commit'}}})
            output, _ = run(['python3', str(script)], directory, extra)
        data = json.loads(output)
        manifest = json.loads((directory / 'agent.json').read_text())
        assert set(data['methods']) == set(manifest['methods']), agent + ' methods missing'
        assert data['version'] == version, agent + ' wrong version'
        assert 'config' in data and 'secrets' in data, agent + ' incomplete help'
        installer = directory / ('install.sh' if agent == 'server-info' else 'install.py')
        run([str(installer), '--help'], root)
        print(agent + ': native help, client/helper imports and installer loaded')

    ai = root / 'agents/ai'
    output, _ = run(['python3', str(ai / 'ai.py')], root,
                    {'OPENROUTER_API_KEY': ''}, status=78)
    assert output.strip() == 'error: OPENROUTER_API_KEY is not set', 'AI entry failed outside checkout'
    assert (ai / 'agent.json').is_file() and (ai / 'env.dist').is_file()
    print('ai: runner entry loaded without credentials')
    voyage = root / 'agents/voyage'
    output, _ = run(['python3', str(voyage / 'voyage.py')], root, {'VOYAGE_API_KEY':''}, status=78)
    assert output.strip() == 'error: VOYAGE_API_KEY is not set', 'Voyage entry failed outside checkout'
    output, _ = run(['python3', str(voyage / 'voyage.py')], root, {'AGENT_BUS_ASK':'help-json'})
    assert set(json.loads(output)['methods']) == {'embed', 'request'}
    print('voyage: packaged helper and metadata loaded without credentials')
    qdrant = root / 'agents/qdrant'
    output, _ = run(['python3', str(qdrant / 'qdrant.py')], root, {'QDRANT_API_KEY':''}, status=78)
    assert output.strip() == 'error: QDRANT_API_KEY is not set', 'Qdrant entry failed outside checkout'
    output, _ = run(['python3', str(qdrant / 'qdrant.py')], root, {'AGENT_BUS_ASK':'help-json'})
    data = json.loads(output)
    assert set(data['methods']) == {'request'} and set(data['config']) == {'url'}
    assert 'default' not in data['config']['url'], 'Qdrant must have no localhost default'
    assert json.loads((qdrant / 'agent.json').read_text())['roles'] == ['read', 'write', 'admin']
    print('qdrant: packaged helper/metadata loaded without credentials or a default URL')
    sms = root / 'agents/sms-telnyx'
    output, _ = run(['python3', str(sms / 'sms.py')], root, {'TELNYX_API_KEY': ''}, status=78)
    assert output.strip() == 'error: TELNYX_API_KEY is not set', 'SMS entry failed outside checkout'
    assert (sms / 'agent.json').is_file() and (sms / 'env.dist').is_file()
    print('sms: runner entry loaded without credentials')

    output, _ = run(['python3', str(root / 'agents/debug/debug.py'), 'hello release'], root)
    assert output.strip() == 'hello, release'
    code = "import runpy,sys,types; m=runpy.run_path(sys.argv[1],run_name='runtime_check'); print(m['handle'](None,types.SimpleNamespace(ask='help-json',roles=[])))"
    output, _ = run(['python3', '-c', code, str(root / 'agents/debug/debug.py')], root)
    data = json.loads(output)
    assert 'selftest' in data['methods'] and data['version'] == version, 'debug packaged handler/helper/version missing'
    for agent in ('slack', 'telegram'):
        directory = root / 'agents' / agent
        manifest = json.loads((directory / 'agent.json').read_text())
        entry = directory / manifest['command'].removeprefix('./')
        # Every module must load before the entry point can refuse an absent
        # identity. No credential or external Slack/Telegram connection is used.
        _, error = run(['bun', str(entry)], directory, status=1)
        assert agent + ': AGENT_BUS_NAME is not set' in error, agent + ': bundle failed before entry point: ' + error
        assert (directory / 'filter-worker.js').is_file(), agent + ': filter worker missing'
        assert (directory / 'env.dist').is_file(), agent + ': Secret descriptions missing'
        print(agent + ': standalone bundle loaded')
    print('debug: hello release answered')


if __name__ == '__main__':
    try:
        check(sys.argv[1])
    except Exception as error:
        # Negative packaging fixtures fail by status, never an ABRT traceback.
        print('agent runtime check failed: ' + str(error), file=sys.stderr)
        sys.exit(1)
