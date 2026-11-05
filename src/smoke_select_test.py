"""The selection contract, without building or running any runtime fixture."""
import importlib.util
from pathlib import Path
import re
import unittest

spec = importlib.util.spec_from_file_location('select_checks', Path(__file__).with_name('smoke-select.py'))
selector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(selector)
ROOT = Path(__file__).resolve().parent.parent

class Selection(unittest.TestCase):
    def plan(self, paths, mode='normal', targets=''):
        return selector.plan(ROOT, mode, targets, paths)

    def test_one_agent_runs_only_its_own_checks_and_the_pack_contract(self):
        for mode in ('normal', 'fast'):
            p = self.plan(['src/agents/telegram/receive.ts'], mode)
            self.assertTrue(p['agents_only'])
            self.assertEqual(p['checks']['agent_telegram'], mode)
            self.assertEqual(p['checks']['agent_pack'], mode)
            running = {name for name, level in p['checks'].items() if level != 'skip'}
            self.assertEqual(running, {'agent_telegram', 'agent_pack'}, mode)

    def test_a_shared_agent_helper_runs_the_agents_that_use_it_and_no_other(self):
        p = self.plan(['src/agents/shared/mail.py'])
        running = {name for name, level in p['checks'].items() if level != 'skip'}
        self.assertEqual(running, {'agent_smtp', 'agent_imap', 'agent_pack'})

    def test_a_daemon_or_runner_change_runs_the_agents_at_fast_only(self):
        for path in ('src/internal/core/core.go', 'src/internal/protocol/envelope.go',
                     'src/cmd/agent-bus-runner/serve.go', 'src/package.sh'):
            with self.subTest(path=path):
                p = self.plan([path])
                self.assertFalse(p['agents_only'])
                self.assertEqual(p['modes']['L'], 'fast')
                agents = {level for name, level in p['checks'].items() if name.startswith('agent_') and name != 'agent_guide'}
                self.assertEqual(agents - {'skip'}, {'fast'}, path)

    def test_imap_changes_include_bundle_and_runtime_contract_checks(self):
        for path in ('imap.py', 'agent.json', 'folders.py', 'package.sh'):
            with self.subTest(path=path):
                plan = self.plan(['src/agents/imap/' + path])
                self.assertEqual(plan['checks']['agent_imap'], 'normal')
                self.assertEqual(plan['checks']['agent_pack'], 'normal')
                for shard in ('go_checks', 'go_unit', 'go_release', 'agent_slack'):
                    self.assertEqual(plan['checks'][shard], 'skip', shard)

    def test_every_bundled_manifest_selects_the_runner_reload_gate(self):
        import json
        manifests = list((ROOT / 'src/agents').rglob('agent.json'))
        manifests = [path for path in manifests if 'node_modules' not in path.parts]
        self.assertTrue(manifests, 'no bundled manifest was checked')
        for path in manifests:
            rel = path.relative_to(ROOT).as_posix()
            with self.subTest(path=rel):
                self.assertEqual(self.plan([rel], 'fast')['checks']['agent_pack'], 'fast')
                self.assertEqual(self.plan([rel], 'normal')['checks']['agent_pack'], 'normal')
        shard = (ROOT / 'src/agents/smoke.sh').read_text()
        self.assertIn("./cmd/agent-bus-runner -run '^TestEveryBundledManifestPassesRunnerReload$'", shard)

    def test_every_agent_manifest_selects_setup_runtime_checks_with_its_reason(self):
        manifests = sorted((ROOT / 'src/agents').rglob('agent.json'))
        manifests = [p.relative_to(ROOT).as_posix() for p in manifests if 'node_modules' not in p.parts]
        self.assertIn('src/agents/rag/agent.json', manifests)
        for path in manifests:
            with self.subTest(path=path):
                plan = self.plan([path], 'normal')
                self.assertEqual(plan['checks']['agent_pack'], 'normal', path + ': setup runtime tests omitted')
                self.assertEqual(plan['reasons']['L'], 'changed: ' + path)

    def test_rag_manifest_selection_output_names_the_reason(self):
        import shutil, subprocess, tempfile
        (ROOT/'tmp').mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=ROOT/'tmp') as directory:
            repo = Path(directory)
            for relative in ('src/smoke-select.py','src/smoke-map.json','Plans/subsystems.md'):
                target = repo/relative
                target.parent.mkdir(parents=True,exist_ok=True)
                shutil.copy(ROOT/relative,target)
            manifest = repo/'src/agents/rag/agent.json'
            manifest.parent.mkdir(parents=True)
            manifest.write_text('{}\n')
            def git(*args):
                subprocess.run(['git','-C',str(repo),'-c','core.hooksPath=/dev/null','-c','commit.gpgSign=false',*args],check=True,capture_output=True,timeout=10)
            git('init','-q','-b','main')
            git('add','src/smoke-select.py','src/smoke-map.json','Plans/subsystems.md','src/agents/rag/agent.json')
            git('-c','user.name=Fixture','-c','user.email=fixture@example.test','commit','-qm','base')
            manifest.write_text('{}\n\n')
            run = subprocess.run(['python3',str(repo/'src/smoke-select.py'),'--mode','normal','--out',str(repo/'plan')],capture_output=True,text=True,timeout=10)
            self.assertEqual(run.returncode,0,run.stdout+run.stderr)
            self.assertIn('L: normal — changed: src/agents/rag/agent.json',run.stdout)
            self.assertIn('check agent_pack: normal',run.stdout)
            self.assertIn('agent pack only',run.stdout)
            self.assertNotIn('check go_unit',run.stdout)

    def test_shared_router_selects_both_platforms(self):
        p = self.plan(['src/agents/shared/router.ts'])
        self.assertEqual(p['checks']['agent_telegram'], 'normal')
        self.assertEqual(p['checks']['agent_slack'], 'normal')

    def test_helper_outside_leaf_prefixes_broadens_the_subsystem(self):
        p = self.plan(['src/agents/standalone/metadata.py'])
        self.assertEqual(p['checks']['agent_telegram'], 'normal')
        self.assertEqual(p['checks']['agent_systemd'], 'normal')

    def test_lower_layer_includes_reverse_dependencies(self):
        p = self.plan(['src/internal/protocol/envelope.go'])
        for letter in 'ABCEFGHIJK': self.assertEqual(p['modes'][letter], 'normal', letter)
        self.assertEqual(p['modes']['L'], 'fast')
        p = self.plan(['src/internal/core/core.go'])
        self.assertEqual(p['modes']['I'], 'normal')
        self.assertEqual(p['checks']['agent_telegram'], 'fast')

    def test_version_and_every_root_changelog_are_always_ignored(self):
        for paths in (['src/internal/version/VERSION'], ['CHANGELOG.0.9.md'], ['CHANGELOG.manual.md'],
                      ['src/agents/VERSION'], ['src/agents/CHANGELOG.md']):
            p = self.plan(paths)
            self.assertEqual(p['fallback'], 'nothing changed: all fast')
            self.assertEqual(set(p['modes'].values()), {'fast'})
            self.assertEqual(set(p['checks'].values()), {'fast','skip'})
        p = self.plan(['CHANGELOG.0.9.md', 'src/agents/telegram/config.ts'])
        self.assertEqual(p['checks']['agent_slack'], 'skip')
        self.assertEqual(p['checks']['agent_telegram'], 'normal')

    def test_container_image_is_its_own_subsystem(self):
        p = self.plan(['src/container/Containerfile'])
        self.assertEqual({c for c, m in p['modes'].items() if m != 'fast'}, {'Q'})
        self.assertEqual({c for c, l in p['checks'].items() if l == 'normal'}, {'container_image'})
        self.assertEqual(self.plan([], 'normal', 'Q')['checks']['container_image'], 'normal')

    def test_distro_install_runs_only_in_the_slow_set_for_its_paths(self):
        for path in ['install.sh', 'src/acceptance/distro/in-container.sh', 'src/cmd/agent-bus-setup/upgrade.go']:
            self.assertEqual(self.plan([path], 'slow')['checks']['distro_install'], 'slow', path)
            self.assertEqual(self.plan([path], 'fast')['checks']['distro_install'], 'skip', path)
        self.assertEqual(self.plan(['src/agents/telegram/receive.ts'], 'slow')['checks']['distro_install'], 'skip')
        self.assertEqual(self.plan([], 'slow', 'all')['checks']['distro_install'], 'slow')

    def test_agent_guide_shard_is_selected_with_a_named_reason_for_each_of_its_paths(self):
        # One shard runs every guide example (src/acceptance/agent-guide); each owner of what it exercises selects it.
        cases = {
            'H': 'src/cmd/agent-bus/run.go', 'F': 'src/cmd/agent-bus-runner/serve.go',
            'K': 'src/clients/examples/agent-guide/packages/time-simple/time-simple.sh',
            'N': 'docs/user/write-agent-cli.md', 'M': 'src/acceptance/agent-guide/guide.sh',
        }
        for letter, path in cases.items():
            with self.subTest(letter=letter, path=path):
                p = self.plan([path])
                self.assertEqual(p['checks']['agent_guide'], 'normal')
                self.assertEqual(p['reasons'][letter], 'changed: ' + path)
                fast = self.plan([path], 'fast')
                self.assertEqual(fast['checks']['agent_guide'], 'fast')
        # Unrelated code of another face leaves it at fast, and fast is only the bash time service.
        p = self.plan(['src/clients/go/bus.go'])
        self.assertEqual(p['checks']['agent_guide'], 'fast')
        import json
        config = json.loads((ROOT/'src/smoke-map.json').read_text())
        self.assertEqual(config['fast']['agent_guide'], [{'argv': ['bash', 'acceptance/agent-guide/guide.sh', 'fast']}])

    def test_shared_or_unknown_fails_closed(self):
        # The subsystem table is the selector's own input, so a change to it can move any selection.
        for path in ['src/smoke.sh', 'src/go.mod', 'src/web/bun.lock', 'new-unknown-file', 'Plans/subsystems.md']:
            p = self.plan([path])
            self.assertEqual(set(p['checks'].values()), {'normal'}, path)

    def test_a_linked_node_modules_is_not_a_change(self):
        # Every worktree links node_modules from the main checkout. A symlink is a file to git, so
        # ignore patterns written for directories ("node_modules/") let it through as untracked, and
        # a docs-only diff then selected the web, MCP, client and every agent check at normal (J25).
        import shutil, subprocess, tempfile
        (ROOT/'tmp').mkdir(exist_ok=True)
        repo = Path(tempfile.mkdtemp(prefix='select-ignore-', dir=ROOT/'tmp'))
        try:
            def git(*args): subprocess.run(['git', '-C', str(repo), *args], check=True, capture_output=True)
            git('init', '-q', '-b', 'main')
            git('config', 'user.email', 't@t'); git('config', 'user.name', 't')
            shutil.copy(ROOT/'.gitignore', repo/'.gitignore')
            (repo/'README.md').write_text('x\n')
            git('add', '.gitignore', 'README.md'); git('commit', '-q', '-m', 'base')
            target = repo/'elsewhere'
            (target/'node_modules').mkdir(parents=True)
            for where in ('src/web', 'src/mcp', 'src/clients/typescript'):
                (repo/where).mkdir(parents=True)
                (repo/where/'node_modules').symlink_to(target/'node_modules')
            (repo/'Plans/R1.0-Release').mkdir(parents=True)
            (repo/'Plans/R1.0-Release/TODO.md').write_text('docs only\n')
            self.assertEqual(selector.changed(repo), ['Plans/R1.0-Release/TODO.md'])
            p = selector.plan(ROOT, 'normal', paths=selector.changed(repo))
            self.assertEqual({c for c, m in p['modes'].items() if m == 'normal'}, {'O'})
            self.assertEqual({c for c, l in p['checks'].items() if l == 'normal'}, set())
        finally:
            shutil.rmtree(repo, ignore_errors=True)

    def test_override_and_release(self):
        p = self.plan([], 'slow', 'L')
        self.assertEqual(p['checks']['agent_slack'], 'slow')
        self.assertEqual(p['checks']['web_ts'], 'fast')
        self.assertEqual(set(self.plan([], 'slow', 'all')['checks'].values()), {'slow'})
        self.assertEqual(self.plan([], 'normal', 'A,F')['modes']['F'], 'normal')
        for target in ['Z', 'L,I,', '--normal', 'L L']:
            with self.assertRaises(ValueError): self.plan([], targets=target)

    def test_docs_and_clean_tree_do_not_get_empty_green(self):
        for paths in [[], ['docs/00-overview.md'], ['Plans/R1.0-Release/TODO.md']]:
            p = self.plan(paths, 'fast')
            self.assertTrue(any(level != 'skip' for level in p['checks'].values()))

    def test_fast_uses_only_explicit_allowlists_and_normal_keeps_regressions(self):
        import json
        config = json.loads((ROOT/'src/smoke-map.json').read_text())
        p = self.plan(['src/smoke.sh'], 'fast')
        self.assertEqual({check for check,level in p['checks'].items() if level != 'skip'},set(config['fast']))
        p = self.plan(['src/smoke.sh'])
        self.assertEqual(set(p['checks'].values()),{'normal'})

    def test_split_go_checks_keep_the_original_selection_for_every_subsystem(self):
        for mode in ('fast', 'normal', 'slow'):
            for target in selector.ALL:
                with self.subTest(mode=mode, target=target):
                    selected = self.plan([], mode, target)['checks']
                    self.assertEqual(selected['go_unit'], selected['go_checks'])
                    self.assertEqual(selected['go_release'], selected['go_checks'])
        for paths in (['src/npm/pack.py'], ['src/internal/core/core.go'], ['src/smoke.sh']):
            selected = self.plan(paths)['checks']
            self.assertEqual([selected[n] for n in ('go_checks', 'go_unit', 'go_release')], ['normal'] * 3)

    def test_daemon_api_doc_selects_the_go_route_contract(self):
        # api/route_contract_test.go reads docs/09-daemon-api.md; an edit there ran no Go test (J111).
        p = self.plan(['docs/09-daemon-api.md'])
        self.assertEqual(self.plan(['docs/09-daemon-api.md'], 'fast')['fallback'], '')
        self.assertEqual(p['reasons']['E'], 'changed: docs/09-daemon-api.md')
        self.assertEqual(p['checks']['go_unit'], 'normal')

    # A test or tool that reads a Markdown file is a check of that file, so a change to the
    # page must run the reader's own subsystem at normal. The docs checks are the exception:
    # #ab-gate runs them for a Markdown-only landing, and go_checks' fast list in every other.
    DOCS_CHECK_READERS = {
        'src/check-links.py', 'src/check-markers.py',
        'src/agents/agent_docs_test.py', 'src/agents/docs_order_test.py',
    }
    # (file, path): a literal that names a Markdown file and never reads it; one reason each.
    NAMES_ONLY = {
        ('src/internal/display/urls.go', 'docs/agents/bus.md'): 'the source URL printed in help',
        ('src/cmd/agent-bus-runner/main.go', 'docs/runner/configuration.md'): 'the source URL printed in help',
        ('src/cmd/agent-bus/usage.go', 'docs/user/cli.md'): 'the source URL printed in help',
        ('src/cmd/agent-bus/usage.go', 'docs/user/dedicated-runner.md'): 'the source URL printed in help',
        ('src/cmd/agent-bus/usage_test.go', 'docs/user/cli.md'): 'asserts the help URL text',
        ('src/cmd/agent-bus/run_stdin_test.go', 'docs/user/dedicated-runner.md'): 'asserts the help URL text',
    }

    def markdown_literals(self):
        """(file, path) for each string literal in src/ code naming a tracked Markdown file under docs/, Plans/ or the root."""
        import os, subprocess
        files = subprocess.check_output(['git', '-C', str(ROOT), 'ls-files'], text=True).split()
        tracked = set(files)
        literal = re.compile(r'''["'`]([A-Za-z0-9_./-]*/[A-Za-z0-9_.-]+\.md)["'`]''')
        found = set()
        for name in files:
            # This file's literals are selection inputs and the lists above, not reads.
            if not name.startswith('src/') or '/node_modules/' in name or name == 'src/smoke_select_test.py':
                continue
            if not name.endswith(('.go', '.py', '.ts', '.js', '.sh', '.rs')):
                continue
            for line in (ROOT/name).read_text(errors='replace').splitlines():
                if line.strip().startswith(('#', '//', '*')):
                    continue
                for m in literal.finditer(line):
                    for path in {os.path.normpath(os.path.join(os.path.dirname(name), m[1])), os.path.normpath(m[1].lstrip('/'))}:
                        if path in tracked and (path.startswith(('docs/', 'Plans/')) or '/' not in path):
                            found.add((name, path))
        return found

    def test_every_markdown_read_by_code_runs_its_reader(self):
        import json
        found = self.markdown_literals()
        self.assertIn(('src/internal/api/route_contract_test.go', 'docs/09-daemon-api.md'), found)
        fast = json.dumps(json.loads((ROOT/'src/smoke-map.json').read_text())['fast']['go_checks'])
        for reader in self.DOCS_CHECK_READERS:
            self.assertIn(Path(reader).name, fast, reader)
        rows = selector.prefixes(ROOT)
        def runs_reader(name, path):
            letter = next((letter for prefix, letter in rows if name.startswith(prefix)), None)
            modes = self.plan([path])['modes']
            return all(m == 'normal' for m in modes.values()) if letter is None else modes[letter] == 'normal'
        self.assertTrue(runs_reader('src/internal/api/route_contract_test.go', 'docs/09-daemon-api.md'))
        self.assertFalse(runs_reader('src/internal/api/route_contract_test.go', 'docs/glossary.md'))
        unowned = []
        for name, path in sorted(found):
            if name in self.DOCS_CHECK_READERS or (name, path) in self.NAMES_ONLY:
                continue
            if not runs_reader(name, path):
                unowned.append(f'{name} reads {path}')
        self.assertEqual(unowned, [], "map the page to the reader's subsystem, or list a name-only literal with a reason")
        stale = sorted(set(self.NAMES_ONLY) - found)
        self.assertEqual(stale, [], 'NAMES_ONLY entries no longer in src/: remove them')
        self.assertTrue(all(reason.strip() for reason in self.NAMES_ONLY.values()))

    def test_map_covers_every_shard_and_core_section(self):
        script = (ROOT/'src/smoke.sh').read_text()
        names = re.search(r'SHARDS=\((.*?)\)', script, re.S)[1].split()
        expected = (set(names)-{'core'}) | {f'core_{n}' for n in range(1,11)}
        actual = set(self.plan([])['checks'])
        self.assertEqual(actual, expected)
        for prefix, letter in selector.prefixes(ROOT):
            self.assertIn(letter, selector.ALL)
        self.assertEqual(self.plan(['src/cmd/agent-bus-runner/serve.go'])['modes']['F'], 'normal')

if __name__ == '__main__': unittest.main()
