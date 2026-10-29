"""A bounded HTTP listener; one raw UTF-8 body becomes one configured bus send."""
from concurrent.futures import ThreadPoolExecutor, wait
from collections import OrderedDict
from socketserver import TCPServer
import http.client
from http.server import BaseHTTPRequestHandler, HTTPServer
import socket
import threading
import time
from agent_bus.errors import BusError, Refusal, UncertainOutcome
from rules import HEADER_MAX, Invalid, basic_ok, path, signature_ok

GENERIC = b'Not found\n'


class HeaderReader:
    def __init__(self, raw, connection, started):
        self.raw = raw; self.connection = connection
        self.left = HEADER_MAX; self.end = started + 2; self.first_byte = started + 1
        self.done = False; self.first = True; self.buffer = b''; self.seen = False
    def readline(self, size=-1):
        if self.done: return self.raw.readline(size)
        while b'\n' not in self.buffer:
            remaining = min(self.end, self.first_byte if not self.seen else self.end) - time.monotonic()
            if remaining <= 0: raise TimeoutError()
            self.connection.settimeout(remaining)
            chunk = self.raw.read1(max(1, self.left + 1 - len(self.buffer)))
            if not chunk: break
            self.seen = True; self.buffer += chunk
            if len(self.buffer) > self.left and b'\n' not in self.buffer:
                raise http.client.LineTooLong('headers')
        at = self.buffer.find(b'\n')
        end = at + 1 if at >= 0 else len(self.buffer)
        line, self.buffer = self.buffer[:end], self.buffer[end:]
        self.left -= len(line)
        if self.left < 0: raise http.client.LineTooLong('headers')
        if line.endswith(b'\n') and not line.endswith(b'\r\n'):
            raise http.client.HTTPException('CRLF required')
        if not self.first and line.startswith((b' ', b'\t')):
            raise http.client.HTTPException('folded header')
        if not self.first and line == b'\r\n': self.done = True
        self.first = False
        return line
    def read1(self, size):
        if self.buffer:
            result, self.buffer = self.buffer[:size], self.buffer[size:]
            return result
        return self.raw.read1(size)
    def peek(self, size):
        return self.buffer or self.raw.peek(size)
    def __getattr__(self, name): return getattr(self.raw, name)


class Listener(HTTPServer):
    allow_reuse_address = True
    allow_reuse_port = False
    def __init__(self, address, routes, secrets, bus, state):
        self.routes = {r['path']: r for r in routes if 'path' in r}
        self.secrets = secrets; self.bus = bus; self.state = state
        self.slots = threading.BoundedSemaphore(8); self.unauth = OrderedDict()
        self.pool = ThreadPoolExecutor(max_workers=8, thread_name_prefix='webhook-http')
        self.guard = threading.Lock(); self.futures = set(); self.connections = set(); self.stopping = False
        try:
            super().__init__(address, Handler)
        except BaseException:
            self.pool.shutdown(wait=False, cancel_futures=True)
            raise
    def server_bind(self):
        # HTTPServer's default calls getfqdn; numeric binding needs no DNS.
        TCPServer.server_bind(self)
        self.server_name, self.server_port = self.server_address
    def process_request(self, connection, address):
        old = None
        with self.guard:
            if len(self.unauth) >= 6:
                old, previous = self.unauth.popitem(last=False)
                previous['revoked'] = True
        if old is not None:
            try: old.shutdown(socket.SHUT_RDWR)
            except OSError: pass
        slot = self.slots.acquire(blocking=False)
        if self.stopping or not slot:
            if slot: self.slots.release()
            # Never write a rejection on the accept loop: close/drop is bounded.
            self.shutdown_request(connection); self.state.count('busy'); return
        lease = {'started':time.monotonic(), 'revoked':False}
        with self.guard:
            self.connections.add(connection); self.unauth[connection] = lease
        def run():
            self._lease.connection = connection; self._lease.value = lease
            try: self.finish_request(connection, address)
            except (OSError, ValueError, http.client.HTTPException): self.state.count('framing')
            finally:
                self.shutdown_request(connection)
                with self.guard:
                    self.connections.discard(connection); self.unauth.pop(connection, None)
                self.slots.release()
        future = self.pool.submit(run)
        with self.guard: self.futures.add(future)
        def complete(f):
            with self.guard: self.futures.discard(f)
        future.add_done_callback(complete)
    _lease = threading.local()
    def authenticated(self):
        with self.guard:
            if self._lease.value['revoked']: return False
            self.unauth.pop(self._lease.connection, None)
            return True
    def activity(self):
        with self.guard: return len(self.connections) - len(self.unauth), len(self.unauth)
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
        self.rfile = HeaderReader(self.rfile, self.connection, self.server._lease.value["started"])
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
        self.server.state.count(reason, route); self.server.state.log(route, 0, reason)
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
        length = headers.get('Content-Length')
        if length is None or not length.isascii() or not length.isdigit() or len(length) > 9:
            self.fail(400, 'framing'); return
        size = int(length)
        if size > 256 * 1024:
            self.fail(404, 'size'); return
        try: path(self.path)
        except Invalid:
            self.fail(404, 'authentication'); return
        route = self.server.routes.get(self.path)
        # Basic is deliberately checked before any body is read. Unknown paths
        # and bad credentials have identical unauthenticated response bytes.
        if route is None or not basic_ok(route, headers, self.server.secrets):
            self.fail(404, 'authentication'); return
        if size > route['max_bytes']:
            self.fail(413 if 'basic' in route else 404, 'size', route['name']); return
        if 'basic' in route and not self.server.authenticated():
            self.close_connection = True; return
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
        except (OSError, ValueError, UnicodeError):
            self.fail(400, 'body', route['name']); return
        if not signature_ok(route, headers, body, self.server.secrets):
            self.fail(404, 'authentication'); return
        if not self.server.authenticated():
            self.close_connection = True; return
        try: decoded = body.decode('utf-8')
        except UnicodeError:
            self.fail(400, 'body', route['name']); return
        if not self.server.state.take(route):
            self.fail(429, 'rate', route['name']); return
        try:
            record = self.server.bus.record(route['target']).lookup()
            if record.get('kind') not in ('queue', 'agent'):
                self.fail(503, 'target', route['name']); return
            self.server.bus.send(route['target'], decoded)
        except UncertainOutcome:
            self.server.state.count('unknown', route['name']); self.server.state.log(route['name'], size, 'unknown')
            self.respond(503, b'Delivery outcome unknown; not retried\n'); return
        except BusError:
            self.fail(503, 'delivery', route['name']); return
        self.server.state.count('accepted', route['name']); self.server.state.log(route['name'], size, 'accepted')
        self.respond(202, b'Accepted\n')
