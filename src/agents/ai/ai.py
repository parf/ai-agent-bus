#!/usr/bin/env python3
"""One runner call, one OpenAI-compatible request; no persistent state."""
import contextlib
import http.client
import json
import os
import re
import signal
import sys
import tempfile
import urllib.parse

from pathlib import Path
HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE / 'support/agents/shared' if (HERE / 'support').exists() else HERE.parent / 'shared'))
from metadata import dispatch

from provider_http import MAX_BYTES, ProxyError, redact, scrub, provider_error, arm_deadline, connect, post

MODEL_ID = r"[A-Za-z0-9._/~:@+-]{1,128}"
# The runner posts each answer to /send as a JSON envelope (answerAs, src/cmd/agent-bus-runner/serve.go), and the
# daemon reads at most 1 MiB of it (Server.read, src/internal/api/server.go): the envelope, not the body, is bounded.
ANSWER_LIMIT = MAX_BYTES
# Python's widest float spelling, -1.7976931348623157e+308, is 24 characters; a comma follows each.
FLOAT_WIDTH = 24
# Balance and key describe the account, not a model answer: the record's Owner and Maintainers only.
MANAGERS = {"owner", "maintainer"}
KEY_FIELDS = ("limit", "limit_remaining", "limit_reset", "usage", "is_free_tier", "rate_limit", "free_model_daily_requests")
DIMS_FILE = "embed-dims.json"
EMBED_READ = 8 * MAX_BYTES
DIMS_KEPT = 256
# Image requests: options the provider takes, and this agent's own caps on one call.
IMAGE_OPTIONS = {"aspect_ratio", "resolution", "size", "output_format", "quality", "n", "input_references", "background", "seed"}
IMAGE_FORMATS = {"png", "jpeg", "webp", "svg", "gif"}
IMAGE_N_MAX = 4
IMAGE_REFERENCES_MAX = 16
IMAGE_UNAVAILABLE = "error: image generation is not available yet: #ai cannot store images until it serves itself (T-36 step 2b)"


def refusal(status, message):
    """A provider error answer; a refused key or credit is named, since no retry helps."""
    if status in (401, 403):
        return f"error: the provider refused the configured key ({status}: {message})"
    if status == 402:
        return f"error: the provider refused for lack of credit (402: {message})"
    return provider_error(status, message)


def reject_constant(_value):
    raise ValueError("non-standard JSON constant")


def delivered_roles(environ):
    # Only the two exact names the runner sets matter here; any other spelling is no role.
    return {role for role in MANAGERS if environ.get("AB_ROLE_" + role.upper()) == "1"}


def model_path(value):
    """author/slug[:variant] with no empty, . or .. segment: the only shape that becomes a provider URL path."""
    name, _, variant = value.partition(":")
    parts = name.split("/")
    return (len(parts) == 2 and all(re.fullmatch(r"[A-Za-z0-9._~+@-]+", p) and p not in (".", "..") for p in parts)
            and (not variant and ":" not in value or re.fullmatch(r"[A-Za-z0-9._-]+", variant) is not None))


def method_call(body):
    """(method, argument) for the provider methods beyond ask and decide, or None for a chat request."""
    word = body.strip()
    if word in ("models", "balance", "key"):
        return word, None
    if word == "models --images":
        return "models", "images"
    if body.startswith("image "):
        return "image", {"prompt": body[6:]}
    # One word with a slash is a model lookup; "model railways" stays a prompt.
    if body.startswith("model ") and re.fullmatch(r"\S*/\S*", body[6:].strip()):
        if not model_path(body[6:].strip()):
            raise ProxyError("model takes an author/slug model id")
        return "model", body[6:].strip()
    if body.startswith("embed "):
        text = body[6:]
        return "embed", {"input": json.loads(text) if text.lstrip().startswith("[") else text}
    if not body.lstrip().startswith("{"):
        return None
    try:
        request = json.loads(body)
    except ValueError:
        return None
    name = request.get("method") if isinstance(request, dict) else None
    if name == "models" and set(request) == {"method", "images"} and request["images"] is True:
        return name, "images"
    if name in ("models", "balance", "key"):
        if set(request) != {"method"}:
            raise ProxyError(name + " takes no arguments" if name != "models" else "models takes only images: true")
        return name, None
    if name == "image":
        return name, {k: v for k, v in request.items() if k != "method"}
    if name == "model":
        if set(request) != {"method", "id"} or not isinstance(request["id"], str) or not model_path(request["id"]):
            raise ProxyError("model takes exactly an author/slug model id")
        return name, request["id"]
    if name == "embed":
        if not set(request) <= {"method", "input", "model"} or "input" not in request:
            raise ProxyError("embed takes input and an optional model")
        return name, {k: v for k, v in request.items() if k != "method"}
    return None


