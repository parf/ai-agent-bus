#!/usr/bin/env python3
"""Exercise an exported agent tree with no checkout on its import path."""
import json
import os
from pathlib import Path
import subprocess
import sys


def check(root):
    root = Path(root).resolve()
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

    for agent in ('systemd', 'server-info', 'git', 'logwatch'):
        directory = root / 'agents/standalone' / agent
        script = directory / {'git': 'git_agent.py', 'logwatch': 'logwatch.py', 'systemd': 'systemd.py', 'server-info': 'server-info.py'}[agent]
        if agent == 'logwatch':
            # Load the whole native service, including the shipped Python client,
            # then ask its real handler without opening a bus connection.
            code = "import runpy,sys,types; m=runpy.run_path(sys.argv[1],run_name='runtime_check'); print(m['Agent'].handle(None,types.SimpleNamespace(ask='help-json')))"
            output, _ = run(['python3', '-c', code, str(script)], directory)
        else:
            output, _ = run(['python3', str(script)], directory, {'AGENT_BUS_ASK': 'help-json'})
        data = json.loads(output)
        manifest = json.loads((directory / 'agent.json').read_text())
        assert set(data['methods']) == set(manifest['methods']), agent + ' methods missing'
        assert data['version'] == version, agent + ' wrong version'
        assert 'config' in data and 'secrets' in data, agent + ' incomplete help'
        installer = directory / ('install.sh' if agent == 'server-info' else 'install.py')
        run([str(installer), '--help'], root)
        print(agent + ': native help, client/helper imports and installer loaded')

    output, _ = run(['python3', str(root / 'agents/debug/debug.py'), 'hello release'], root)
    assert output.strip() == 'hello, release'
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
    check(sys.argv[1])
