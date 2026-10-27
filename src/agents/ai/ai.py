#!/usr/bin/env python3
"""One runner call, one OpenAI-compatible request; no persistent state."""
import datetime
import http.client
import json
import os
import signal
import sys
import time
import urllib.parse

from pathlib import Path
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE / 'support/agents/shared' if (HERE / 'support').exists() else HERE.parent / 'shared'))
from metadata import dispatch

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


def answer(envelope, config, key):
    if not key:
        return "error: OPENROUTER_API_KEY is not set", 78
    status, model, sent, received = "invalid", "", 0, 0
    connection = None
    timer = None
    try:
        body = envelope["body"]
        if not isinstance(body, str) or len(body.encode()) > MAX_BYTES:
            raise ProxyError("request exceeds 1048576 bytes or is not text")
        if not isinstance(config, dict) or set(config) - {"model", "base_url"}:
            raise ProxyError("Configuration accepts only model and base_url")
        is_json = body.lstrip().startswith("{")
        request = json.loads(body) if is_json else {"messages": [{"role": "user", "content": body}]}
        if not isinstance(request, dict):
            raise ProxyError("expected a chat-completions object")
        if "model" not in request:
            request["model"] = config.get("model")
        model = request["model"]
        if not isinstance(model, str) or not model:
            raise ProxyError("set Configuration model or provide a request model")
        if request.get("stream"):
            raise ProxyError("streaming is not supported")
        base = urllib.parse.urlsplit(config.get("base_url", "https://openrouter.ai/api/v1"))
        if base.scheme not in ("https", "http") or not base.hostname or base.username or base.password or base.query or base.fragment:
            raise ProxyError("base_url must be an http(s) URL without credentials, query or fragment")
        payload = json.dumps(request, ensure_ascii=False).encode()
        sent = len(payload)
        if sent > MAX_BYTES:
            raise ProxyError("request exceeds 1048576 bytes")
        remaining = 60.0
        if envelope.get("deadline") and not envelope["deadline"].startswith("0001-"):
            deadline = datetime.datetime.fromisoformat(envelope["deadline"].replace("Z", "+00:00"))
            remaining = min(remaining, deadline.timestamp() - time.time() - 0.1)
        if remaining <= 0:
            raise TimeoutError()
        # Linux runner starts this script on its main thread. A whole-request
        # alarm also bounds DNS, trickling headers and a trickling response body.
        timer = signal.signal(signal.SIGALRM, expired)
        signal.setitimer(signal.ITIMER_REAL, remaining)
        transport = http.client.HTTPSConnection if base.scheme == "https" else http.client.HTTPConnection
        connection = transport(base.hostname, base.port, timeout=remaining)
        connection.request("POST", base.path.rstrip("/") + "/chat/completions", payload,
                           {"Authorization": "Bearer " + key, "Content-Type": "application/json"})
        response = connection.getresponse()
        status = response.status
        raw = response.read(MAX_BYTES + 1)
        received = len(raw)
        if received > MAX_BYTES:
            raise ProxyError("provider answer exceeds 1048576 bytes")
        try:
            result = scrub(json.loads(raw), key)
        except (ValueError, UnicodeError):
            if status >= 300:
                return provider_error(status, redact(raw.decode("utf-8", errors="replace"), key)), 0
            raise ProxyError("provider did not return JSON") from None
        if status >= 300:
            error = result.get("error", result) if isinstance(result, dict) else result
            message = error.get("message", error) if isinstance(error, dict) else error
            return provider_error(status, message), 0
        if is_json:
            output = json.dumps(result, ensure_ascii=False)
        else:
            try:
                output = result["choices"][0]["message"]["content"]
            except (KeyError, IndexError, TypeError):
                raise ProxyError("provider answer has no assistant text") from None
            if not isinstance(output, str):
                raise ProxyError("provider answer has no assistant text")
        if len(output.encode()) > MAX_BYTES:
            raise ProxyError("provider answer exceeds 1048576 bytes")
        return output, 0
    except TimeoutError:
        status = "timeout"
        return "error: provider request timed out", 0
    except ProxyError as error:
        return "error: " + redact(str(error), key), 0
    except (ValueError, TypeError, KeyError, AttributeError):
        # Only our own validation messages are emitted; decoder/URL exceptions
        # can contain input. Keep those classes generic.
        return "error: invalid request, Configuration or provider response", 0
    except (OSError, http.client.HTTPException):
        status = "unavailable"
        return "error: provider connection failed", 0
    finally:
        if timer is not None:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, timer)
        if connection is not None:
            connection.close()
        # Never log exceptions, prompts, answers, URLs or provider error text.
        print(json.dumps({"caller": redact(str(envelope.get("from", "unknown")), key)[:512], "model": redact(model, key) if isinstance(model, str) else "invalid",
                          "request_bytes": sent, "answer_bytes": received, "status": status}), file=sys.stderr)


def main():
    result = dispatch(HERE, os.environ.get('AGENT_BUS_ASK'))
    if result is not None:
        print(result)
        return 0
    key = os.environ.get("OPENROUTER_API_KEY", "")
    if not key:
        print("OPENROUTER_API_KEY is not set", file=sys.stderr)
        print("error: OPENROUTER_API_KEY is not set")
        return 78
    try:
        # The envelope adds escaping and labels to the bounded body.
        raw = sys.stdin.buffer.read(8 * MAX_BYTES + 1)
        if len(raw) > 8 * MAX_BYTES:
            raise ValueError()
        envelope = json.loads(raw)
        config = json.loads(os.environ.get("AGENT_BUS_CONFIG", "{}"))
        output, code = answer(envelope, config, key)
    except (ValueError, TypeError):
        output, code = "error: invalid message or Configuration", 0
    print(output)
    return code


if __name__ == "__main__":
    sys.exit(main())