def go_json_size(value):
    """Bytes of value as Go's encoding/json writes it: UTF-8 kept, <, > and & as \\u00XX, U+2028/9 escaped."""
    text = json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    return len(text.encode()) + 5 * sum(text.count(c) for c in "<>&") + 3 * (text.count("\u2028") + text.count("\u2029"))


def reply_size(envelope, body):
    """The runner's /send request carrying body back along the message's route."""
    back = envelope.get("reply_to") if isinstance(envelope.get("reply_to"), dict) else None
    to, topic, tag = (back.get("name"), back.get("topic"), back.get("tag")) if back else (envelope.get("from"), envelope.get("topic"), envelope.get("tag"))
    # protocol.Envelope always writes these five plus its zero times, which omitempty does not drop.
    zero = "0001-01-01T00:00:00Z"
    shell = {"message_id": "", "from": "", "to": to or "", "body": body, "at": zero, "expires": zero, "deadline": zero}
    shell.update({k: v for k, v in (("topic", topic), ("tag", tag)) if v})
    return go_json_size(shell)


def max_items(model, dims, envelope):
    """How many vectors of dims numbers fit in one answer envelope, each number at its widest spelling.

    O is the envelope around the empty answer, its escaping included; the numbers, commas and brackets
    a vector adds need no escaping."""
    empty = reply_size(envelope, json.dumps({"model": model, "vectors": []}, ensure_ascii=False, separators=(",", ":")))
    return (ANSWER_LIMIT - empty) // (dims * (FLOAT_WIDTH + 1) + 2)


def known_dims(work):
    try:
        value = json.loads((Path(work) / DIMS_FILE).read_text()) if work else {}
    except (OSError, ValueError):
        return {}
    return value if isinstance(value, dict) else {}


def remember_dims(work, model, dims):
    if not work:
        return
    value = known_dims(work)
    if value.get(model) == dims:
        return
    value.pop(model, None)
    # Newest last; the oldest go first once the file holds DIMS_KEPT models.
    value = dict(list(value.items())[-(DIMS_KEPT - 1):] + [(model, dims)])
    try:
        with tempfile.NamedTemporaryFile("w", dir=work, prefix="." + DIMS_FILE + ".", delete=False) as temporary:
            temporary.write(json.dumps(value))
        os.replace(temporary.name, Path(work) / DIMS_FILE)
    except OSError:
        # A write that failed part way leaves no hidden partial file behind.
        with contextlib.suppress(OSError, NameError):
            os.unlink(temporary.name)


def usd(value):
    return f"{value:.6f}".rstrip("0").rstrip(".") + " USD"


def usage_line(result):
    usage = result.get("usage") if isinstance(result, dict) else None
    if not isinstance(usage, dict):
        return ""
    parts = []
    numbers = {k: usage.get(k) for k in ("prompt_tokens", "completion_tokens", "cost")}
    numbers = {k: v for k, v in numbers.items() if isinstance(v, (int, float)) and not isinstance(v, bool)}
    if "prompt_tokens" in numbers or "completion_tokens" in numbers:
        parts.append(f"tokens {numbers.get('prompt_tokens', '?')} in, {numbers.get('completion_tokens', '?')} out")
    if "cost" in numbers:
        parts.append("cost " + usd(numbers["cost"]))
    return "\n\n[" + "; ".join(parts) + "]" if parts else ""


def exchange(envelope, base, verb, path, payload, key, limit=MAX_BYTES):
    """One provider request under the caller's deadline: (status, raw body)."""
    timer, remaining = arm_deadline(envelope)
    connection = None
    try:
        connection = connect(base, remaining)
        if verb == "POST":
            response = post(connection, path, payload, key)
        else:
            connection.request("GET", path, headers={"Authorization": "Bearer " + key})
            response = connection.getresponse()
        raw = response.read(limit + 1)
        if len(raw) > limit:
            raise ProxyError(f"provider answer exceeds {limit} bytes")
        return response.status, raw
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, timer)
        if connection is not None:
            connection.close()


