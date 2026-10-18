"""Record-bound private reads, typed KV and named lock helpers."""
import base64
import copy
import threading
import time

from .errors import BusTimeout, InvalidArgument
from .message import Secret
from .transport import duration, seconds

_MISSING = object()


class LazyValue:
    def __init__(self):
        self.condition = threading.Condition()
        self.generation = 0
        self.value = _MISSING
        self.loading = None

    def get(self, load, timeout):
        end = time.monotonic() + timeout
        with self.condition:
            while self.loading == self.generation and self.value is _MISSING:
                left = end - time.monotonic()
                if left <= 0:
                    raise BusTimeout("private-value initialization timed out")
                self.condition.wait(left)
            if self.value is not _MISSING:
                return self.value
            generation = self.generation
            self.loading = generation
        try:
            value = load(max(0.000001, end - time.monotonic()))
        except BaseException:
            with self.condition:
                if self.loading == generation:
                    self.loading = None
                self.condition.notify_all()
            raise
        with self.condition:
            if generation == self.generation:
                self.value = value
            if self.loading == generation:
                self.loading = None
            self.condition.notify_all()
        return value

    def invalidate(self):
        with self.condition:
            self.generation += 1
            self.value = _MISSING
            self.condition.notify_all()


class Record:
    def __init__(self, bus, name):
        if not isinstance(name, str) or not name.strip():
            raise InvalidArgument("record name is required")
        self.name = name.strip().lower()
        self.bus = bus
        self.kv = RecordKV(self)
        self.locks = RecordLocks(self)
        self._config, self._secret = LazyValue(), LazyValue()

    def lookup(self):
        return self.bus._request("GET", "/lookup", query={"name": self.name})

    def register(self, *, create_only=True, **metadata):
        if "name" in metadata and metadata.pop("name") != self.name:
            raise InvalidArgument("registration cannot change the handle name")
        return self.bus._request("POST", "/register", {"name": self.name, **metadata},
                                 headers={"If-None-Match": "*"} if create_only else {})

    def manage(self, **changes):
        if "name" in changes:
            raise InvalidArgument("management cannot change the handle name")
        return self.bus._request("POST", "/manage", {"name": self.name, **changes})

    def unregister(self):
        return self.bus._request("POST", "/unregister", {"name": self.name})

    @property
    def config(self):
        self.bus._check()
        value = self._config.get(lambda timeout: self.bus._request(
            "GET", "/config", query={"name": self.name}, timeout=timeout), self.bus.timeout)
        return copy.deepcopy(value)

    @property
    def secret(self):
        self.bus._check()
        value = self._secret.get(lambda timeout: self.bus._request(
            "GET", "/secret", query={"name": self.name}, timeout=timeout, raw=True), self.bus.timeout)
        return Secret(value)

    def refresh(self, *names):
        names = names or ("config", "secret")
        if any(name not in ("config", "secret") for name in names):
            raise InvalidArgument("refresh selects config or secret")
        self.bus._check()
        for name in names:
            getattr(self, "_" + name).invalidate()


def int64(value):
    if isinstance(value, bool) or not isinstance(value, int) or not -(1 << 63) <= value < (1 << 63):
        raise InvalidArgument("integer must fit signed int64")
    return value


def kind_of(kind):
    if kind not in ("string", "int", "json"):
        raise InvalidArgument("kind is string, int or json")
    return kind


class RecordKV:
    def __init__(self, record):
        self.record = record

    def get(self, key, *, kind="string"):
        answer = self.record.bus._request("GET", "/kv", query={
            "record": self.record.name, "name": key, "kind": kind_of(kind)})
        if "value_base64" in answer:
            return base64.b64decode(answer["value_base64"], validate=True)
        return answer["value"]

    def set(self, key, value, *, kind="string", mode="set"):
        kind_of(kind)
        if mode not in ("set", "add", "replace"):
            raise InvalidArgument("mode is set, add or replace")
        body = {"record": self.record.name, "name": key, "kind": kind, "how": mode}
        if kind == "int":
            body["value"] = int64(value)
        elif kind == "json":
            if not isinstance(value, dict):
                raise InvalidArgument("JSON storage takes an object")
            body["value"] = value
        elif isinstance(value, str):
            body["value"] = value
        elif isinstance(value, bytes):
            # An empty byte string is valid, but empty value_base64 is not a wire value.
            try:
                body["value"] = value.decode("utf-8")
            except UnicodeError:
                body["value_base64"] = base64.b64encode(value).decode("ascii")
        else:
            raise InvalidArgument("string values are str or bytes")
        return self.record.bus._request("POST", "/kv/set", body)

    def delete(self, key, *, kind="string"):
        return self.record.bus._request("POST", "/kv/delete", {
            "record": self.record.name, "name": key, "kind": kind_of(kind)})["deleted"]

    def list(self):
        return self.record.bus._request("GET", "/kv/list", query={"record": self.record.name})["values"]

    def inc(self, key, *, by=1):
        return self.record.bus._request("POST", "/kv/inc", {
            "record": self.record.name, "name": key, "n": int64(by)})["value"]

    def json(self, key, *operations):
        if not 1 <= len(operations) <= 100:
            raise InvalidArgument("JSON edit takes 1 to 100 operations")
        return self.record.bus._request("POST", "/kv/json", {
            "record": self.record.name, "name": key, "ops": list(operations)})["results"]


class RecordLocks:
    def __init__(self, record):
        self.record = record

    def acquire(self, name, *, ttl, wait=0):
        return self._take("/lock", name, ttl, wait)

    def try_acquire(self, name, *, ttl):
        return self._take("/try-lock", name, ttl, 0)

    def _take(self, route, name, ttl, wait):
        seconds(ttl, positive=True)
        seconds(wait)
        if ttl > 86400 or wait > 60:
            raise InvalidArgument("lock ttl <= 24h and wait <= 60s")
        start = time.monotonic()
        self.record.bus._request("POST", route, {"record": self.record.name,
            "name": name, "ttl": duration(ttl)}, query={"wait": duration(wait)}, wait=wait)
        # Conservative estimate: the grant is no earlier than the request start.
        return Lease(self, name, start + ttl)

    def extend(self, name, *, ttl):
        if seconds(ttl, positive=True) > 86400:
            raise InvalidArgument("lock ttl <= 24h")
        return self.record.bus._request("POST", "/extend", {
            "record": self.record.name, "name": name, "ttl": duration(ttl)})

    def release(self, name):
        return self.record.bus._request("POST", "/release", {"record": self.record.name, "name": name})

    def force_release(self, name):
        return self.record.bus._request("POST", "/release-force", {
            "record": self.record.name, "name": name}).get("holder")

    def holders(self):
        return self.record.bus._request("GET", "/holders", query={"record": self.record.name})["locks"]


class Lease:
    def __init__(self, locks, name, deadline):
        self.locks, self.name, self.deadline = locks, name, deadline
        self._lock = threading.Lock()
        self._released = False

    def _check(self):
        if self._released or time.monotonic() >= self.deadline:
            raise InvalidArgument("lease is released or past its conservative expiry")

    def extend(self, ttl):
        with self._lock:
            self._check()
            start = time.monotonic()
            # An uncertain extension must not be assumed locally successful.
            result = self.locks.extend(self.name, ttl=ttl)
            self.deadline = start + ttl
            return result

    def release(self):
        with self._lock:
            self._check()
            self._released = True  # never automatically retry an uncertain release
            return self.locks.release(self.name)

    def __enter__(self):
        self._check()
        return self

    def __exit__(self, *_):
        if not self._released and time.monotonic() < self.deadline:
            self.release()
