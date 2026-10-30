#!/usr/bin/env python3
"""Loopback-only npm fixture registry; serves local tarballs, never publishes."""
import argparse
import base64
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import shutil
import tarfile
from urllib.parse import unquote, quote, urlsplit


def registry(directory):
    packages = {}
    archives = {}
    for path in directory.glob('*.tgz'):
        with tarfile.open(path) as tar:
            value = json.load(tar.extractfile('package/package.json'))
        archives[path.name] = path
        packages.setdefault(value['name'], {})[value['version']] = (value, path)

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_GET(self):
            name = unquote(urlsplit(self.path).path.lstrip('/'))
            if name.startswith('tarballs/') and name[9:] in archives:
                path = archives[name[9:]]
                self.send_response(200)
                self.send_header('Content-Type', 'application/octet-stream')
                self.send_header('Content-Length', str(path.stat().st_size))
                self.end_headers()
                with path.open('rb') as source:
                    shutil.copyfileobj(source, self.wfile)
                return
            versions = packages.get(name)
            if versions is None:
                self.send_error(404, 'no fixture package')
                return
            result = {}
            for version, (value, path) in versions.items():
                with path.open('rb') as source:
                    checksum = base64.b64encode(hashlib.file_digest(source, 'sha512').digest()).decode()
                result[version] = {**value, 'dist': {
                    'tarball': f'http://127.0.0.1:{self.server.server_port}/tarballs/{quote(path.name)}',
                    'integrity': 'sha512-' + checksum}}
            body = json.dumps({'name': name, 'versions': result,
                               'dist-tags': {'latest': sorted(result)[-1]}}).encode()
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    return ThreadingHTTPServer(('127.0.0.1', 0), Handler)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path)
    parser.add_argument('--port-file', type=Path, required=True)
    args = parser.parse_args()
    server = registry(args.directory)
    args.port_file.write_text(str(server.server_port))
    server.serve_forever()
