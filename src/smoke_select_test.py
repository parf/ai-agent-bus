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

    def test_telegram_leaf_keeps_other_faces_fast(self):
        p = self.plan(['src/agents/telegram/receive.ts'])
        self.assertEqual(p['checks']['agent_telegram'], 'normal')
        self.assertEqual(p['checks']['agent_slack'], 'fast')
        self.assertEqual(p['modes']['A'], 'fast')
        self.assertEqual(self.plan(['src/agents/telegram/receive.ts'], 'fast')['checks']['agent_slack'], 'skip')

    def test_imap_changes_include_bundle_and_runtime_contract_checks(self):
        for path in ('imap.py', 'agent.json', 'folders.py', 'package.sh'):
            with self.subTest(path=path):
                plan = self.plan(['src/agents/imap/' + path])
                for shard in ('go_checks', 'go_unit', 'go_release'):
                    self.assertEqual(plan['checks'][shard], 'normal', shard)
                self.assertEqual(plan['checks']['agent_slack'], 'fast')

    def test_every_bundled_manifest_selects_the_runner_reload_gate(self):
        import json
        manifests = list((ROOT / 'src/agents').rglob('agent.json'))
        manifests = [path for path in manifests if 'node_modules' not in path.parts]
        self.assertTrue(manifests, 'no bundled manifest was checked')
        for path in manifests:
            rel = path.relative_to(ROOT).as_posix()
            with self.subTest(path=rel):
                self.assertEqual(self.plan([rel], 'fast')['checks']['go_unit'], 'fast')
                self.assertEqual(self.plan([rel], 'normal')['checks']['go_unit'], 'normal')
        commands = json.loads((ROOT / 'src/smoke-map.json').read_text())['fast']['go_unit']
        self.assertIn({'argv': ['go', 'test', '-count=1', './cmd/agent-bus-runner',
                               '-run', '^TestEveryBundledManifestPassesRunnerReload$']}, commands)

    def test_every_agent_manifest_selects_setup_runtime_checks_with_its_reason(self):
        manifests = sorted((ROOT / 'src/agents').rglob('agent.json'))
        manifests = [p.relative_to(ROOT).as_posix() for p in manifests if 'node_modules' not in p.parts]
        self.assertIn('src/agents/rag/agent.json', manifests)
        for path in manifests:
            with self.subTest(path=path):
                plan = self.plan([path], 'normal')
                self.assertEqual(plan['checks']['go_unit'], 'normal', path + ': setup runtime tests omitted')
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
            self.assertIn('check go_unit: normal',run.stdout)

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
        for letter in 'ABCEFGHIJKL': self.assertEqual(p['modes'][letter], 'normal', letter)
        p = self.plan(['src/internal/core/core.go'])
        self.assertEqual(p['modes']['I'], 'normal')
        self.assertEqual(p['checks']['agent_telegram'], 'normal')

    def test_version_and_every_root_changelog_are_always_ignored(self):
        for paths in (['src/internal/version/VERSION'], ['CHANGELOG.0.9.md'], ['CHANGELOG.manual.md']):
            p = self.plan(paths)
            self.assertEqual(p['fallback'], 'nothing changed: all fast')
            self.assertEqual(set(p['modes'].values()), {'fast'})
            self.assertEqual(set(p['checks'].values()), {'fast','skip'})
        p = self.plan(['CHANGELOG.0.9.md', 'src/agents/telegram/config.ts'])
        self.assertEqual(p['checks']['agent_slack'], 'fast')

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
        for path in ['src/smoke.sh', 'src/go.mod', 'src/web/bun.lock', 'new-unknown-file']:
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
