"""Small stdlib provider boundary shared by the AI and Voyage proxies."""
import datetime
import http.client
import signal
import time

MAX_BYTES = 1 << 20


class ProxyError(ValueError):
    pass


def redact(text, key):
    # Providers may echo credentials even in successful JSON or error messages.
    # JSON escaping can spell the same key differently, so inspect string values.
    return text.replace(key, "***") if key else text


def scrub(value, key):
    if isinstance(value, str):
        return redact(value, key)
    if isinstance(value, list):
        return [scrub(item, key) for item in value]
    if isinstance(value, dict):
        return {redact(name, key): scrub(item, key) for name, item in value.items()}
    return value


def provider_error(status, message):
    output = f"error: {status} {message}"
    if len(output.encode()) > MAX_BYTES:
        return f"error: {status} provider error exceeds 1048576 bytes"
    return output


def expired(_signum, _frame):
    raise TimeoutError()


def arm_deadline(envelope):
    remaining = 60.0
    if envelope.get("deadline") and not envelope["deadline"].startswith("0001-"):
        deadline = datetime.datetime.fromisoformat(envelope["deadline"].replace("Z", "+00:00"))
        remaining = min(remaining, deadline.timestamp() - time.time() - 0.1)
    if remaining <= 0:
        raise TimeoutError()
    timer = signal.signal(signal.SIGALRM, expired)
    signal.setitimer(signal.ITIMER_REAL, remaining)
    return timer, remaining


def connect(base, remaining):
    transport = http.client.HTTPSConnection if base.scheme == "https" else http.client.HTTPConnection
    return transport(base.hostname, base.port, timeout=remaining)


def post(connection, path, payload, key):
    # One POST, no redirects and no retry, even on an uncertain result.
    connection.request("POST", path, payload,
                       {"Authorization": "Bearer " + key, "Content-Type": "application/json"})
    return connection.getresponse()
