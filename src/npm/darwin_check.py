#!/usr/bin/env python3
"""P.1c: macOS platform packages from the npm builder, on local tarballs only.

The macOS client archives are the Linux client's files under darwin names (the
builder never runs a binary), so this runs on any Linux node; the Mac itself runs
the acceptance steps in the plan. Nothing is published.

  NPM_DIST=<dir> python3 src/npm/darwin_check.py     reuses built Linux archives
"""
import contextlib
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import threading
import unittest

from registry import registry

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('npm_pack', Path(__file__).with_name('pack.py'))
pack = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pack)

ARCH = 'aarch64' if os.uname().machine == 'aarch64' else 'x86_64'
MAC = {'arm64': 'aarch64', 'x64': 'x86_64'}  # npm cpu -> the archive name's CPU, as package.sh names it


def darwin_archive(client_archive, version, mac_cpu, out):
    """The Linux client's release directory under a darwin name, archived."""
    work = out / ('src-' + mac_cpu)
    work.mkdir(parents=True)
    with tarfile.open(client_archive) as tar:
        tar.extractall(work, filter='data')
    old = next(work.iterdir())
    name = f'agent-bus-client-{version}-darwin-{MAC[mac_cpu]}'
    old.rename(work / name)
    archive = out / (name + '.tar.gz')
    with tarfile.open(archive, 'w:gz', compresslevel=1) as tar:
        tar.add(work / name, arcname=name)
    return archive


def slim(archive, out, version):
    """The release archive with every large file replaced by a small stand-in, its manifest and checksum kept
    consistent. The packing and wiring under test do not care what the programs are; agent-bus answers --version."""
    work = out / ('slim-' + archive.name.split('.tar')[0])
    work.mkdir(parents=True)
    with tarfile.open(archive) as tar:
        tar.extractall(work, filter='data')
    root = next(work.iterdir())
    lines = []
    for line in (root / 'MANIFEST.sha256').read_text().splitlines():
        name = line.split('  ', 1)[1]
        path = root / name
        if path.stat().st_size > 1 << 20:
            path.write_text(f'#!/bin/sh\necho {version}\n' if name == 'agent-bus' else '#!/bin/sh\nexit 0\n')
            line = f'{pack.digest(path)}  {name}'
        lines.append(line + '\n')
    (root / 'MANIFEST.sha256').write_text(''.join(lines))
    slimmed = out / archive.name
    with tarfile.open(slimmed, 'w:gz', compresslevel=1) as tar:
        tar.add(root, arcname=root.name)
    shutil.rmtree(work)
    return slimmed


