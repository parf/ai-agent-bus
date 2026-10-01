import concurrent.futures
import hashlib
import http.server
import json
import os
from pathlib import Path
import select
import socket
import ssl
import subprocess
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

from agent_bus import (Bus, BusClosed, BusError, BusTimeout, CallDone, CallReply,
                       IdentityMismatch, InvalidArgument, ReaderInUse, Refusal, UncertainOutcome)

ROOT = Path(__file__).resolve().parents[4]
FIXTURE = Path(os.environ.get("AB_CLIENT_FIXTURE", ROOT / "tmp/client-fixture"))


class LiveClient(unittest.TestCase):
    def setUp(self):
        if not FIXTURE.exists():
            self.fail("build src/clients/fixture first (see README)")
        self.scratch = tempfile.TemporaryDirectory(dir=ROOT / "tmp")
        self.unix = str(Path(self.scratch.name) / "user-fixture.sock")
        self.process = subprocess.Popen([str(FIXTURE), self.unix], stdin=subprocess.PIPE,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        ready, _, _ = select.select([self.process.stdout], [], [], 5)
        if not ready:
            self.process.kill()
            self.fail("fixture startup timed out")
        self.info = json.loads(self.process.stdout.readline())
        self.buses = []
        self.servers = []
        self.owner = self.bus("owner@fixture")
        self.caller = self.bus("#caller@fixture")
        self.worker = self.bus("#worker@fixture")

    def bus(self, name):
        bus = Bus(name, addr=self.info["addr"], token=self.info["tokens"][name], timeout=3)
        self.buses.append(bus)
        return bus

    def serve(self, bus, handler, **options):
        stop = threading.Event()
        result = concurrent.futures.Future()
        def run():
            try:
                bus.serve(handler, stop=stop, drain_timeout=2, **options)
                result.set_result(None)
            except BaseException as exc:
                result.set_exception(exc)
        thread = threading.Thread(target=run, daemon=True)
        self.servers.append((stop, thread, result))
        thread.start()
        end = time.monotonic() + 1
        while not bus._serving and time.monotonic() < end:
            time.sleep(0.005)
        return stop, thread, result

    def tearDown(self):
        for stop, _, _ in self.servers:
            stop.set()
        for _, thread, _ in self.servers:
            thread.join(3)
        for bus in self.buses:
            bus.close()
        for _, thread, _ in self.servers:
            self.assertFalse(thread.is_alive(), "serving thread leaked")
        self.process.stdin.close()
        self.process.wait(timeout=5)
        self.process.stdout.close()
        self.process.stderr.close()
        self.scratch.cleanup()

    def test_identity_checked_before_write_and_roles_are_received(self):
        bad = Bus("#worker@fixture", addr=self.info["addr"], token=self.info["tokens"]["#caller@fixture"])
        self.buses.append(bad)
        with self.assertRaises(IdentityMismatch):
            bad.send("#worker@fixture", "must not send")
        self.assertIsNone(self.worker.consume(wait=0))
        self.assertIn("version", self.owner.identity())
        with self.assertRaises(InvalidArgument):
            self.owner.send("#worker@fixture", "x", roles=["owner"])
        sent = self.owner.send("#worker@fixture", "work", topic="build", tag="one")
        message = self.worker.consume(wait=0)
        self.assertEqual(message.id, sent)
        self.assertEqual(message.roles, ("owner",))
        with self.assertRaises(TypeError):
            message.envelope["body"] = "forged"
        message.ack()
        ack = self.owner.consume(wait=0)
        self.assertEqual((ack.receipt, ack.re, ack.topic, ack.tag), ("ack", sent, "build", "one"))
        with self.assertRaises(InvalidArgument):
            ack.ack()

    def test_private_authority_copy_and_single_fetch(self):
        original = self.worker._request
        count = 0
        count_lock = threading.Lock()
        def slow(method, path, *args, **kwargs):
            nonlocal count
            if path == "/config":
                with count_lock:
                    count += 1
                time.sleep(0.04)
            return original(method, path, *args, **kwargs)
        with patch.object(self.worker, "_request", slow):
            with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
                values = list(pool.map(lambda _: self.worker.config, range(8)))
        self.assertEqual(count, 1)
        values[0]["label"] = "changed"
        self.assertEqual(self.worker.config["label"], "fixture")
        secret = self.worker.secret
        self.assertEqual(secret.reveal(), b"TOKEN=dummy-fixture-only\n")
        self.assertNotIn("dummy", str(secret))
        self.assertNotIn("dummy", repr(secret))
        self.assertEqual(self.owner.record("#worker@fixture").secret.reveal(), secret.reveal())
        with self.assertRaises(Refusal) as denied:
            _ = self.caller.record("#worker@fixture").config
        self.assertEqual(denied.exception.status, 403)
        self.assertIsNone(self.caller.config)
        with self.assertRaises(Refusal) as missing:
            _ = self.caller.secret
        self.assertEqual(missing.exception.status, 404)

    def test_refresh_during_fetch_and_failed_read_not_cached(self):
        ready, finish = threading.Event(), threading.Event()
        original = self.worker._request
        first = True
        def slow(method, path, *args, **kwargs):
            nonlocal first
            answer = original(method, path, *args, **kwargs)
            if path == "/config" and first:
                first = False
                ready.set()
                self.assertTrue(finish.wait(2))
            return answer
        with patch.object(self.worker, "_request", slow), concurrent.futures.ThreadPoolExecutor() as pool:
            old = pool.submit(lambda: self.worker.config)
            self.assertTrue(ready.wait(1))
            self.worker.refresh("config")
            self.owner._request("POST", "/configure", {"name": "#worker@fixture", "config": {"label": "new"}})
            self.assertEqual(self.worker.config["label"], "new")
            finish.set()
            self.assertEqual(old.result(timeout=2)["label"], "fixture")
            self.assertEqual(self.worker.config["label"], "new")
        self.worker.refresh("config")
        with patch.object(self.worker, "_request", side_effect=Refusal(403, "denied")):
            with self.assertRaises(Refusal):
                _ = self.worker.config
        self.assertEqual(self.worker.config["label"], "new")

    def test_typed_kv_atomic_ops_modes_and_namespaces(self):
        kv = self.worker.kv
        for value in ("", "hello", b"", b"\xff\x00"):
            kv.set("bytes", value)
            expected = "" if value == b"" else value
            self.assertEqual(kv.get("bytes"), expected)
        kv.set("same", (1 << 63) - 1, kind="int")
        self.assertEqual(kv.get("same", kind="int"), (1 << 63) - 1)
        kv.set("same", "separate")
        self.assertEqual(kv.get("same"), "separate")
        with self.assertRaises(InvalidArgument):
            kv.set("overflow", 1 << 63, kind="int")
        self.assertEqual(kv.inc("counter", by=3), 3)
        kv.set("state", {"n": 1, "items": []}, kind="json")
        result = kv.json("state", {"op": "inc", "key": "n", "value": 2},
                         {"op": "push", "key": "items", "value": "x"})
        self.assertEqual(len(result), 2)
        self.assertEqual(kv.get("state", kind="json"), {"n": 3, "items": ["x"]})
        with self.assertRaises(Refusal):
            kv.json("state", {"op": "set", "key": "n", "value": 99}, {"op": "invalid", "key": "n"})
        self.assertEqual(kv.get("state", kind="json")["n"], 3)
        with self.assertRaises(Refusal) as exists:
            kv.set("state", {}, kind="json", mode="add")
        self.assertEqual(exists.exception.status, 412)
        self.assertEqual(set(kv.list()), {"string", "int", "json"})
        self.assertTrue(kv.delete("same"))
        with self.assertRaises(Refusal) as missing:
            kv.get("same")
        self.assertEqual(missing.exception.status, 404)

    def test_registry_locks_and_force_release(self):
        record = self.owner.record("#temp@fixture")
        record.register(kind="agent", descr="first", allow=["*"])
        with self.assertRaises(Refusal):
            record.register(kind="agent")
        self.assertEqual(record.manage(descr="second")["descr"], "second")
        self.assertEqual(record.lookup()["descr"], "second")
        self.assertIn("#temp@fixture", [r["name"] for r in self.owner.records(kind="agent")])
        locks = self.owner.record("jobs@fixture").locks
        self.owner.record("jobs@fixture").manage(maintainers=["#worker@fixture"])
        lease = locks.acquire("build", ttl=1)
        self.assertEqual(locks.holders()["build"]["holder"], "owner@fixture")
        with self.assertRaises(Refusal) as busy:
            locks.try_acquire("build", ttl=1)
        self.assertEqual(busy.exception.status, 409)
        lease.extend(2)
        self.assertEqual(self.worker.record("jobs@fixture").locks.force_release("build"), "owner@fixture")
        # The stale helper cannot know force-release happened: no fencing is promised.
        newer = locks.acquire("build", ttl=1)
        newer.release()
        with locks.acquire("scoped", ttl=1):
            self.assertIn("scoped", locks.holders())
        self.assertNotIn("scoped", locks.holders())
        record.unregister()
        with self.assertRaises(Refusal):
            record.lookup()

    def test_calls_parallel_full_capacity_and_alternate_reply_inbox(self):
        helper = self.bus("#helper@fixture")
        self.serve(helper, lambda m: "helper:" + m.body)
        def work(message):
            self.assertEqual(self.worker.config["workers"], 1)
            result = self.worker.call("#helper@fixture", message.body, timeout=2)
            return result.message.body
        self.serve(self.worker, work, workers=1)
        with self.assertRaises(ReaderInUse):
            self.worker.consume(wait=0)
        receipts = []
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
            futures = [pool.submit(self.caller.call, "#worker@fixture", str(i), timeout=3,
                on_receipt=lambda m: receipts.append((m.receipt, self.caller.status()["you"]))) for i in range(4)]
            results = [future.result(timeout=4) for future in futures]
        self.assertEqual([r.message.body for r in results], [f"helper:{i}" for i in range(4)])
        self.assertEqual(len(receipts), 4)
        result = self.caller.call("#worker@fixture", "alt", timeout=2,
            reply_to={"name": "jobs@fixture", "topic": "return"})
        self.assertEqual(result.message.body, "helper:alt")
        self.assertEqual(result.message.topic, "return")

    def test_done_empty_answer_failure_and_no_duplicate_terminal(self):
        def work(message):
            if message.body == "done":
                return None
            if message.body == "fail":
                raise RuntimeError("test failure")
            if message.body == "explicit":
                message.reply("once")
                return "twice"
            return ""
        self.serve(self.worker, work)
        self.assertIsInstance(self.caller.call("#worker@fixture", "done", timeout=2), CallDone)
        empty = self.caller.call("#worker@fixture", "empty", timeout=2)
        self.assertIsInstance(empty, CallReply)
        self.assertEqual(empty.message.body, "")
        self.assertEqual(self.caller.call("#worker@fixture", "explicit", timeout=2).message.body, "once")
        with self.assertRaises(BusTimeout) as timed:
            self.caller.call("#worker@fixture", "fail", timeout=0.25)
        self.assertEqual([m.receipt for m in timed.exception.receipts], ["ack"])
        self.assertIsNone(self.caller.consume(wait=0))

    def test_expired_request_is_neither_run_nor_acknowledged(self):
        handled = []
        self.caller.send("#worker@fixture", "expired", tag="expired", wait="1ms")
        time.sleep(0.03)
        self.serve(self.worker, lambda m: handled.append(m.body))
        self.caller.send("#worker@fixture", "barrier")
        end = time.monotonic() + 2
        while not handled and time.monotonic() < end:
            time.sleep(0.01)
        self.assertEqual(handled, ["barrier"])
        receipts = []
        while (message := self.caller.consume(wait=0)) is not None:
            receipts.append(message)
        self.assertTrue(receipts)
        self.assertTrue(all(message.tag != "expired" for message in receipts))

    def test_no_prefetch_and_stop_waits_for_taken_poll(self):
        entered, release = threading.Event(), threading.Event()
        original = self.worker._consume
        def delayed(**options):
            result = original(**options)
            if result:
                entered.set()
                self.assertTrue(release.wait(2))
            return result
        self.caller.send("#worker@fixture", "first")
        handled = []
        with patch.object(self.worker, "_consume", delayed):
            stop, thread, result = self.serve(self.worker, lambda m: handled.append(m.body))
            self.assertTrue(entered.wait(1))
            self.caller.send("#worker@fixture", "second")
            stop.set()
            time.sleep(0.08)
            self.assertTrue(thread.is_alive(), "stop dropped a message already taken by the poll")
            release.set()
            thread.join(2)
            result.result(timeout=1)
        self.assertEqual(handled, ["first"])
        self.assertEqual(self.worker.consume(wait=0).body, "second")

    def test_full_workers_do_not_take_extra_work(self):
        entered, release = threading.Event(), threading.Event()
        def handle(message):
            entered.set()
            self.assertTrue(release.wait(2))
            return "done"
        self.serve(self.worker, handle, workers=1)
        self.caller.send("#worker@fixture", "first")
        self.assertTrue(entered.wait(1))
        self.caller.send("#worker@fixture", "queued")
        try:
            time.sleep(0.12)
            self.assertEqual(self.worker.record(self.worker.name).lookup()["queued"], 1,
                             "dispatcher consumed work without a free worker")
        finally:
            release.set()

    def test_wrong_receipt_cannot_complete_call(self):
        original = self.caller._request
        def forged(method, path, body=None, **options):
            result = original(method, path, body, **options)
            if path == "/send" and body.get("to") == self.worker.name:
                self.worker._request("POST", "/send", {"to": self.caller.name, "body": "",
                    "topic": body.get("topic", ""), "tag": body["tag"], "receipt": "done",
                    "re": "ffffffffffffffff"})
            return result
        self.serve(self.worker, lambda message: "real answer")
        with patch.object(self.caller, "_request", forged):
            result = self.caller.call(self.worker.name, "question", timeout=2)
        self.assertIsInstance(result, CallReply)
        self.assertEqual(result.message.body, "real answer")

    def test_late_answer_is_never_handler_work(self):
        with self.assertRaises(BusTimeout):
            self.caller.call(self.worker.name, "timed out", timeout=0.02, topic="old-call")
        expired = self.worker.consume(wait=1)
        # Let the timed-out call's last filtered poll finish before serving.
        time.sleep(0.1)
        bodies = []
        handled = threading.Event()
        def handler(message):
            bodies.append(message.body)
            handled.set()
        self.serve(self.caller, handler)
        self.owner._request("POST", "/send", {"to": self.caller.name, "body": "late answer",
            "re": "ffffffffffffffff", "topic": "old", "tag": "old"})
        self.worker.send(self.caller.name, "legacy late answer", topic=expired.topic, tag=expired.tag)
        self.owner.send(self.caller.name, "work")
        self.assertTrue(handled.wait(2))
        self.assertEqual(bodies, ["work"])
        request_id = self.owner.send(self.worker.name, "question", topic="reply-re", tag="one")
        request = self.worker.consume(wait=1)
        request.reply("answer")
        answer = self.owner.consume(topic="reply-re", tag="one", wait=1)
        self.assertEqual(answer.re, request_id)

    def test_expired_lease_does_not_release_new_hold(self):
        locks = self.worker.locks
        old = locks.acquire("reused", ttl=0.04)
        time.sleep(0.08)
        newer = locks.acquire("reused", ttl=1)
        with self.assertRaises(InvalidArgument):
            old.release()
        self.assertIn("reused", locks.holders())
        newer.release()

    def test_unix_identity_override_and_fork_guard(self):
        local = Bus("owner@fixture", addr=self.unix, token="")
        agent = Bus("#worker@fixture", addr=self.unix, token=self.info["tokens"]["#worker@fixture"])
        self.buses.extend((local, agent))
        self.assertEqual(local.name, "owner@fixture")
        self.assertEqual(agent.name, "#worker@fixture")
        read, write = os.pipe()
        self.worker._identity_lock.acquire()
        child = os.fork()
        if child == 0:
            os.close(read)
            try:
                self.worker.status()
            except BusError:
                os.write(write, b"guarded")
            os._exit(0)
        os.close(write)
        try:
            ready, _, _ = select.select([read], [], [], 1)
            if not ready:
                os.kill(child, 9)
            self.assertTrue(ready, "fork guard reached an inherited held mutex")
            self.assertEqual(os.read(read, 20), b"guarded")
        finally:
            self.worker._identity_lock.release()
            os.waitpid(child, 0)
            os.close(read)


class TransportChecks(unittest.TestCase):
    def test_streamed_response_has_absolute_deadline(self):
        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_): pass
            def do_GET(self):
                payload = b'{"you":"#worker@fixture"}'
                self.send_response(200); self.send_header("Content-Length", str(len(payload))); self.end_headers()
                for byte in payload:
                    self.wfile.write(bytes([byte])); self.wfile.flush()
                    time.sleep(0.015)
        server = self.server(Handler)
        bus = Bus("#worker@fixture", addr=f"http://127.0.0.1:{server.server_port}", token="dummy", timeout=0.08)
        self.addCleanup(bus.close)
        started = time.monotonic()
        with self.assertRaises(BusTimeout):
            bus.status()
        self.assertLess(time.monotonic() - started, 0.25)

    def server(self, handler, tls=None):
        class QuietServer(http.server.ThreadingHTTPServer):
            def handle_error(self, request, client_address):
                pass  # deliberately rejected TLS and hung-up connections are asserted by the caller
        server = QuietServer(("127.0.0.1", 0), handler)
        if tls:
            server.socket = tls.wrap_socket(server.socket, server_side=True)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        return server

    def test_uncertain_send_is_not_replayed(self):
        posts = []
        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_): pass
            def do_GET(self):
                payload = b'{"you":"#worker@fixture"}'
                self.send_response(200); self.send_header("Content-Length", str(len(payload))); self.end_headers()
                self.wfile.write(payload)
            def do_POST(self):
                posts.append(self.rfile.read(int(self.headers["Content-Length"])))
                self.connection.shutdown(socket.SHUT_RDWR)
                self.connection.close()
        server = self.server(Handler)
        bus = Bus("#worker@fixture", addr=f"http://127.0.0.1:{server.server_port}", token="dummy")
        self.addCleanup(bus.close)
        with self.assertRaises(UncertainOutcome):
            bus.send("#other@fixture", "once")
        self.assertEqual(len(posts), 1)

    def test_tls_pin_checked_before_credentials(self):
        scratch = tempfile.TemporaryDirectory(dir=ROOT / "tmp")
        self.addCleanup(scratch.cleanup)
        key, cert = Path(scratch.name) / "key.pem", Path(scratch.name) / "cert.pem"
        subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
            "-subj", "/CN=localhost", "-keyout", str(key), "-out", str(cert)],
            check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(cert, key)
        requests = []
        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_): pass
            def do_GET(self):
                requests.append(self.headers.get("X-Agent-Bus-Token"))
                payload = b'{"you":"#worker@fixture"}'
                self.send_response(200); self.send_header("Content-Length", str(len(payload))); self.end_headers()
                self.wfile.write(payload)
        server = self.server(Handler, context)
        address = f"https://127.0.0.1:{server.server_port}"
        wrong = Bus("#worker@fixture", addr=address, token="dummy-sensitive", fingerprint="0" * 64)
        self.addCleanup(wrong.close)
        with self.assertRaises(BusError):
            wrong.status()
        self.assertEqual(requests, [], "pin mismatch transmitted an HTTP credential")
        pin = hashlib.sha256(ssl.PEM_cert_to_DER_cert(cert.read_text())).hexdigest()
        right = Bus("#worker@fixture", addr=address, token="dummy-sensitive", fingerprint="sha256:" + pin)
        self.addCleanup(right.close)
        self.assertEqual(right.name, "#worker@fixture")
        self.assertEqual(requests, ["dummy-sensitive"])


if __name__ == "__main__":
    unittest.main()