def provider_call(name, envelope, base, verb, path, payload, key, limit=MAX_BYTES):
    """The provider's scrubbed JSON for one method, or the error answer as a string."""
    status, raw = exchange(envelope, base, verb, path, payload, key, limit)
    if status == 404:
        return status, "error: provider has no " + name
    try:
        result = scrub(json.loads(raw, parse_constant=reject_constant), key)
    except (ValueError, UnicodeError):
        if status >= 300:
            return status, refusal(status, redact(raw.decode("utf-8", errors="replace"), key))
        raise ProxyError("provider did not return JSON") from None
    if status >= 300:
        error = result.get("error", result) if isinstance(result, dict) else result
        message = error.get("message", error) if isinstance(error, dict) else error
        return status, refusal(status, redact(json.dumps(message, ensure_ascii=False) if not isinstance(message, str) else message, key))
    return status, result


def image_request(argument, config):
    """The provider request for one image call, validated against this agent's caps."""
    request = dict(argument)
    model = request.pop("model", config.get("image_model"))
    prompt = request.pop("prompt", None)
    if not isinstance(model, str) or not model_path(model):
        raise ProxyError("set Configuration image_model or provide a request model")
    if not isinstance(prompt, str) or not prompt.strip():
        raise ProxyError("image takes a prompt")
    if set(request) - IMAGE_OPTIONS:
        raise ProxyError("image options are " + ", ".join(sorted(IMAGE_OPTIONS)))
    n = request.get("n", 1)
    if type(n) is not int or not 1 <= n <= IMAGE_N_MAX:
        raise ProxyError(f"n is 1 to {IMAGE_N_MAX} images per call")
    if "output_format" in request and request["output_format"] not in IMAGE_FORMATS:
        raise ProxyError("output_format is one of " + ", ".join(sorted(IMAGE_FORMATS)))
    references = request.get("input_references", [])
    if not isinstance(references, list) or len(references) > IMAGE_REFERENCES_MAX:
        raise ProxyError(f"input_references is a list of at most {IMAGE_REFERENCES_MAX} images")
    wrapped = []
    for item in references:
        url = item if isinstance(item, str) else (item.get("image_url") or {}).get("url") if isinstance(item, dict) else None
        # A reference is a URL or inline base64; a bigfile name is not taken yet.
        if not isinstance(url, str) or not (url.startswith("https://") or re.match(r"data:image/[a-z0-9.+-]+;base64,", url)):
            raise ProxyError("an input reference is an https URL or a data:image/...;base64 URL")
        wrapped.append({"type": "image_url", "image_url": {"url": url}})
    if references:
        request["input_references"] = wrapped
    payload = json.dumps({"model": model, "prompt": prompt, **request}, ensure_ascii=False).encode()
    if len(payload) > MAX_BYTES:
        raise ProxyError("request exceeds 1048576 bytes")
    return payload


def generate_images(payload, envelope, base, key):
    """Generate and store the images. #ai has no bus access of its own yet, so nothing is sent."""
    return IMAGE_UNAVAILABLE, "unavailable"


