"""A bounded HTTP listener; one raw UTF-8 body becomes one configured bus send."""
from concurrent.futures import ThreadPoolExecutor, wait
import http.client
from http.server import BaseHTTPRequestHandler, HTTPServer
import socket
import threading
import time
from agent_bus.errors import BusError, Refusal, UncertainOutcome
from rules import HEADER_MAX, Invalid, basic_ok, path, signature_ok

GENERIC = b'Not found\n'


class HeaderReader:
    def __init__(self, raw, connection):
        self.raw = raw; self.connection = connection
        self.left = HEADER_MAX; self.end = time.monotonic() + 2; self.done = False; self.first = True
    def readline(self, size=-1):
        if self.done:
            return self.raw.readline(size)
        remaining = self.end - time.monotonic()
        if remaining <= 0:
            raise TimeoutError()
        self.connection.settimeout(remaining)
        line = self.raw.readline(min(self.left + 1, size if size >= 0 else self.left + 1))
        self.left -= len(line)
        if self.left < 0:
            raise http.client.LineTooLong('headers')
        if not self.first and line.startswith((b' ', b'\t')):
            raise http.client.HTTPException('folded header')
        if not self.first and line in (b'\r\n', b'\n'):
            self.done = True
        self.first = False
        return line
    def __getattr__(self, name):
        return getattr(self.raw, name)


class Listener(HTTPServer):
    allow_reuse_address = True
    allow_reuse_port = False
    def __init__(self, address, routes, secrets, bus, state):
        self.routes = {r['path']: r for r in routes if 'path' in r}
        self.secrets = secrets; self.bus = bus; self.state = state
        self.slots = threading.BoundedSemaphore(8); self.preauth = threading.BoundedSemaphore(2)
        self.pool = ThreadPoolExecutor(max_workers=8, thread_name_prefix='webhook-http')
        self.guard = threading.Lock(); self.futures = set(); self.connections = set(); self.stopping = False
        try:
            super().__init__(address, Handler)
        except BaseException:
            self.pool.shutdown(wait=False, cancel_futures=True)
            raise
    def process_request(self, connection, address):
        slot = self.slots.acquire(blocking=False)
        auth = self.preauth.acquire(blocking=False) if slot else False
        if self.stopping or not slot or not auth:
            if slot: self.slots.release()
            if auth: self.preauth.release()
            try:
                connection.settimeout(.2)
                connection.sendall(b'HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\nContent-Length: 5\r\n\r\nBusy\n')
            except OSError: pass
            self.shutdown_request(connection); self.state.count('busy'); return
        with self.guard:
            self.connections.add(connection)
        def run():
            # A handler releases this early only after actual authentication.
            lease = [True]
            self._lease.connection = connection; self._lease.value = lease
            try:
                self.finish_request(connection, address)
            except (OSError, ValueError, http.client.HTTPException):
                self.state.count('framing')
            finally:
                if lease[0]: self.preauth.release()
                self.shutdown_request(connection)
                with self.guard: self.connections.discard(connection)
                self.slots.release()
        future = self.pool.submit(run)
        with self.guard: self.futures.add(future)
        def complete(f):
            with self.guard: self.futures.discard(f)
        future.add_done_callback(complete)
    _lease = threading.local()
    def authenticated(self):
        lease = self._lease.value
        if lease[0]:
            lease[0] = False; self.preauth.release()
    def start(self):
        self.thread = threading.Thread(target=self.serve_forever, kwargs={'poll_interval': .05}, daemon=True)
        self.thread.start()
    def stop(self, grace=15):
        self.stopping = True; self.shutdown(); self.server_close(); self.thread.join(timeout=1)
        with self.guard: pending = list(self.futures)
        _, pending = wait(pending, timeout=grace)
        if pending:
            with self.guard: connections = list(self.connections)
            for connection in connections:
                try: connection.shutdown(socket.SHUT_RDWR)
                except OSError: pass
            wait(pending, timeout=4)
        self.pool.shutdown(wait=False, cancel_futures=True)


