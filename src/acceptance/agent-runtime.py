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
    for relative in ('agents/shared/subscriptions.py', 'agents/shared/subscription_filters.py', 'agents/shared/metadata.py', 'agents/shared/envfile.py', 'agents/shared/mail_oauth.py', 'agents/smtp/support/agents/shared/mail_oauth.py', 'agents/imap/cache.py', 'agents/imap/wire.py', 'agents/imap/budget.py', 'agents/imap/matching.py', 'agents/imap/progress.py', 'agents/imap/outbox.py', 'agents/imap/publication.py', 'agents/imap/forwarding.py', 'agents/imap/mailbox_session.py', 'agents/imap/support/agents/shared/mail.py', 'agents/imap/support/agents/shared/mail_oauth.py', 'agents/imap/support/agents/shared/subscriptions.py', 'agents/imap/support/agents/shared/subscription_filters.py', 'agents/imap/support/clients/python/agent_bus/client.py', 'agents/shared/mail.py', 'agents/smtp/support/agents/shared/mail.py', 'agents/shared/provider_http.py', 'agents/ai/support/agents/shared/provider_http.py', 'agents/voyage/support/agents/shared/provider_http.py', 'agents/qdrant/support/agents/shared/provider_http.py', 'agents/standalone/tmux-sessions/config.json.dist', 'clients/python/agent_bus/client.py',
                     'agents/cron/support/agents/shared/scheduler.py', 'agents/cron/support/agents/shared/cronexpr.py', 'agents/at/support/agents/shared/scheduler.py', 'agents/at/support/agents/shared/cronexpr.py', 'agents/at/support/clients/python/agent_bus/client.py',
                     'agents/standalone/logwatch/logwatch.py'):
        if not (root / relative).is_file():
            raise ValueError('missing runtime artifact: ' + relative)
    for relative in ('agents/eventlog/agent.json', 'agents/eventlog/eventlog.py', 'agents/eventlog/env.dist', 'agents/eventlog/config.json.dist', 'agents/eventlog/LICENSE.md', 'agents/eventlog/licenses/NOTICE.md', 'agents/eventlog/support/agents/shared/metadata.py', 'agents/eventlog/support/agents/shared/event_rings.py', 'agents/eventlog/support/agents/shared/subscriptions.py', 'agents/eventlog/support/agents/shared/subscription_filters.py', 'agents/eventlog/support/clients/LICENSE.md', 'agents/eventlog/support/internal/version/VERSION', 'agents/eventlog/support/clients/python/agent_bus/__init__.py', 'agents/eventlog/support/clients/python/agent_bus/_version.py', 'agents/eventlog/support/clients/python/agent_bus/client.py', 'agents/eventlog/support/clients/python/agent_bus/errors.py', 'agents/eventlog/support/clients/python/agent_bus/message.py', 'agents/eventlog/support/clients/python/agent_bus/storage.py', 'agents/eventlog/support/clients/python/agent_bus/transport.py', 'agents/shared/event_rings.py'):
        if not (root / relative).is_file():
            raise ValueError('missing runtime artifact: ' + relative)
    rag_files = ('agents/rag/rag.py', 'agents/rag/rules.py', 'agents/rag/core.py', 'agents/rag/chunks.py', 'agents/rag/prompts.py', 'agents/rag/blocks.py', 'agents/rag/support/agents/shared/metadata.py', 'agents/rag/support/clients/python/agent_bus/__init__.py', 'agents/rag/support/clients/python/agent_bus/_version.py', 'agents/rag/support/clients/python/agent_bus/client.py', 'agents/rag/support/clients/python/agent_bus/errors.py', 'agents/rag/support/clients/python/agent_bus/message.py', 'agents/rag/support/clients/python/agent_bus/storage.py', 'agents/rag/support/clients/python/agent_bus/transport.py')
    for relative in rag_files:
        if not (root / relative).is_file():
            raise ValueError('missing runtime artifact: ' + relative)
    for relative in ('agents/webhook/incoming.py', 'agents/webhook/outgoing.py', 'agents/webhook/rules.py', 'agents/webhook/support/agents/shared/envfile.py', 'agents/webhook/support/agents/shared/metadata.py', 'agents/webhook/support/clients/python/agent_bus/__init__.py', 'agents/webhook/support/clients/python/agent_bus/_version.py', 'agents/webhook/support/clients/python/agent_bus/client.py', 'agents/webhook/support/clients/python/agent_bus/errors.py', 'agents/webhook/support/clients/python/agent_bus/message.py', 'agents/webhook/support/clients/python/agent_bus/storage.py', 'agents/webhook/support/clients/python/agent_bus/transport.py', 'agents/webhook/webhook.py'):
        if not (root / relative).is_file():
            raise ValueError('missing runtime artifact: ' + relative)
    for relative in ('agents/fetch/fetch.py', 'agents/fetch/worker.py', 'agents/fetch/policy.py', 'agents/fetch/transport.py', 'agents/fetch/extract.py', 'agents/fetch/support/agents/shared/metadata.py'):
        if not (root / relative).is_file():
            raise ValueError('missing runtime artifact: ' + relative)
    env = {k: v for k, v in os.environ.items() if not k.startswith(('AGENT_BUS_', 'AB_ROLE_', 'PYTHON'))}
    env.update(PYTHONDONTWRITEBYTECODE='1', AB_ROLE_OWNER='1')
    version = (root / 'internal/version/VERSION').read_text().strip()
    assert 'PolyForm Noncommercial License 1.0.0' in (root / 'agents/LICENSE.md').read_text()
    assert 'MIT License' in (root / 'clients/LICENSE.md').read_text()
    assert (root / 'agents/licenses/NOTICE.md').is_file()

    def run(argv, cwd, extra=None, status=0):
        p = subprocess.run(argv, cwd=cwd, env=env | (extra or {}), capture_output=True, text=True, timeout=15)
        assert p.returncode == status, f"{argv[0]} runtime failed: {p.stderr}"
        return p.stdout, p.stderr

    code = "import sys; sys.path.insert(0,sys.argv[1]); import subscriptions,subscription_filters as f; assert f.matches(f.compile_filter('/^ok$/'), 'ok'); assert hasattr(subscriptions, 'Subscriptions')"
    run(['python3', '-I', '-c', code, str(root / 'agents/shared')], root)
    print('subscriptions: exported engine, regex helper and client imports loaded')

    eventlog = root / 'agents/eventlog'
    output, _ = run(['python3', str(eventlog / 'eventlog.py')], root,
                    {'AGENT_BUS_ASK':'help-json', 'AGENT_BUS_CONFIG':'invalid'})
    metadata = json.loads(output)
    assert set(metadata['methods']) == {'help'}
    assert metadata['hidden_methods']['count'] == 9
    assert metadata['secrets'] == [] and metadata['body_formats'] == ['text', 'json']
    assert metadata['docs'].endswith('/eventlog.md')
    assert json.loads((eventlog / 'config.json.dist').read_text()) == {'rings':{},'logs':{},'max_logs':100}
    for shared in ('event_rings.py','subscriptions.py','subscription_filters.py'):
        assert (eventlog / 'support/agents/shared' / shared).read_bytes() == (root / 'agents/shared' / shared).read_bytes(), 'eventlog engine copy'
    print('eventlog: offline metadata, private config example and shared engine copies loaded')

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

    rag = root / 'agents/rag'
    output, _ = run(['python3', str(rag / 'rag.py')], root, {'AGENT_BUS_ASK':'help-json'})
    rag_help = json.loads(output)
    rag_manifest = json.loads((rag / 'agent.json').read_text())
    assert rag_help['version'] == rag_manifest['version'], 'RAG own app version lost'
    assert set(rag_help['methods']) == {'upsert','get','list','delete','prompts'}
    assert (rag / 'config.json.dist').is_file() and (rag / 'CHANGELOG.md').is_file()
    print('rag: app version, client/helper imports and JSON methods loaded without live blocks')

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
    fetch = root / 'agents/fetch'
    output, _ = run(['python3', str(fetch / 'fetch.py')], root, {'AGENT_BUS_ASK':'help-json', 'AGENT_BUS_CONFIG':'invalid'})
    metadata = json.loads(output)
    assert set(metadata['methods']) == {'get','curl'}
    assert metadata['version'] == version
    assert metadata['docs'].endswith('/fetch.md')
    assert set(metadata['config']) == {'allow'} and metadata['config']['allow']['type'] == 'list' and metadata['secrets'] == []
    print('fetch: stateless get/curl package and allowlist metadata loaded without any network call')

    webhook = root / 'agents/webhook'
    output, _ = run(['python3', str(webhook / 'webhook.py')], root, {'AGENT_BUS_ASK':'help-json', 'WEBHOOK_LISTEN':'invalid'})
    metadata = json.loads(output)
    assert set(metadata['methods']) == {'post','routes'}
    assert metadata['config']['(root)']['type'] == 'list'
    assert metadata['version'] == version
    assert json.loads((webhook / 'agent.json').read_text())['serves'] == 'itself'
    print('webhook: kept route-list package and shared client/envfile load without opening a listener')

    imap = root / 'agents/imap'
    output, _ = run(['python3', str(imap / 'imap.py')], root, {'AGENT_BUS_ASK':'help-json'})
    metadata = json.loads(output)
    assert metadata['body_formats'] == ['text', 'json']
    assert set(metadata['methods']) == {'sync', 'list', 'search', 'read', 'publish', 'help', 'sub add', 'sub list', 'sub delete', 'status', 'mark'}
    assert metadata['docs'].endswith('/imap.md')
    assert all(not item.get('has_default') for item in metadata['secrets'])
    assert json.loads((imap / 'agent.json').read_text())['serves'] == 'itself'
    print('imap: kept-reader package, cache, protocol and client load without credentials')

    smtp = root / 'agents/smtp'
    output, _ = run(['python3', str(smtp / 'smtp.py')], root, {'AGENT_BUS_ASK':'help-json'})
    metadata = json.loads(output)
    assert metadata['body_formats'] == ['json']
    assert 'send' in metadata['methods']
    assert metadata['docs'].endswith('/smtp.md')
    assert all(not item.get('has_default') for item in metadata['secrets'])
    print('smtp: packaged mail/metadata loaded without credentials or a server connection')

    for name, methods in (('cron', {'sub.add', 'sub.list', 'sub.delete', 'later', 'later.list', 'later.cancel', 'help'}),
                          ('at', {'in', 'at', 'list', 'cancel', 'help'})):
        package = root / 'agents' / name
        output, _ = run(['python3', str(package / (name + '.py'))], root, {'AGENT_BUS_ASK':'help-json'})
        metadata = json.loads(output)
        assert set(metadata['methods']) == methods, name + ' methods'
        assert metadata['docs'].endswith('/' + name + '.md')
        assert json.loads((package / 'agent.json').read_text())['serves'] == 'itself'
        for shared in ('scheduler.py', 'cronexpr.py'):
            assert (package / 'support/agents/shared' / shared).read_bytes() == (root / 'agents/shared' / shared).read_bytes(), name + ' engine copy'
    print('cron, at: kept packages with the shared scheduler engine load without a bus')

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
        bundle = entry.read_text()
        assert 'class Subscriptions' in bundle and 'filter_invalid' in bundle, agent + ': shared subscription engine missing'
        assert not (directory / 'filter-worker.js').exists(), agent + ': obsolete local filter worker present'
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