def provider_method(name, argument, envelope, config, key, roles, work, base):
    """Answer models, model, balance, key, embed or image: (answer, status)."""
    if name in ("balance", "key", "image") and not roles & MANAGERS:
        return f"error: {name} needs the owner or maintainer role", "refused"
    root = base.path.rstrip("/")
    if name == "image":
        return generate_images(image_request(argument, config), envelope, base, key)
    if name == "embed":
        model = argument.get("model", config.get("embed_model"))
        if not isinstance(model, str) or not re.fullmatch(MODEL_ID, model):
            raise ProxyError("set Configuration embed_model or provide a request model")
        texts = argument["input"]
        items = [texts] if isinstance(texts, str) else texts
        if not isinstance(items, list) or not items or not all(isinstance(t, str) and t for t in items):
            raise ProxyError("embed takes a text or a list of texts")
        dims = known_dims(work).get(model)
        if isinstance(dims, int) and dims > 0 and len(items) > max_items(model, dims, envelope):
            return f"error: {len(items)} items × {dims} dims > max {max_items(model, dims, envelope)}", "refused"
        payload = json.dumps({"model": model, "input": texts}, ensure_ascii=False).encode()
        if len(payload) > MAX_BYTES:
            raise ProxyError("request exceeds 1048576 bytes")
        # Provider JSON spends bytes on per-row fields the answer drops, so a full answer's source is larger.
        status, result = provider_call(name, envelope, base, "POST", root + "/embeddings", payload, key, EMBED_READ)
        if isinstance(result, str):
            return result, status
        rows = sorted(result["data"], key=lambda row: row["index"])
        vectors = [row["embedding"] for row in rows]
        if len(vectors) != len(items) or not all(isinstance(v, list) and v and all(isinstance(x, (int, float)) and not isinstance(x, bool) for x in v) for v in vectors):
            raise ProxyError("provider answer has no vectors")
        dims = len(vectors[0])
        remember_dims(work, model, dims)
        if len(items) > max_items(model, dims, envelope):
            return f"error: {len(items)} items × {dims} dims > max {max_items(model, dims, envelope)}", status
        return json.dumps({"model": model, "vectors": vectors}, ensure_ascii=False, separators=(",", ":")), status
    path = {"models": "/images/models" if argument == "images" else "/models", "balance": "/credits", "key": "/key"}.get(name)
    if name == "model":
        path = "/models/" + urllib.parse.quote(argument, safe="/:@+~._-") + "/endpoints"
    status, result = provider_call(name, envelope, base, "GET", root + path, None, key)
    kind = "text"
    if name == "model" and status == 404:
        # Not a chat model: an image model lists its endpoints under the Image API.
        kind = "image"
        status, result = provider_call(name, envelope, base, "GET", root + "/images" + path, None, key)
    if isinstance(result, str):
        return result, status
    data = result["data"]
    if name == "models":
        default = config.get("image_model" if argument == "images" else "model")
        return "\n".join(row["id"] + (" (default)" if row["id"] == default else "") for row in data), status
    if name == "model":
        fields = ("context_length", "max_completion_tokens", "pricing") if kind == "text" else ("pricing", "supported_parameters")
        endpoints = [{"provider": e.get("provider_name"), "name": e.get("name"), **{f: e.get(f) for f in fields}}
                     for e in data["endpoints"]]
        detail = {"id": data["id"], "name": data.get("name"), "kind": kind, "input": (data.get("architecture") or {}).get("input_modalities")}
        if kind == "image":
            detail["parameters"] = data.get("supported_parameters")
        return json.dumps({**detail, "endpoints": endpoints}, ensure_ascii=False), status
    if name == "balance":
        total, used = data["total_credits"], data["total_usage"]
        return f"{usd(total - used)} left ({usd(total)} bought, {usd(used)} used)", status
    # The key's label repeats part of the key itself, so only the named limits come back.
    return json.dumps({field: data[field] for field in KEY_FIELDS if field in data}, ensure_ascii=False), status


def trusted_base(config):
    base = urllib.parse.urlsplit(config.get("base_url", "https://openrouter.ai/api/v1"))
    if base.scheme not in ("https", "http") or not base.hostname or base.username or base.password or base.query or base.fragment:
        raise ProxyError("base_url must be an http(s) URL without credentials, query or fragment")
    return base


