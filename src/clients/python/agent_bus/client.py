"""One authenticated caller with a demand-driven inbox dispatcher."""
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass, field
from datetime import datetime, timezone
import logging
import os
import threading
import time
import uuid
from urllib.parse import urlencode

from .errors import (BusClosed, BusError, BusTimeout, DrainTimeout, IdentityMismatch,
                     InvalidArgument, ReaderInUse, UnexpectedAnswer)
from .message import CallDone, CallReply, Message
from .storage import Record
from .transport import Transport, discover_socket, duration, no_wait, seconds

_log = logging.getLogger("agent_bus")


@dataclass
class Pending:
    key: tuple
    on_receipt: object
    event: threading.Event = field(default_factory=threading.Event)
    receipts: list = field(default_factory=list)
    queued_receipts: list = field(default_factory=list)
    result: object = None
    error: object = None
    sent_id: str = ""
    early: list = field(default_factory=list)
    request: object = None


@dataclass
class Serving:
    handler: object
    inbox: str
    topic: object
    share: bool
    workers: int
    executor: object
    stop: threading.Event = field(default_factory=threading.Event)
    futures: set = field(default_factory=set)
    error: object = None
    accepting: bool = True
    polling: int = 0


class Bus:
    def __init__(self, name=None, *, addr=None, token=None, timeout=30, fingerprint=None):
        self.timeout = seconds(timeout, positive=True)
        self._expected = (name if name is not None else os.environ.get("AGENT_BUS_NAME", "")).strip().lower()
        token = os.environ.get("AGENT_BUS_TOKEN", "") if token is None else token
        addr = addr or os.environ.get("AGENT_BUS_ADDR") or discover_socket(token)
        self._transport = Transport(addr, token, timeout,
            fingerprint if fingerprint is not None else os.environ.get("AGENT_BUS_TLS_FINGERPRINT"))
        self._pid = os.getpid()
        self._name = None
        self._identity_lock = threading.Lock()
        self._condition = threading.Condition()
        self._closed = False
        self._records = {}
        self._pending = {}
        self._serving = None
        self._raw = False
        self._dispatcher = None
        self._cursor = 0
        self._call_prefix = "call-" + uuid.uuid4().hex + "-"

    def _check(self):
        # Check PID before touching inherited locks, which can be permanently held after fork.
        if os.getpid() != self._pid:
            raise BusError("construct a new Bus after fork")
        if self._closed:
            raise BusClosed("client is closed")

    def _ensure_identity(self, deadline):
        self._check()
        if self._name is not None:
            return
        left = deadline - time.monotonic()
        if left <= 0 or not self._identity_lock.acquire(timeout=left):
            raise BusTimeout("identity check timed out")
        try:
            if self._name is None:
                left = deadline - time.monotonic()
                if left <= 0:
                    raise BusTimeout("identity check timed out")
                status = self._transport.request("GET", "/status", timeout=left)
                actual = status["you"]
                if self._expected and actual != self._expected:
                    raise IdentityMismatch(f"expected {self._expected}, credential acts as {actual}")
                self._name = actual
        finally:
            self._identity_lock.release()

    def _request(self, method, path, body=None, *, query=None, timeout=None, raw=False,
                 headers=None, destructive=False, wait=0):
        self._check()
        budget = self.timeout if timeout is None else min(self.timeout, seconds(timeout, positive=True))
        # A request the daemon holds open (consume, lock) gets its wait on top of the budget.
        budget += min(seconds(wait), 60)
        deadline = time.monotonic() + budget
        if path != "/identity":
            self._ensure_identity(deadline)
        left = deadline - time.monotonic()
        if left <= 0:
            raise BusTimeout()
        if query:
            path += "?" + urlencode({key: value for key, value in query.items() if value is not None})
        return self._transport.request(method, path, body, timeout=left, raw=raw,
                                       headers=headers, destructive=destructive)

    @property
    def name(self):
        self._ensure_identity(time.monotonic() + self.timeout)
        return self._name

    def identity(self):
        return self._request("GET", "/identity")

    def status(self):
        return self._request("GET", "/status")

    def may_send(self, who, name):
        """Whether the daemon's Send rule admits `who` sending to `name` right now.

        Asked as this caller, about a target this caller can see. True or False
        is the daemon's answer and nothing else: a target the caller cannot see
        raises Refusal (404), and anything that is not a well-formed answer for
        exactly this `who` (a transport failure, a timeout, an older daemon that
        ignores `who` and answers for the caller) raises, never returns False.
        `who` and `name` are trimmed and lower-cased, as the daemon reads them,
        and the daemon must echo exactly that `who` (a name without a realm stays
        without one). A refusal (404) is `Refusal`, a foreign answer is
        `UnexpectedAnswer`: agents tell "no such destination" from "daemon too
        old". It is a check that goes stale at once; ask again before each send.
        """
        if not isinstance(who, str) or not who.strip() or not isinstance(name, str) or not name.strip():
            raise InvalidArgument("may_send wants who and a target name")
        who = who.strip().lower()
        got = self._request("GET", "/may-send", query={"who": who, "name": name.strip()})
        echoed = got.get("who") if isinstance(got, dict) else None
        if not isinstance(echoed, str) or not isinstance(got.get("may_send"), bool) or echoed != who:
            raise UnexpectedAnswer("may-send answered for another name or in an unknown shape: is the daemon older than RT33?")
        return got["may_send"]

    def records(self, **filters):
        # /ls accepts kind; other daemon listing views are deliberately separate.
        if set(filters) - {"kind"}:
            raise InvalidArgument("registry listing supports kind")
        return self._request("GET", "/ls", query=filters)

    def record(self, name):
        self._check()
        if not isinstance(name, str) or not name.strip():
            raise InvalidArgument("record name is required")
        name = name.strip().lower()
        with self._condition:
            if name not in self._records:
                self._records[name] = Record(self, name)
            return self._records[name]

    @property
    def config(self):
        return self.record(self.name).config

    @property
    def secret(self):
        return self.record(self.name).secret

    @property
    def kv(self):
        return self.record(self.name).kv

    @property
    def locks(self):
        return self.record(self.name).locks

    def refresh(self, *names):
        return self.record(self.name).refresh(*names)

    def send(self, to, body, **options):
        allowed = {"topic", "tag", "ttl", "wait", "reply_to", "ask"}
        if set(options) - allowed:
            raise InvalidArgument("unsupported send option")
        if not isinstance(body, str):
            raise InvalidArgument("message body must be text")
        if "wait" in options and no_wait(options["wait"]):
            del options["wait"]
        for key in ("ttl", "wait"):
            if key in options and isinstance(options[key], (int, float)):
                options[key] = duration(options[key])
        return self._request("POST", "/send", {"to": to, "body": body, **options})["message_id"]

    def publish(self, channel, body, **options):
        return self.send(channel, body, **options)

    def unsubscribe(self, channel):
        return self._request("POST", "/subscribe", {"channel": channel, "off": True})

    def _respond(self, message, body, receipt=None):
        if not isinstance(message, Message) or message._bus is not self:
            raise InvalidArgument("response needs a message received by this Bus")
        if message.receipt:
            raise InvalidArgument("do not acknowledge or reply to receipts")
        if not isinstance(body, str):
            raise InvalidArgument("reply body must be text")
        route = message.reply_to or {}
        to = route.get("name", message.envelope["from"])
        payload = {"to": to, "body": body}
        for key in ("topic", "tag"):
            value = route.get(key, message.envelope.get(key))
            if value is not None:
                payload[key] = value
        if receipt:
            payload["receipt"] = receipt
        payload["re"] = message.id
        with message._completion.lock:
            if receipt != "ack":
                if message._completion.terminal:
                    raise InvalidArgument("message already has a terminal response")
                message._completion.terminal = True
            return self._request("POST", "/send", payload)["message_id"]

    def reply(self, message, body):
        return self._respond(message, body)

    def ack(self, message):
        return self._respond(message, "", "ack")

    def done(self, message):
        return self._respond(message, "", "done")

    def _consume(self, *, inbox, topic=None, tag=None, wait=0.05, share=False, timeout=None):
        query = {"inbox": inbox, "topic": topic, "tag": tag, "wait": duration(wait)}
        if share:
            query["share"] = "1"  # daemon tests presence, not a Boolean value
        data = self._request("GET", "/consume", query=query,
                             timeout=timeout, wait=wait, destructive=True)
        return Message.received(data, self) if data is not None else None

    def consume(self, *, inbox=None, topic=None, tag=None, wait=30, share=False):
        self._check()
        if seconds(wait) > 60:
            raise InvalidArgument("consume wait <= 60s")
        with self._condition:
            if self._raw or self._serving or self._pending:
                raise ReaderInUse("raw consume conflicts with another reader mode")
            self._raw = True
        try:
            return self._consume(inbox=inbox or self.name, topic=topic, tag=tag, wait=wait, share=share)
        finally:
            with self._condition:
                self._raw = False
                self._condition.notify_all()

    def _start_dispatcher(self):
        # condition is held by the caller
        if self._dispatcher is None:
            self._dispatcher = threading.Thread(target=self._dispatch, name="agent-bus-inbox", daemon=True)
            self._dispatcher.start()

    def call(self, to, body, *, timeout, on_receipt=None, **options):
        deadline = time.monotonic() + seconds(timeout, positive=True)
        self._ensure_identity(min(deadline, time.monotonic() + self.timeout))
        if "tag" in options:
            raise InvalidArgument("call generates its own unique tag")
        topic = options.get("topic", "")
        tag = self._call_prefix + uuid.uuid4().hex
        route = options.get("reply_to") or {}
        inbox = route.get("name", self.name)
        reply_topic, reply_tag = route.get("topic", topic) or "", tag
        if route:
            # Unique reply correlation is preserved even on an alternate inbox; the
            # topic is sent too, since a replier reads an omitted one as none.
            options["reply_to"] = {**route, "topic": reply_topic, "tag": tag}
        pending = Pending((inbox, reply_topic, reply_tag), on_receipt)
        with self._condition:
            self._check()
            if self._raw:
                raise ReaderInUse("call conflicts with raw consume")
            self._pending[pending.key] = pending
            self._start_dispatcher()
            self._condition.notify_all()
        try:
            # The outgoing duration is bounded by the original call deadline.
            options["wait"] = duration(max(0, deadline - time.monotonic()))
            options["tag"] = tag
            send_timeout = deadline - time.monotonic()
            if send_timeout <= 0:
                raise BusTimeout()
            payload = {"to": to, "body": body, **options}
            if not isinstance(body, str) or set(options) - {"topic", "tag", "ttl", "wait", "reply_to", "ask"}:
                raise InvalidArgument("invalid call body or options")
            if isinstance(payload.get("ttl"), (int, float)):
                payload["ttl"] = duration(payload["ttl"])
            sent = self._request("POST", "/send", payload, timeout=send_timeout)
            with self._condition:
                pending.sent_id = sent["message_id"]
                for message in pending.early:
                    self._deliver(pending, message)
                pending.early.clear()
                self._condition.notify_all()
            while True:
                with self._condition:
                    receipts = pending.queued_receipts[:]
                    pending.queued_receipts.clear()
                    error, result = pending.error, pending.result
                # Callbacks run on the calling thread, never the dispatcher or under its lock.
                for receipt in receipts:
                    if on_receipt:
                        on_receipt(receipt)
                if error:
                    raise error
                left = deadline - time.monotonic()
                if left <= 0:
                    raise BusTimeout(receipts=pending.receipts)
                if result:
                    return result
                pending.event.wait(min(left, 0.05))
                pending.event.clear()
        finally:
            with self._condition:
                self._pending.pop(pending.key, None)
                self._condition.notify_all()

    def _deliver(self, pending, message):
        # condition is held. A work poll can see the response before send() returns its ID.
        if not pending.sent_id:
            pending.early.append(message)
            return
        if not message.receipt and not message.re and message.id == pending.sent_id:
            # The accepted self request can arrive before send() returns, too.
            # Classify it here so both early and ordinary delivery retain it.
            pending.request = message
            return
        if message.receipt and message.re != pending.sent_id:
            return
        if message.receipt == "ack":
            pending.receipts.append(message)
            pending.queued_receipts.append(message)
        elif message.receipt == "done":
            pending.result = CallDone(message, tuple(pending.receipts))
        elif not message.receipt:
            pending.result = CallReply(message, tuple(pending.receipts))
        pending.event.set()

    def _dispatch(self):
        while True:
            with self._condition:
                if self._closed:
                    return
                serving = self._serving
                capacity = serving and len(serving.futures) < serving.workers
                work = capacity and serving.accepting
                # A filtered self-call poll can take the request before the work
                # poll. Retain at most that one request per live call, and only
                # submit it when a serving worker is free, just like ordinary work.
                deferred = next((p for p in self._pending.values()
                                 if p.request is not None and p.request.envelope.get("to") == serving.inbox), None) if capacity else None
                if deferred is not None:
                    message, deferred.request = deferred.request, None
                    future = serving.executor.submit(self._handle, serving, message)
                    serving.futures.add(future)
                    future.add_done_callback(lambda done, owner=serving: self._finished(owner, done))
                    continue
                pending = [p for p in self._pending.values() if p.sent_id and not p.result and not p.error]
                choices = len(pending) + int(bool(work))
                if not choices:
                    self._condition.wait(0.05)
                    continue
                self._cursor %= choices
                selected = self._cursor
                self._cursor += 1
                if selected < len(pending):
                    target = pending[selected]
                    inbox, topic, tag = target.key
                    share = False
                else:
                    target = None
                    inbox, topic, tag, share = serving.inbox, serving.topic, None, serving.share
                    serving.polling += 1
            try:
                message = self._consume(inbox=inbox, topic=topic, tag=tag, share=share, timeout=1)
            except BaseException as exc:
                with self._condition:
                    if target:
                        target.error = exc
                        target.event.set()
                    elif serving is self._serving:
                        serving.polling -= 1
                        serving.error = exc
                        serving.stop.set()
                        serving.accepting = False
                    self._condition.notify_all()
                continue
            if message is None:
                if target is None:
                    with self._condition:
                        serving.polling -= 1
                        self._condition.notify_all()
                continue
            with self._condition:
                if target is None:
                    serving.polling -= 1
                # A reply arriving in an unfiltered work read still belongs to its call.
                key = (inbox, message.topic or "", message.tag or "")
                routed = self._pending.get(key)
                if (routed is None and not message.receipt and not message.re
                        and message.envelope.get("from") == self._name):
                    # An alternate reply inbox/topic changes the reply key,
                    # not the request's unique call tag on our serving inbox.
                    routed = next((p for p in self._pending.values()
                                   if p.key[2] == message.tag), None)
                if routed:
                    self._deliver(routed, message)
                elif (target is None and not (message.tag or "").startswith(self._call_prefix)
                      and not message.receipt and not message.re
                      and serving is self._serving):
                    # Poll completion can race a stop. The taken message must still be handled.
                    future = serving.executor.submit(self._handle, serving, message)
                    serving.futures.add(future)
                    future.add_done_callback(lambda done, owner=serving: self._finished(owner, done))
                self._condition.notify_all()

    def _finished(self, serving, future):
        with self._condition:
            serving.futures.discard(future)
            self._condition.notify_all()

    def _handle(self, serving, message):
        try:
            if message.deadline and not message.deadline.startswith("0001-01-01T00:00:00") and datetime.fromisoformat(message.deadline.replace("Z", "+00:00")) <= datetime.now(timezone.utc):
                _log.info("agent-bus skipped expired message %s", message.id)
                return
            message.ack()
            answer = serving.handler(message)
            if not message._completion.terminal:
                if answer is None:
                    message.done()
                else:
                    message.reply(answer)
        except Exception:
            # Avoid logging envelope/secret values or exception messages containing them.
            _log.error("agent-bus handler failed for message %s", message.id)

    def serve(self, handler, *, inbox=None, topic=None, share=False, workers=1,
              stop=None, drain_timeout=30):
        self._check()
        if not callable(handler) or isinstance(workers, bool) or not isinstance(workers, int) or workers < 1:
            raise InvalidArgument("serve requires a handler and positive worker count")
        seconds(drain_timeout)
        serving = Serving(handler, inbox or self.name, topic, share, workers,
                          ThreadPoolExecutor(max_workers=workers, thread_name_prefix="agent-bus-handler"))
        with self._condition:
            if self._raw or self._serving:
                serving.executor.shutdown(wait=False)
                raise ReaderInUse("another inbox reader is active")
            self._serving = serving
            self._start_dispatcher()
            self._condition.notify_all()
        try:
            while not serving.stop.is_set() and not (stop and stop.is_set()):
                with self._condition:
                    self._condition.wait(0.05)
            end = time.monotonic() + drain_timeout
            with self._condition:
                serving.accepting = False
                while (serving.futures or serving.polling or
                       any(p.request is not None and p.request.envelope.get("to") == serving.inbox
                           for p in self._pending.values())):
                    left = end - time.monotonic()
                    if left <= 0:
                        raise DrainTimeout(len(serving.futures))
                    self._condition.wait(left)
            if serving.error:
                raise serving.error
        finally:
            with self._condition:
                serving.accepting = False
                if self._serving is serving:
                    self._serving = None
                self._condition.notify_all()
            serving.executor.shutdown(wait=False, cancel_futures=True)

    def stop(self):
        self._check()
        with self._condition:
            if self._serving:
                self._serving.accepting = False
                self._serving.stop.set()
                self._condition.notify_all()

    def close(self):
        if os.getpid() == self._pid and self._closed:
            return
        self._check()
        with self._condition:
            self._closed = True
            if self._serving:
                self._serving.accepting = False
                self._serving.stop.set()
            for pending in self._pending.values():
                pending.error = BusClosed("client closed while waiting")
                pending.event.set()
            self._condition.notify_all()
        self._transport.close()
        if self._dispatcher and self._dispatcher is not threading.current_thread():
            self._dispatcher.join(timeout=1)

    def __enter__(self):
        self._check()
        return self

    def __exit__(self, *_):
        self.close()