class Handler(BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'
    server_version = 'webhook'
    sys_version = ''
    def setup(self):
        super().setup()
        self.rfile = HeaderReader(self.rfile, self.connection)
    def log_message(self, *_): pass
    def handle_expect_100(self):
        self.fail(417, 'framing'); return False
    def send_error(self, code, message=None, explain=None):
        self.fail(code, 'framing')
    def respond(self, code, body=GENERIC):
        self.close_connection = True
        try:
            self.connection.settimeout(1)
            self.send_response_only(code)
            self.send_header('Connection', 'close')
            self.send_header('Content-Type', 'text/plain; charset=utf-8')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers(); self.wfile.write(body)
        except OSError: pass
    def fail(self, code, reason, route='-'):
        self.server.state.count(reason); self.server.state.log(route, 0, reason)
        self.respond(code)
    def do_GET(self): self.fail(404, 'method')
    do_HEAD = do_GET
    do_PUT = do_GET
    do_PATCH = do_GET
    do_DELETE = do_GET
    do_OPTIONS = do_GET
    def do_POST(self):
        headers = self.headers
        if self.request_version not in ('HTTP/1.0', 'HTTP/1.1') or len(headers) > 32 or len(headers.get_all('Host', [])) != 1 or any(len(headers.get_all(k, [])) > 1 for k in ('Content-Length', 'Authorization', 'X-Hub-Signature-256', 'Stripe-Signature')) or 'Transfer-Encoding' in headers or 'Expect' in headers:
            self.fail(400, 'framing'); return
        try: path(self.path)
        except Invalid:
            self.fail(404, 'authentication'); return
        route = self.server.routes.get(self.path)
        # Basic is deliberately checked before any body is read. Unknown paths
        # and bad credentials have identical unauthenticated response bytes.
        if route is None or not basic_ok(route, headers, self.server.secrets):
            self.fail(404, 'authentication'); return
        length = headers.get('Content-Length')
        if length is None or not length.isascii() or not length.isdigit() or len(length) > 9:
            self.fail(400, 'framing'); return
        size = int(length)
        if size > route['max_bytes']:
            self.fail(413, 'size', route['name']); return
        if 'basic' in route:
            self.server.authenticated()
        try:
            chunks, left = [], size
            end = time.monotonic() + 5
            while left:
                remaining = end - time.monotonic()
                if remaining <= 0: raise TimeoutError()
                self.connection.settimeout(remaining)
                chunk = self.rfile.read1(min(left, 65536))
                if not chunk: raise ValueError()
                chunks.append(chunk); left -= len(chunk)
            body = b''.join(chunks)
            # Refuse already pipelined bytes, and never process a second request.
            self.connection.setblocking(False)
            try: extra = self.rfile.peek(1)
            except (BlockingIOError, OSError): extra = b''
            finally: self.connection.settimeout(1)
            if extra:
                self.fail(400, 'framing'); return
            decoded = body.decode('utf-8')
        except (OSError, ValueError, UnicodeError):
            self.fail(400, 'body', route['name']); return
        if not signature_ok(route, headers, body, self.server.secrets):
            self.fail(404, 'authentication'); return
        self.server.authenticated()
        if not self.server.state.take(route):
            self.fail(429, 'rate', route['name']); return
        try:
            record = self.server.bus.record(route['target']).lookup()
            if record.get('kind') not in ('queue', 'agent'):
                self.fail(503, 'target', route['name']); return
            self.server.bus.send(route['target'], decoded)
        except UncertainOutcome:
            self.server.state.count('unknown'); self.server.state.log(route['name'], size, 'unknown')
            self.respond(503, b'Delivery outcome unknown; not retried\n'); return
        except BusError:
            self.fail(503, 'delivery', route['name']); return
        self.server.state.count('accepted'); self.server.state.log(route['name'], size, 'accepted')
        self.respond(202, b'Accepted\n')