def answer(envelope, config, key, roles=frozenset(), work=None):
    if not key:
        return "error: OPENROUTER_API_KEY is not set", 78
    status, model, sent, received = "invalid", "", 0, 0
    connection = None
    timer = None
    try:
        body = envelope["body"]
        if not isinstance(body, str) or len(body.encode()) > MAX_BYTES:
            raise ProxyError("request exceeds 1048576 bytes or is not text")
        if not isinstance(config, dict) or set(config) - {"model", "embed_model", "image_model", "base_url"}:
            raise ProxyError("Configuration accepts only model, embed_model, image_model and base_url")
        call = method_call(body)
        if call is not None:
            base = trusted_base(config)
            output, status = provider_method(*call, envelope, config, key, set(roles), work, base)
            output = redact(output, key)
            if len(output.encode()) > MAX_BYTES:
                raise ProxyError("provider answer exceeds 1048576 bytes")
            return output, 0
        deciding = body.startswith("decide ") and body[7:].lstrip().startswith("{")
        decision_body = body[7:] if deciding else None
        if not deciding:
            if body.startswith("ask "):
                body = body[4:]
            is_json = body.lstrip().startswith("{")
            request = json.loads(body) if is_json else {"messages": [{"role": "user", "content": body}]}
            if not isinstance(request, dict):
                raise ProxyError("expected a chat-completions object")
            if request.get("method") == "decide":
                if set(request) != {"method", "body"} or not isinstance(request["body"], str):
                    raise ProxyError("decide takes exactly a raw JSON body string")
                deciding, decision_body = True, request["body"]
            elif request.get("method") == "ask":
                if set(request) == {"method", "prompt"} and isinstance(request["prompt"], str):
                    request = {"messages": [{"role": "user", "content": request["prompt"]}]}
                    is_json = False
                elif set(request) == {"method", "prompt"} and isinstance(request["prompt"], dict):
                    request = request["prompt"]
                else:
                    raise ProxyError("ask takes prompt text or a chat request object")
        if deciding:
            request = json.loads(decision_body, parse_constant=reject_constant)
            if not isinstance(request, dict):
                raise ProxyError("decide needs a JSON request object")
            model = request.get("model", "")  # logging only; the provider validates the model
            is_json = True
            payload = decision_body.encode()  # validate only: never re-dump a decisions request
        else:
            if "model" not in request:
                request["model"] = config.get("model")
            model = request["model"]
            if not isinstance(model, str) or not model:
                raise ProxyError("set Configuration model or provide a request model")
            payload = json.dumps(request, ensure_ascii=False).encode()
        if request.get("stream"):
            raise ProxyError("streaming is not supported")
        base = trusted_base(config)
        sent = len(payload)
        if sent > MAX_BYTES:
            raise ProxyError("request exceeds 1048576 bytes")
        timer, remaining = arm_deadline(envelope)
        connection = connect(base, remaining)
        path = "/api/alpha/decisions" if deciding else base.path.rstrip("/") + "/chat/completions"
        response = post(connection, path, payload, key)
        status = response.status
        raw = response.read(MAX_BYTES + 1)
        received = len(raw)
        if received > MAX_BYTES:
            raise ProxyError("provider answer exceeds 1048576 bytes")
        try:
            result = scrub(json.loads(raw, **({"parse_constant": reject_constant} if deciding else {})), key)
        except (ValueError, UnicodeError):
            if status >= 300:
                return refusal(status, redact(raw.decode("utf-8", errors="replace"), key)), 0
            raise ProxyError("provider did not return JSON") from None
        if status >= 300:
            error = result.get("error", result) if isinstance(result, dict) else result
            message = error.get("message", error) if isinstance(error, dict) else error
            return refusal(status, redact(message if isinstance(message, str) else json.dumps(message, ensure_ascii=False), key)), 0
        if is_json:
            output = json.dumps(result, ensure_ascii=False, allow_nan=not deciding)
        else:
            try:
                output = result["choices"][0]["message"]["content"]
            except (KeyError, IndexError, TypeError):
                raise ProxyError("provider answer has no assistant text") from None
            if not isinstance(output, str):
                raise ProxyError("provider answer has no assistant text")
            output += usage_line(result)
        if len(output.encode()) > MAX_BYTES:
            raise ProxyError("provider answer exceeds 1048576 bytes")
        return output, 0
    except TimeoutError:
        status = "timeout"
        return "error: provider request timed out", 0
    except ProxyError as error:
        return "error: " + redact(str(error), key), 0
    except (ValueError, TypeError, KeyError, AttributeError, RecursionError):
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
        print(json.dumps({"caller": redact(str(envelope.get("from", "unknown")), key)[:512], "model": redact(model, key) if isinstance(model, str) and re.fullmatch(r"[A-Za-z0-9._/~:@+-]{1,128}", model) else "invalid",
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
        output, code = answer(envelope, config, key, delivered_roles(os.environ), os.environ.get("AGENT_BUS_WORK"))
    except (ValueError, TypeError, RecursionError):
        output, code = "error: invalid message or Configuration", 0
    print(output)
    return code


if __name__ == "__main__":
    sys.exit(main())
