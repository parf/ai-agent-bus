#!/usr/bin/env python3
"""Wrap the release's full/client archives as split npm packages; never publish."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile

# The sole package-name defaults. Owner may revise these; commands do not change.
NAMES = {'bus': 'ai-agent-bus', 'client': 'ai-agent-bus-client'}


def names():
    result = {key: os.environ.get('NPM_' + key.upper() + '_NAME', value)
              for key, value in NAMES.items()}
    for name in result.values():
        if not re.fullmatch(r'(?:@[a-z0-9._-]+/)?[a-z0-9][a-z0-9._-]*', name):
            raise ValueError('invalid npm package name')
    if result['bus'] == result['client']:
        raise ValueError('bus and client package names must differ')
    return result


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def manifest(path):
    result = {}
    for line in path.read_text().splitlines():
        checksum, name = line.split(maxsplit=1)
        name = name.removeprefix('*')
        if (not re.fullmatch('[0-9a-f]{64}', checksum) or name in result
                or not name or Path(name).is_absolute() or '..' in Path(name).parts):
            raise ValueError('invalid or repeated checksum path')
        result[name] = checksum
    return result


# Shipped in the full archive for the guides to point at, never installed or run:
# the one file the release manifest is allowed not to list.
UNCHECKED = {'launchers/review-settings.json.dist'}


def verify(root):
    entries = manifest(root / 'MANIFEST.sha256')
    for name, checksum in entries.items():
        path = root / name
        if path.is_symlink() or not path.is_file() or digest(path) != checksum:
            raise ValueError('release manifest mismatch: ' + name)
    # A regular file the manifest does not list was never checked, so nothing is
    # built from a release that has one (links are not copied into a package).
    unlisted = sorted(rel for rel in (str(Path(directory, name).relative_to(root)) for directory, _, files in os.walk(root) for name in files
                                      if not Path(directory, name).is_symlink())
                      if rel not in entries and rel not in UNCHECKED | {'MANIFEST.sha256'})
    if unlisted:
        raise ValueError('release has files its manifest does not check: ' + ', '.join(unlisted[:5]))
    return entries


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')


# Every package carries the license and the notices its dependencies require,
# at the paths a release archive has them (licenses/NOTICE.md).
LEGAL = ('LICENSE.md', 'licenses/NOTICE.md')


def ship_legal(release, root):
    for name in LEGAL:
        (root / name).parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(release / name, root / name)


DARWIN_CPU = {'x86_64': 'x64', 'aarch64': 'arm64'}  # package.sh's names for the archives, npm's for the packages


def build(full, client, checksums, output, darwin=()):
    """Wrap the Linux full and client archives, and any macOS client archives, as npm packages.

    macOS has the client edition only (MAC.1; the daemon's archive is not built for it), so a
    darwin archive becomes a client platform package, listed beside the Linux ones in both command
    packages; the bus platform packages stay Linux, and a daemon command on macOS refuses by name.
    """
    package_names = names()
    matched = re.fullmatch(r'agent-bus-(\d+\.\d+\.\d+)-linux-(x86_64|aarch64)\.tar\.gz', full.name)
    if not matched:
        raise ValueError('expected a Linux full release archive')
    version, release_arch = matched.groups()
    cpu = {'x86_64': 'x64', 'aarch64': 'arm64'}[release_arch]
    if client.name != f'agent-bus-client-{version}-linux-{release_arch}.tar.gz':
        raise ValueError('full and client archives must have exactly the same version/platform')
    sums = manifest(checksums)
    darwin_cpus = {}
    for archive in darwin:
        found = re.fullmatch(rf'agent-bus-client-{re.escape(version)}-darwin-(x86_64|aarch64)\.tar\.gz', archive.name)
        if not found or DARWIN_CPU[found.group(1)] in darwin_cpus:
            raise ValueError('expected one macOS client archive per CPU, of the same version: ' + archive.name)
        darwin_cpus[DARWIN_CPU[found.group(1)]] = archive
    for archive in (full, client, *darwin_cpus.values()):
        if sums.get(archive.name) != digest(archive):
            raise ValueError('archive does not match SHA256SUMS: ' + archive.name)
    output.mkdir(parents=True, exist_ok=True)
    scratch = Path(__file__).resolve().parents[2] / 'tmp'
    scratch.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='npm-pack-', dir=scratch) as temporary:
        work = Path(temporary)
        roots = []
        for index, archive in enumerate((full, client)):
            target = work / str(index)
            target.mkdir()
            with tarfile.open(archive) as tar:
                if any(not (entry.isfile() or entry.isdir() or entry.issym()) for entry in tar.getmembers()):
                    raise ValueError('release archive contains unsupported special files')
                tar.extractall(target, filter='data')
            children = list(target.iterdir())
            if len(children) != 1 or not children[0].is_dir():
                raise ValueError('archive must contain one release directory')
            verify(children[0])
            if (children[0] / 'internal/version/VERSION').read_text().strip() != version:
                raise ValueError('archive contents have a different release version')
            roots.append(children[0])
        full_root, client_root = roots
        full_files, client_files = verify(full_root), verify(client_root)
        for name, checksum in client_files.items():
            if not (full_root / name).is_file() or digest(full_root / name) != checksum:
                raise ValueError('client file differs from full release: ' + name)
        darwin_roots = {}
        for number, (mac_cpu, archive) in enumerate(sorted(i for i in darwin_cpus.items() if i[0] != 'x64')):
            target = work / f'darwin-{number}'
            target.mkdir()
            with tarfile.open(archive) as tar:
                if any(not (entry.isfile() or entry.isdir() or entry.issym()) for entry in tar.getmembers()):
                    raise ValueError('release archive contains unsupported special files')
                tar.extractall(target, filter='data')
            children = list(target.iterdir())
            if len(children) != 1 or not children[0].is_dir():
                raise ValueError('archive must contain one release directory')
            mac_files = verify(children[0])
            if (children[0] / 'internal/version/VERSION').read_text().strip() != version:
                raise ValueError('archive contents have a different release version')
            if set(mac_files) != set(client_files):
                raise ValueError('the macOS client archive differs from the Linux client in files: ' + ', '.join(sorted(set(mac_files) ^ set(client_files))[:5]))
            darwin_roots[mac_cpu] = (children[0], mac_files)
        platform_sets = {key: {arch: name + '-' + arch for arch in ('linux-x64', 'linux-arm64')}
                         for key, name in package_names.items()}
        for mac_cpu in darwin_roots:
            platform_sets['client']['darwin-' + mac_cpu] = f"{package_names['client']}-darwin-{mac_cpu}"
        platforms = {key: values['linux-' + cpu] for key, values in platform_sets.items()}
        provenance = {'sha256sums': digest(checksums), 'fullArchive': digest(full), 'clientArchive': digest(client)}
        if darwin_cpus:
            provenance['darwinClientArchives'] = {mac_cpu: digest(path) for mac_cpu, path in sorted(darwin_cpus.items())}
        packages = []
        for kind in ('client', 'bus'):
            root = work / (kind + '-platform')
            bundle = root / 'bundle'
            bundle.mkdir(parents=True)
            selected = client_files if kind == 'client' else {name: checksum for name, checksum in full_files.items() if name not in client_files}
            source = client_root if kind == 'client' else full_root
            for name in selected:
                destination = bundle / name
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source / name, destination)
            # The full manifest reconstructs the exact checked release at setup.
            if kind == 'bus':
                shutil.copy2(full_root / 'MANIFEST.sha256', bundle / 'MANIFEST.sha256')
            ship_legal(full_root, root)
            write_json(root / 'package.json', {'name': platforms[kind], 'version': version,
                'description': f'agent-bus {kind} files for Linux {cpu}',
                'license': 'SEE LICENSE IN LICENSE.md', 'os': ['linux'], 'cpu': [cpu],
                'files': ['bundle', *LEGAL], 'agentBusRelease': provenance})
            packages.append(root)
        for mac_cpu, (mac_root, mac_files) in sorted(darwin_roots.items()):
            root = work / f'client-platform-darwin-{mac_cpu}'
            bundle = root / 'bundle'
            bundle.mkdir(parents=True)
            for name in mac_files:
                destination = bundle / name
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(mac_root / name, destination)
            ship_legal(mac_root, root)
            write_json(root / 'package.json', {'name': platform_sets['client']['darwin-' + mac_cpu], 'version': version,
                'description': f'agent-bus client files for macOS {mac_cpu}',
                'license': 'SEE LICENSE IN LICENSE.md', 'os': ['darwin'], 'cpu': [mac_cpu],
                'files': ['bundle', *LEGAL], 'agentBusRelease': provenance})
            packages.append(root)
        for kind in ('client', 'bus'):
            root = work / kind
            (root / 'bin').mkdir(parents=True)
            ship_legal(full_root, root)
            shutil.copy2(Path(__file__).with_name('launch.cjs'), root / 'launch.cjs')
            bins = ['agent-bus', 'agent-bus-runner', 'agent-bus-mcp', 'ab-claude', 'ab-codex', 'ab-opencode', 'ab-opencode1']
            if kind == 'bus':
                # npm does not put transitive dependencies' commands in the
                # global prefix. Bus shims delegate to the pinned client bytes.
                bins += ['agent-busd', 'agent-bus-admin', 'agent-bus-setup']
            for program in bins:
                script = root / 'bin' / program
                script.write_text('#!/usr/bin/env node\nrequire("../launch.cjs")(' + json.dumps(program) + ', process.argv.slice(2));\n')
                script.chmod(0o755)
            value = {'name': package_names[kind], 'version': version,
                     'license': 'SEE LICENSE IN LICENSE.md', 'description': f'agent-bus {kind} commands',
                     'bin': {program: 'bin/' + program for program in bins},
                     'files': ['bin', 'launch.cjs', *LEGAL], 'engines': {'node': '>=18'},
                     'optionalDependencies': {name: version for name in {**platform_sets[kind], **{k: v for k, v in platform_sets['client'].items() if k.startswith('darwin-')}}.values()},
                     'os': ['linux', *(['darwin'] if darwin_roots else [])], 'cpu': ['x64', 'arm64'],
                     'agentBus': {'kind': kind, 'platforms': platform_sets[kind],
                                  'client': package_names['client'], 'clientPlatforms': platform_sets['client']},
                     'agentBusRelease': {'sha256sums': digest(checksums)}}
            if kind == 'bus':
                value['dependencies'] = {package_names['client']: version}
                shutil.copy2(Path(__file__).with_name('setup.cjs'), root / 'setup.cjs')
                shutil.copy2(Path(__file__).with_name('root-trust.cjs'), root / 'root-trust.cjs')
                value['files'] += ['setup.cjs', 'root-trust.cjs']
            write_json(root / 'package.json', value)
            packages.append(root)
        tarballs = []
        for root in packages:
            result = subprocess.run(['npm', 'pack', '--json', '--pack-destination', str(output)],
                                    cwd=root, text=True, capture_output=True, check=True)
            tarballs.append(str(output / json.loads(result.stdout)[0]['filename']))
        print('\n'.join(tarballs))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('full', type=Path)
    parser.add_argument('client', type=Path)
    parser.add_argument('checksums', type=Path, help='SHA256SUMS from the verified release')
    parser.add_argument('output', type=Path)
    parser.add_argument('--darwin-client', type=Path, action='append', default=[], metavar='ARCHIVE',
                        help='a macOS client archive (agent-bus-client-<version>-darwin-x86_64|aarch64.tar.gz); one per CPU')
    args = parser.parse_args()
    build(*(path.resolve() for path in (args.full, args.client, args.checksums, args.output)),
          darwin=[path.resolve() for path in args.darwin_client])


if __name__ == '__main__':
    main()
