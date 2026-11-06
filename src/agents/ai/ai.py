#!/usr/bin/env python3
"""Kept processes that serve #ai's own inbox: one message at a time, one OpenAI-compatible request each."""
import base64
import binascii
import contextlib
import http.client
import json
import math
import os
import re
import secrets
import signal
import sys
import tempfile
import urllib.parse

from collections.abc import Mapping
from pathlib import Path
HERE = Path(__file__).resolve().parent
SUPPORT = HERE / 'support' if (HERE / 'support').exists() else HERE.parents[1]
sys.path.insert(0, str(SUPPORT / 'clients/python'))
sys.path.insert(0, str(SUPPORT / 'agents/shared'))
from metadata import dispatch
from agent_bus import Bus
from agent_bus.errors import BusError, Refusal

from provider_http import MAX_BYTES, ProxyError, redact, scrub, provider_error, arm_deadline, connect, post

MODEL_ID = r"[A-Za-z0-9._/~:@+-]{1,128}"
# Each answer goes to /send in a JSON request the daemon reads up to 1 MiB (Server.read, src/internal/api/server.go):
# the request, not the body, is bounded. The cap below sizes the runner's answer envelope (answerAs,
# src/cmd/agent-bus-runner/serve.go); the Python client's reply request #ai now sends carries no zero times and
# escapes no <, > or &, so it is never larger, and the cap stays a safe bound.
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
# Generated images go to #ai's own bigfile record, under the daemon's per-record limits (docs/03-records-kv.md#bigfiles).
IMAGE_MAX = 32 * 1024 * 1024
RECORD_KEYS = 100
RECORD_BYTES = 256 * 1024 * 1024
IMAGE_TTL = 86400
IMAGE_TYPES = {"image/png": "png", "image/jpeg": "jpeg", "image/webp": "webp", "image/gif": "gif", "image/svg+xml": "svg"}
# The provider answer carries every image in base64, a third larger than its bytes. It is held about three times
# over while it is parsed and decoded, so the read stays well inside the runner's default 512M memory limit.
IMAGE_READ = 96 * 1024 * 1024


def refusal(status, message):
    """A provider error answer; a refused key or credit is named, since no retry helps."""
    if status in (401, 403, 402):
        named = "the configured key" if status != 402 else "for lack of credit"
        output = f"error: the provider refused {named} ({status}: {message})"
        if len(output.encode()) > MAX_BYTES:
            output = f"error: the provider refused {named} ({status}: provider error exceeds 1048576 bytes)"
        return output
    return provider_error(status, message)


def reject_constant(_value):
    raise ValueError("non-standard JSON constant")


def message_roles(message, name):
    """The caller's roles on #ai, as the daemon set them on a message sent to #ai itself; the body cannot add one."""
    envelope = message.envelope
    original = envelope.get("original_to")
    if original not in (None, "") and (not isinstance(original, str) or original.lower() != name.lower()):
        return set()
    roles = envelope.get("roles", ())
    return {role for role in roles if role in MANAGERS} if isinstance(roles, (list, tuple)) else set()


def model_path(value):
    """author/slug[:variant] with no empty, . or .. segment: the only shape that becomes a provider URL path."""
    name, _, variant = value.partition(":")
    parts = name.split("/")
    return (len(parts) == 2 and all(re.fullmatch(r"[A-Za-z0-9._~+@-]+", p) and p not in (".", "..") for p in parts)
            and (not variant and ":" not in value or re.fullmatch(r"[A-Za-z0-9._-]+", variant) is not None))


FILTER_WORD = r"[A-Za-z0-9._:/~+-]{1,64}"


def models_options(words):
    """models [--images] [--json] [<word>], or None: anything else after models is a prompt, not the method."""
    options = {"images": False, "json": False, "filter": None}
    for word in words:
        flag = {"--images": "images", "--json": "json"}.get(word)
        if flag and not options[flag]:
            options[flag] = True
        elif not word.startswith("-") and options["filter"] is None and re.fullmatch(FILTER_WORD, word):
            options["filter"] = word
        else:
            return None
    return options


def price(value):
    """A provider price as a finite float, or None: NaN and infinities are no price."""
    try:
        number = float(value)
    except (TypeError, ValueError):
        return None
    return number if math.isfinite(number) else None


