#!/usr/bin/env python3
"""File TODO audit, with independent PHP file statistics."""
import hashlib
import json
import os
from pathlib import Path
import signal
import sys
import threading

from agent_bus import Bus, CallReply

NAME = "#stats-python@localhost"


def file_data(request):
    root = Path(os.environ["STATS_ROOT"]).resolve(strict=True)
    path = Path(request["path"])
    if not path.is_absolute():
        raise ValueError("path must be absolute")
    path = path.resolve(strict=True)
    path.relative_to(root)
    return path, path.read_bytes()


def answer(bus, target, topic, request):
    outcome = bus.call(target, json.dumps(request), topic=topic, timeout=20)
    if not isinstance(outcome, CallReply):
        raise ValueError("peer completed without an answer")
    result = json.loads(outcome.message.body)
    if "error" in result:
        raise ValueError("peer refused the file")
    return result


def handle(bus, message):
    try:
        request = json.loads(message.body)
        if message.topic == "relay":
            route = list(request["route"])
            hops = [*request["hops"], "python"]
            result = {**request, "route": route[1:], "hops": hops} if route else {**request, "hops": hops}
            return json.dumps(answer(bus, route[0], "relay", result) if route else result)
        path, data = file_data(request)
        served = bus.kv.inc("files", by=1)
        digest = hashlib.sha256(data).hexdigest()
        if message.topic == "stats":
            result = {"worker": "python", "path": str(path), "bytes": len(data),
                      "lines": data.count(b"\n") + int(bool(data) and not data.endswith(b"\n")),
                      "todos": data.count(b"TODO"), "sha256": digest}
        elif message.topic == "audit":
            stats = answer(bus, "#stats-php@localhost", "stats", {"path": str(path)})
            if stats["sha256"] != digest:
                raise ValueError("file changed during audit")
            result = {"worker": "python", "path": str(path), "stats": stats,
                      "findings": [{"line": n, "text": line} for n, line in
                                   enumerate(data.decode("utf-8", errors="replace").splitlines(), 1)
                                   if "TODO" in line]}
        else:
            raise ValueError("unknown topic")
        result["served"] = served
        return json.dumps(result)
    except (OSError, ValueError, KeyError, TypeError):
        return json.dumps({"worker": "python", "error": "invalid request or unavailable file"})


def main():
    with Bus(os.environ["AGENT_BUS_NAME"]) as bus:
        if "--demo" in sys.argv:
            root = Path(os.environ["STATS_ROOT"])
            path = (Path(os.environ["SAMPLES"]) / "notes.txt" if "SAMPLES" in os.environ else
                    Path(os.environ.get("DEMO_FILE", str(root / "sample.json")))).resolve()
            result = answer(bus, NAME, "audit", {"path": str(path)})
            assert result["worker"] == "python" and result["stats"]["worker"] == "php", result
            expected = [{"line": n, "text": line} for n, line in
                        enumerate(path.read_text().splitlines(), 1) if "TODO" in line]
            assert result["findings"] == expected, "audit lost TODO line findings"
            refused = bus.call(NAME, json.dumps({"path": "/etc/passwd"}), topic="stats", timeout=20)
            assert isinstance(refused, CallReply) and "error" in json.loads(refused.message.body), "path jail escaped"
            print(json.dumps(result))
            return
        stop = threading.Event()
        signal.signal(signal.SIGTERM, lambda *_: stop.set())
        signal.signal(signal.SIGINT, lambda *_: stop.set())
        bus.serve(lambda message: handle(bus, message), workers=2, stop=stop, drain_timeout=25)


if __name__ == "__main__":
    main()
