"""HTTP over TCP/TLS or Unix sockets; one bounded connection per request."""
import hashlib
import http.client
import json
import math
import os
import pwd
import socket
import ssl
import stat
import threading
import time
from urllib.parse import urlsplit

from .errors import BusClosed, BusTimeout, InvalidArgument, Refusal, TransportError, UncertainOutcome


def seconds(value, *, positive=False):
    if isinstance(value, bool) or not isinstance(value, (float, int)) or not math.isfinite(value):
        raise InvalidArgument("duration is a finite number of seconds")
    if value < 0 or positive and value == 0:
        raise InvalidArgument("invalid duration")
    return float(value)


def duration(value):
    return f"{seconds(value):.9f}s"


def discover_socket(token):
    dirs = []
    if os.environ.get("XDG_RUNTIME_DIR"):
        dirs.append(os.path.join(os.environ["XDG_RUNTIME_DIR"], "agent-bus"))
    dirs.append("/run/agent-bus")
    account = pwd.getpwuid(os.getuid()).pw_name
    for directory in dirs:
        path = os.path.join(directory, "bus.sock" if token else f"user-{account}.sock")
        try:
            if stat.S_ISSOCK(os.stat(path).st_mode):
                return path
        except OSError:
            pass
    runtime = os.environ.get("XDG_RUNTIME_DIR")
    return os.path.join(runtime, "agent-bus", "bus.sock") if runtime else f"/tmp/agent-bus-{os.getuid()}.sock"


class UnixConnection(http.client.HTTPConnection):
    def __init__(self, path, timeout):
        super().__init__("localhost", timeout=timeout)
        self.path = path

    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(self.timeout)
        self.sock.connect(self.path)


class Transport:
    def __init__(self, addr, token, timeout, fingerprint=None):
        self.token = token
        self.addr = addr
        self.timeout = seconds(timeout, positive=True)
        self.pin = (fingerprint or "").strip().lower().removeprefix("sha256:").replace(":", "")
        if self.pin and (len(self.pin) != 64 or any(c not in "0123456789abcdef" for c in self.pin)):
            raise InvalidArgument("TLS fingerprint is SHA-256 hex")
        self.url = urlsplit(addr) if addr.startswith(("http://", "https://")) else None
        if self.url and (not self.url.hostname or self.url.username or self.url.password or self.url.query or self.url.fragment):
            raise InvalidArgument("address must be an HTTP(S) origin")
        self._active = set()
        self._lock = threading.Lock()
        self._closed = False

    def request(self, method, path, body=None, *, timeout=None, raw=False, headers=None, destructive=False):
        # The Bus bounds a given timeout; one held open for a wait may exceed the default.
        timeout = seconds(timeout, positive=True) if timeout is not None else self.timeout
        deadline = time.monotonic() + timeout
        if self.url:
            u = self.url
            if u.scheme == "https":
                context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
                if self.pin:
                    context.check_hostname = False
                    context.verify_mode = ssl.CERT_NONE  # replaced by exact peer-certificate pin below
                else:
                    context.load_default_certs()
                connection = http.client.HTTPSConnection(u.hostname, u.port, timeout=timeout, context=context)
            else:
                connection = http.client.HTTPConnection(u.hostname, u.port, timeout=timeout)
            path = u.path.rstrip("/") + path
        else:
            connection = UnixConnection(self.addr, timeout)
        encoded = None if body is None else json.dumps(body, ensure_ascii=False, allow_nan=False).encode()
        sent = False
        timer = None
        with self._lock:
            if self._closed:
                raise BusClosed("client is closed")
            self._active.add(connection)
        try:
            connection.connect()
            if self.url and self.url.scheme == "https" and self.pin:
                actual = hashlib.sha256(connection.sock.getpeercert(binary_form=True)).hexdigest()
                if actual != self.pin:
                    raise TransportError("daemon TLS certificate does not match the pin")
            left = deadline - time.monotonic()
            if left <= 0:
                raise BusTimeout()
            connection.sock.settimeout(left)
            # A socket timeout bounds each read, not a whole slowly streamed response.
            # Shut down this request's socket at the absolute deadline as well.
            request_socket = connection.sock
            def expire():
                try:
                    request_socket.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass
            timer = threading.Timer(left, expire)
            timer.daemon = True
            timer.start()
            # Set this before writing: a partial send is already an uncertain write.
            sent = True
            connection.request(method, path, body=encoded, headers={
                "X-Agent-Bus-Token": self.token, "Content-Type": "application/json", **(headers or {})})
            response = connection.getresponse()
            data = response.read()
            if time.monotonic() >= deadline:
                if destructive or method != "GET":
                    raise UncertainOutcome("operation exceeded its deadline after sending")
                raise BusTimeout()
            if response.status == 204:
                return None
            if not 200 <= response.status < 300:
                try:
                    message = json.loads(data).get("error", response.reason)
                except (ValueError, AttributeError):
                    message = response.reason
                raise Refusal(response.status, message)
            if raw:
                return data
            try:
                return json.loads(data) if data else None
            except (ValueError, UnicodeError) as exc:
                if destructive or method != "GET":
                    raise UncertainOutcome("operation answered with an invalid response") from exc
                raise TransportError("daemon returned invalid JSON") from exc
        except (OSError, http.client.HTTPException) as exc:
            if sent and (destructive or method != "GET"):
                raise UncertainOutcome("operation may have completed remotely") from exc
            if isinstance(exc, TimeoutError):
                raise BusTimeout() from exc
            if time.monotonic() >= deadline:
                raise BusTimeout() from exc
            raise TransportError("could not reach the daemon") from exc
        finally:
            if timer:
                timer.cancel()
            with self._lock:
                self._active.discard(connection)
            connection.close()

    def close(self):
        with self._lock:
            self._closed = True
            active = list(self._active)
        for connection in active:
            if connection.sock:
                try:
                    connection.sock.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass
            connection.close()