def per_million(value):
    """A per-token price string as USD per million tokens; a negative price means the router decides."""
    number = price(value)
    if number is None:
        return "-"
    if number < 0:
        return "varies"
    return f"{number * 1e6:.4f}".rstrip("0").rstrip(".")


def image_price(row):
    """The cheapest price within one unit, per image if the listing gives one, else per megapixel, or -."""
    for unit in ("image", "megapixel"):
        costs = [price(p.get("cost_usd")) for p in row.get("pricing") or [] if isinstance(p, dict)
                 and p.get("unit") == unit and not isinstance(p.get("cost_usd"), bool)]
        costs = [cost for cost in costs if cost is not None]
        if costs:
            return f"{min(costs):g}/{unit}"
    return "-"


def is_free(row):
    """A :free variant, or a model whose prompt and completion prices are both exactly zero."""
    pricing = row.get("pricing") or {}
    return row["id"].endswith(":free") or (isinstance(pricing, dict) and price(pricing.get("prompt")) == 0
                                           and price(pricing.get("completion")) == 0)


def cell(value):
    """A table cell on one line: control characters from the provider are shown escaped."""
    return re.sub(r"[\x00-\x1f\x7f]", lambda m: f"\\x{ord(m.group()):02x}", str(value))


def default_row(rows, default):
    """The listed row the configured default names: its id, canonical slug, or its base id under an unlisted variant."""
    if not isinstance(default, str) or not default:
        return None
    for want in (default, default.partition(":")[0]):
        for row in rows:
            if want in (row["id"], row.get("canonical_slug")):
                return row
    return None


def models_answer(rows, options, default):
    """A compact table of the listing, filtered; --json passes the provider's full records through."""
    marked = default_row(rows, default)
    word = (options["filter"] or "").lower()
    if word == "free":
        rows = [row for row in rows if (image_price(row).startswith("0/") if options["images"] else is_free(row))]
    elif word:
        rows = [row for row in rows if word in row["id"].lower() or word in str(row.get("name", "")).lower()]
    if options["json"]:
        # Compact and unescaped, the records never outgrow the provider answer, which is read up to 1 MiB.
        return json.dumps(rows, ensure_ascii=False, separators=(",", ":"))
    if options["images"]:
        table = [("id", "price USD", "input")]
        table += [(cell(row["id"]), image_price(row), cell(",".join((row.get("architecture") or {}).get("input_modalities") or []) or "-")) for row in rows]
    else:
        table = [("id", "context", "$/M in", "$/M out", "input")]
        for row in rows:
            pricing = row.get("pricing") if isinstance(row.get("pricing"), dict) else {}
            table.append((cell(row["id"]), cell(row.get("context_length") or "-"), per_million(pricing.get("prompt")),
                          per_million(pricing.get("completion")), cell(",".join((row.get("architecture") or {}).get("input_modalities") or []) or "-")))
    widths = [max(len(line[i]) for line in table) for i in range(len(table[0]))]
    lines = [("* " if marked is not None and line[0] == cell(marked["id"]) and n else "  ")
             + "  ".join(cell.ljust(width) for cell, width in zip(line, widths)).rstrip() for n, line in enumerate(table)]
    if not isinstance(default, str) or not default:
        lines.append("default: unset")
    else:
        lines.append("default: " + cell(default) + ("" if marked is not None else " (not listed)"))
    return "\n".join(lines)


