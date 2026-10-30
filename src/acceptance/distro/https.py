"""Serve a release tree over HTTPS as releases.test, for install.sh's curl.

Usage: python3 https.py <dir> <cert> <key>
"""
import functools
import http.server
import ssl
import sys

root, cert, key = sys.argv[1:4]
handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=root)
server = http.server.ThreadingHTTPServer(('127.0.0.1', 443), handler)
context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
context.load_cert_chain(cert, key)
server.socket = context.wrap_socket(server.socket, server_side=True)
server.serve_forever()