class DarwinPackages(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        (ROOT / 'tmp').mkdir(exist_ok=True)
        scratch = tempfile.TemporaryDirectory(prefix='npm-darwin-', dir=ROOT / 'tmp')
        cls.addClassCleanup(scratch.cleanup)
        cls.work = Path(scratch.name)
        if 'NPM_DIST' in os.environ:
            dist = Path(os.environ['NPM_DIST'])
        else:
            dist = cls.work / 'dist'
            built = subprocess.run([str(ROOT / 'src/package.sh'), str(dist)], capture_output=True, text=True)
            if built.returncode:
                raise AssertionError('release build failed: ' + built.stderr[-1500:])
        full = next(p for p in dist.glob(f'agent-bus-*-linux-{ARCH}.tar.gz') if '-client-' not in p.name)
        cls.version = full.name.removeprefix('agent-bus-').split('-linux-')[0]
        (cls.work / 'slim').mkdir()
        cls.full = slim(full, cls.work / 'slim', cls.version)
        cls.client = slim(dist / full.name.replace('agent-bus-', 'agent-bus-client-', 1), cls.work / 'slim', cls.version)
        cls.names = pack.names()
        (cls.work / 'mac').mkdir()
        cls.mac = {cpu: darwin_archive(cls.client, cls.version, cpu, cls.work / 'mac') for cpu in MAC}
        # The release packed once with both macOS archives; the checks that read the result share it.
        cls.packed = cls.work / 'packed'
        cls.packed.mkdir()
        cls.sums_of([cls.full, cls.client, *cls.mac.values()], cls.packed)
        with contextlib.redirect_stdout(None):
            pack.build(cls.full, cls.client, cls.packed / 'SHA256SUMS', cls.packed / 'tarballs', darwin=list(cls.mac.values()))
        cls.tarballs = cls.packed / 'tarballs'

    @staticmethod
    def sums_of(archives, directory):
        path = directory / 'SHA256SUMS'
        path.write_text(''.join(f'{pack.digest(a)}  {a.name}\n' for a in archives))
        return path

    def sums(self, archives, directory):
        return self.sums_of(archives, directory)

    def build(self, name, *, darwin=None, full=None):
        directory = self.work / name
        directory.mkdir()
        darwin = list(self.mac.values()) if darwin is None else darwin
        full = full or self.full
        sums = self.sums([full, self.client, *darwin], directory)
        with contextlib.redirect_stdout(None):
            pack.build(full, self.client, sums, directory / 'tarballs', darwin=darwin)
        return directory / 'tarballs'

    @staticmethod
    def metadata(tarball):
        with tarfile.open(tarball) as tar:
            return json.load(tar.extractfile('package/package.json')), tar.getnames()

    @classmethod
    def package(cls, tarballs, name):
        found = [p for p in tarballs.glob('*.tgz') if cls.metadata(p)[0]['name'] == name]
        return cls.metadata(found[0]) if found else (None, [])

    def test_darwin_platform_packages_exist_with_names_os_cpu_and_files(self):
        tarballs = self.tarballs
        for cpu in ('arm64', 'x64'):
            meta, files = self.package(tarballs, f"{self.names['client']}-darwin-{cpu}")
            self.assertIsNotNone(meta, f'no macOS {cpu} platform package')
            self.assertEqual((meta['os'], meta['cpu'], meta['version']), (['darwin'], [cpu], self.version))
            for program in ('agent-bus', 'agent-bus-runner', 'agent-bus-ts', 'launchers/ab-claude', 'LICENSE.md'):
                self.assertTrue(any(f.endswith('/' + program) for f in files), f'{cpu} package lacks {program}')
            self.assertNotIn('package/bundle/agent-busd', files, 'the daemon is not in a macOS client package')
        self.assertIsNone(self.package(tarballs, f"{self.names['bus']}-darwin-arm64")[0], 'no macOS bus platform package exists')

    def test_both_command_packages_wire_the_macos_platforms(self):
        tarballs = self.tarballs
        for kind in ('client', 'bus'):
            meta, _ = self.package(tarballs, self.names[kind])
            self.assertIn('darwin', meta['os'])
            self.assertIn('linux', meta['os'])
            for cpu in ('arm64', 'x64'):
                self.assertEqual(meta['optionalDependencies'].get(f"{self.names['client']}-darwin-{cpu}"), self.version,
                                 f'{kind} does not pin the macOS {cpu} client package')
                self.assertEqual(meta['agentBus']['clientPlatforms'].get('darwin-' + cpu), f"{self.names['client']}-darwin-{cpu}")
            self.assertIn('agent-bus', meta['bin'])
        bus, _ = self.package(tarballs, self.names['bus'])
        self.assertFalse([k for k in bus['agentBus']['platforms'] if k.startswith('darwin')], 'the bus has no macOS platform')

    def install(self, tarballs, package, prefix, os_name, cpu):
        server = registry(tarballs)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        self.addCleanup(lambda: (server.shutdown(), server.server_close()))
        env = {'PATH': os.environ['PATH'], 'HOME': str(prefix.parent), 'npm_config_cache': str(prefix.parent / 'cache'), 'npm_config_update_notifier': 'false'}
        done = subprocess.run(['npm', 'install', '-g', '--prefix', str(prefix), '--no-audit', '--no-fund', '--ignore-scripts', f'--os={os_name}', f'--cpu={cpu}',
                               '--registry', f'http://127.0.0.1:{server.server_port}', package + '@' + self.version], env=env, capture_output=True, text=True)
        self.assertEqual(done.returncode, 0, done.stderr[-1500:])
        return env

    def as_mac(self, prefix, env, package, program, *args):
        """The command through the installed launcher, as macOS arm64 reports itself."""
        launcher = prefix / 'lib/node_modules' / package / 'launch.cjs'
        script = ('Object.defineProperty(process,"platform",{value:"darwin"});Object.defineProperty(process,"arch",{value:"arm64"});'
                  f'require({json.dumps(str(launcher))})({json.dumps(program)},{json.dumps(list(args))});')
        return subprocess.run(['node', '-e', script], env=env, capture_output=True, text=True, timeout=30)

    def test_the_agent_bus_command_resolves_on_macos(self):
        # One install serves both: the bus package pulls the macOS client package in for its agent-bus command.
        prefix = self.work / 'prefix' / 'p'
        prefix.parent.mkdir()
        env = self.install(self.tarballs, self.names['bus'], prefix, 'darwin', 'arm64')
        self.assertTrue(any(self.names['client'] + '-darwin-arm64' in str(p) for p in (prefix / 'lib/node_modules').rglob('package.json')),
                        'the macOS client package was not installed')
        result = self.as_mac(prefix, env, self.names['bus'], 'agent-bus', '--version')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.splitlines()[0], self.version)
        # The daemon is Linux's: on macOS its command says so instead of running anything.
        refused = self.as_mac(prefix, env, self.names['bus'], 'agent-busd', '--version')
        self.assertNotEqual(refused.returncode, 0)
        self.assertIn('unsupported platform darwin-arm64', refused.stderr)

    def test_a_macos_archive_must_match_the_release(self):
        # a version that differs from the Linux set
        (self.work / 'badversion').mkdir()
        bad = darwin_archive(self.client, self.version, 'arm64', self.work / 'badversion')
        renamed = bad.with_name(bad.name.replace(self.version, '9.9.9'))
        bad.rename(renamed)
        directory = self.work / 'badver-out'
        directory.mkdir()
        sums = self.sums([self.full, self.client, renamed], directory)
        with self.assertRaises(ValueError) as caught:
            pack.build(self.full, self.client, sums, directory / 'tarballs', darwin=[renamed])
        self.assertIn('same version', str(caught.exception))
        # a macOS client archive that lacks a file the Linux client has, its manifest kept consistent
        (self.work / 'lacking').mkdir()
        short = darwin_archive(self.client, self.version, 'arm64', self.work / 'lacking')
        unpacked = self.work / 'lacking' / 'x'
        unpacked.mkdir()
        with tarfile.open(short) as tar:
            tar.extractall(unpacked, filter='data')
        release = next(unpacked.iterdir())
        (release / 'launchers/ab-claude').unlink()
        lines = (release / 'MANIFEST.sha256').read_text().splitlines()
        (release / 'MANIFEST.sha256').write_text(''.join(line + '\n' for line in lines if not line.endswith('  launchers/ab-claude')))
        with tarfile.open(short, 'w:gz', compresslevel=1) as tar:
            tar.add(release, arcname=release.name)
        directory = self.work / 'lacking-out'
        directory.mkdir()
        sums = self.sums([self.full, self.client, short], directory)
        with self.assertRaises(ValueError) as caught:
            pack.build(self.full, self.client, sums, directory / 'tarballs', darwin=[short])
        self.assertIn('differs from the Linux client in files: launchers/ab-claude', str(caught.exception))
        # an archive SHA256SUMS does not list
        directory = self.work / 'nosums-out'
        directory.mkdir()
        sums = self.sums([self.full, self.client], directory)
        with self.assertRaises(ValueError) as caught:
            pack.build(self.full, self.client, sums, directory / 'tarballs', darwin=[self.mac['arm64']])
        self.assertIn('does not match SHA256SUMS', str(caught.exception))
        # two archives for one CPU
        twin = self.work / 'twin'
        twin.mkdir()
        duplicate = shutil.copy2(self.mac['arm64'], twin / self.mac['arm64'].name)
        sums = self.sums([self.full, self.client, self.mac['arm64']], twin)
        with self.assertRaises(ValueError):
            pack.build(self.full, self.client, sums, twin / 'tarballs', darwin=[self.mac['arm64'], Path(duplicate)])


if __name__ == '__main__':
    unittest.main(verbosity=2)