def method_call(body):
    """(method, argument) for the provider methods beyond ask and decide, or None for a chat request."""
    word = body.strip()
    if word in ("balance", "key"):
        return word, None
    options = models_options(word.split()[1:]) if word.split(None, 1)[:1] == ["models"] else None
    if options is not None:
        return "models", options
    # Like decide, only a JSON object after "image " is the method: "image recognition is…" stays a prompt.
    if body.startswith("image ") and body[6:].lstrip().startswith("{"):
        request = json.loads(body[6:])
        if not isinstance(request, dict) or "method" in request:
            raise ProxyError("image takes a JSON request object")
        return "image", request
    # One word with a slash is a model lookup; "model railways" stays a prompt.
    if body.startswith("model ") and re.fullmatch(r"\S*/\S*", body[6:].strip()):
        if not model_path(body[6:].strip()):
            raise ProxyError("model takes an author/slug model id")
        return "model", body[6:].strip()
    # Only JSON after "embed " is the method: a "text", a [list] or an {"input":…} object.
    # "embed this video in my page" stays a prompt.
    if body.startswith("embed ") and body[6:].lstrip()[:1] in ('"', "[", "{"):
        value = json.loads(body[6:])
        if isinstance(value, dict):
            if not set(value) <= {"input", "model"} or "input" not in value:
                raise ProxyError("embed takes input and an optional model")
            return "embed", value
        return "embed", {"input": value}
    if not body.lstrip().startswith("{"):
        return None
    try:
        request = json.loads(body)
    except ValueError:
        return None
    name = request.get("method") if isinstance(request, dict) else None
    if name == "models":
        if not set(request) <= {"method", "images", "json", "filter"} or any(
                type(request.get(flag, False)) is not bool for flag in ("images", "json")):
            raise ProxyError("models takes images and json as booleans and an optional filter word")
        filter_word = request.get("filter")
        if filter_word is not None and not (isinstance(filter_word, str) and re.fullmatch(FILTER_WORD, filter_word)):
            raise ProxyError("models filter is one word")
        return name, {"images": request.get("images", False), "json": request.get("json", False), "filter": filter_word}
    if name in ("balance", "key"):
        if set(request) != {"method"}:
            raise ProxyError(name + " takes no arguments")
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
    """The runner's /send request carrying body back along the message's route: an upper bound on #ai's own reply."""
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
    if model is None:
        # Never a guessed model: a paid call goes only to a model somebody chose.
        raise ProxyError("no image model configured: set Configuration image_model or give a request model")
    if not isinstance(model, str) or not model_path(model):
        raise ProxyError("image model is an author/slug model id")
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
    return payload, n


def record_full(store, count, size):
    """The refusal when count more keys of size bytes would not fit #ai's bigfile record, else None."""
    rows = store.bigfile.list()
    keys, used = len(rows), sum(row["size"] for row in rows)
    if keys + count > RECORD_KEYS or used + size > RECORD_BYTES:
        return full_answer(store, keys, used)
    return None


def full_answer(store, keys=None, used=None):
    held = f"{keys} keys, {used} bytes of " if keys is not None else ""
    return f"error: {store.name} bigfile record is full ({held}{RECORD_KEYS} keys / 256 MiB); try again after old images expire (24 h)"


def sniffed(data):
    """The media type the bytes themselves show, for an item the provider did not label."""
    for magic, media in ((b"\x89PNG\r\n\x1a\n", "image/png"), (b"\xff\xd8\xff", "image/jpeg"), (b"GIF8", "image/gif")):
        if data.startswith(magic):
            return media
    if data[:4] == b"RIFF" and data[8:12] == b"WEBP":
        return "image/webp"
    if re.match(rb"\s*(<\?xml[^>]*>\s*)?<svg[\s>]", data[:512]):
        return "image/svg+xml"
    return None


def decoded_images(result):
    """(media type, bytes) for each image in a provider answer; anything that is not an image is an error."""
    items = result.get("data") if isinstance(result, dict) else None
    if not isinstance(items, list) or not items or len(items) > IMAGE_N_MAX:
        raise ProxyError("provider returned no image")
    images = []
    for item in items:
        text = item.get("b64_json") if isinstance(item, dict) else None
        url = item.get("url") if isinstance(item, dict) else None
        if not isinstance(text, str) and isinstance(url, str) and re.match(r"data:image/[a-z0-9.+-]+;base64,", url):
            text = url.partition(",")[2]
        if not isinstance(text, str) or not text:
            raise ProxyError("provider returned no image")
        try:
            data = base64.b64decode(text, validate=True)
        except (binascii.Error, ValueError):
            raise ProxyError("provider returned no image") from None
        media = item.get("media_type")
        media = media if media in IMAGE_TYPES else sniffed(data) if media is None else None
        if media is None or not data:
            raise ProxyError("provider returned no image")
        images.append((media, data))
    return images


def store_refusal(store, error):
    if isinstance(error, Refusal):
        return f"error: {store.name} cannot store images ({error.status}: {error})"
    return f"error: {store.name} cannot store images (the bus did not answer)"


