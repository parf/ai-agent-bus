#!/usr/bin/env python3
"""The same time service as a process of your own, written with the Python client.

It answers help and status, reads its format from Configuration and its key from
Secret, keeps its progress in KV under a lock, writes operational events to its
own log, and stops cleanly on SIGTERM. The body is data: only two words are read.
"""
import datetime
import hashlib
import signal
import threading

from agent_bus import Bus, Refusal

HELP = "time [now|iso|epoch] - the current UTC time, now in the configured format; history - the last five reports"


# region: process
def serve():
    stop = threading.Event()
    signal.signal(signal.SIGTERM, lambda *_: stop.set())     # graceful stop: finish what is running, then leave
    with Bus() as bus:                                        # name, token and address come from the environment
        kv, locks = bus.kv, bus.locks

        def read(key, kind="string"):                         # a missing key is not an error here
            try:
                return kv.get(key, kind=kind)
            except Refusal as refused:
                if refused.kind == "not_found":
                    return None
                raise

        def settings():
            fmt = (bus.config or {}).get("format", "iso")     # Configuration: non-secret settings
            key = dict(line.split("=", 1) for line in bus.secret.reveal().decode().splitlines() if "=" in line)
            return fmt, key.get("TIME_STAMP_KEY", "")         # Secret: read, used, never echoed

        def report(word):
            fmt, key = settings()
            word = word or "now"
            if word == "now":
                word = fmt
            if word == "history":
                return read("history") or "no reports yet"
            now = datetime.datetime.now(datetime.timezone.utc)
            if word == "iso":
                text = now.strftime("%Y-%m-%dT%H:%M:%SZ")
            elif word == "epoch":
                text = str(int(now.timestamp()))
            else:
                return "unsupported format; ask help"        # the body is data, never echoed back
            stamp = hashlib.sha256((key + text).encode()).hexdigest()[:8]
            lease = locks.acquire("report", ttl=5, wait=2)    # one report is recorded at a time
            try:
                kv.inc("reports")
                kv.set("history", "\n".join([text] + (read("history") or "").splitlines()[:4]))
            finally:
                lease.release()
            bus.send("#bus", f"write {bus.name} notice reported {word}")   # an event in my own log; no answer is expected
            return f"{text} stamp={stamp}"

        def handle(message):
            if message.ask == "help":
                return HELP
            if message.ask == "status":
                fmt, _ = settings()
                return f"reports: {read('reports', 'int') or 0}; format: {fmt}"
            if message.ask:
                return "unsupported: " + message.ask
            return report((message.body or "").strip())

        bus.serve(handle, workers=2, stop=stop, drain_timeout=5)
# endregion


if __name__ == "__main__":
    serve()