def generate_images(request, envelope, base, key, store):
    """Generate the images and put each into #ai's own bigfile record; the answer names every one."""
    payload, n = request
    if store is None:
        return "error: image needs the kept #ai process, which holds its bigfile record", "refused"
    try:
        # Checked before the paid call too, with room for n images at the largest size a key holds.
        refused = record_full(store, n, n * IMAGE_MAX)
    except BusError as error:
        return store_refusal(store, error), "refused"
    if refused:
        return refused, "refused"
    status, result = provider_call("image", envelope, base, "POST", base.path.rstrip("/") + "/images", payload, key, IMAGE_READ)
    if isinstance(result, str):
        return result, status
    paid = usage_line(result)
    try:
        images = decoded_images(result)
    except ProxyError as error:
        return "error: " + str(error) + paid, status
    lines, kept = [], [(media, data) for media, data in images if len(data) <= IMAGE_MAX]
    try:
        refused = record_full(store, len(kept), sum(len(data) for _, data in kept))
        if refused:
            return refused + paid, status
        for number, (media, data) in enumerate(images, 1):
            if len(data) > IMAGE_MAX:
                lines.append(f"error: image {number} is {len(data)} bytes, over the 32 MiB a bigfile holds; not kept")
                continue
            name = "image-" + secrets.token_hex(8) + "." + IMAGE_TYPES[media]
            store.bigfile.put(name, data, ttl=IMAGE_TTL)
            lines.append(f"{store.name}/{name} {media} {len(data)} bytes, kept 24 h")
    except Refusal as error:
        # Another process may fill the record between the check and the put.
        lines.append(full_answer(store) if error.status == 413 else store_refusal(store, error))
    except BusError as error:
        lines.append(store_refusal(store, error))
    return "\n".join(lines) + paid, status


def provider_method(name, argument, envelope, config, key, roles, work, base, store=None):
    """Answer models, model, balance, key, embed or image: (answer, status)."""
    if name in ("balance", "key", "image") and not roles & MANAGERS:
        return f"error: {name} needs the owner or maintainer role", "refused"
    root = base.path.rstrip("/")
    if name == "image":
        return generate_images(image_request(argument, config), envelope, base, key, store)
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
    path = {"models": "/images/models" if name == "models" and argument["images"] else "/models", "balance": "/credits", "key": "/key"}.get(name)
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
        return models_answer(data, argument, config.get("image_model" if argument["images"] else "model")), status
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


def answer(envelope, config, key, roles=frozenset(), work=None, store=None):
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
            output, status = provider_method(*call, envelope, config, key, set(roles), work, base, store)
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


def thaw(value):
    """A received envelope as the plain dicts and lists answer() reads."""
    if isinstance(value, Mapping):
        return {k: thaw(v) for k, v in value.items()}
    if isinstance(value, tuple):
        return [thaw(v) for v in value]
    return value


def handle(message, config, key, store, work):
    """One message's answer text: a help ask, or the same answer a per-message run gave."""
    roles = message_roles(message, store.name)
    asked = dispatch(HERE, message.ask, roles=roles)
    if asked is not None:
        return asked
    try:
        return answer(thaw(message.envelope), config, key, roles, work, store)[0]
    except (ValueError, TypeError, RecursionError):
        return "error: invalid message or Configuration"


def serve(bus, config, key, work):
    """Read #ai's inbox in the main thread, so SIGALRM deadlines keep working, until SIGTERM or SIGHUP."""
    stopping = []
    # SIGHUP means the Configuration changed: finish, exit, and the runner starts a process with the new one.
    for number in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
        signal.signal(number, lambda *_: stopping.append(True))
    store = bus.record(bus.name)
    while not stopping:
        message = bus.consume(wait=5, share=True)
        if message is None or message.receipt or message.re:
            continue
        reply = handle(message, config, key, store, work)
        try:
            message.reply(reply)
        except BusError:
            print("ai: answer delivery failed", file=sys.stderr)
    return 0


def main():
    result = dispatch(HERE, os.environ.get('AGENT_BUS_ASK'))
    if result is not None:
        print(result)
        return 0
    key = os.environ.get("OPENROUTER_API_KEY", "")
    if not key:
        # A self-serving agent's env.dist is not enforced by the runner, so the process says it.
        print("OPENROUTER_API_KEY is not set", file=sys.stderr)
        return 78
    try:
        config = json.loads(os.environ.get("AGENT_BUS_CONFIG", "{}"))
    except ValueError:
        print("ai: Configuration is not JSON", file=sys.stderr)
        return 78
    try:
        with Bus(timeout=5) as bus:
            return serve(bus, config, key, os.environ.get("AGENT_BUS_WORK"))
    except BusError:
        print("ai: bus unavailable", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
