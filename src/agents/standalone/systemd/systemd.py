#!/usr/bin/env python3
"""B.2: fixed systemctl verbs, delivered roles, and optional instance unit scope."""
import json
import os
import re
import subprocess
import sys
import tempfile

TIERS = {"list": 0, "timers": 0, "failed": 0, "status": 0, "start": 1, "restart": 1, "reload": 1,
         "stop": 2, "enable": 2, "disable": 2, "logs": 1}
TYPES = ("service", "timer", "socket", "mount", "path", "target", "slice", "scope", "automount", "swap", "device")
UNIT = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.:-]*(?:@[A-Za-z0-9_.:-]+)?\.(?:service|socket|target|timer|path|slice|scope|mount|automount|swap|device)\Z")


def delivered_tier():
    return 2 if os.environ.get("AB_ROLE_ADMIN") == "1" else 1 if os.environ.get("AB_ROLE_OPERATE") == "1" else 0


def valid_unit(unit):
    return isinstance(unit, str) and len(unit) <= 255 and UNIT.fullmatch(unit) is not None


def configured_units():
    raw = os.environ.get("AGENT_BUS_CONFIG")
    if raw is None:
        return None
    config = json.loads(raw)
    if not isinstance(config, dict):
        raise ValueError("Configuration must be an object")
    if "units" not in config:
        return None
    value = config["units"]
    if not isinstance(value, str):
        raise ValueError(f"invalid units entry: {value!r}")
    names = [name.strip() for name in value.split(",")]
    for name in names:
        if not valid_unit(name):
            raise ValueError(f"invalid units entry: {name!r}")
    return list(dict.fromkeys(names))


def logs_options(value):
    if set(value) - {"method", "unit", "lines", "since"}:
        raise ValueError("unknown logs field")
    unit = value.get("unit")
    if "unit" in value and not valid_unit(unit):
        raise ValueError("invalid unit name")
    lines = value.get("lines", 100)
    if type(lines) is not int or not 1 <= lines <= 1000:
        raise ValueError("lines must be an integer from 1 to 1000")
    age = value.get("since")
    if "since" in value and (not isinstance(age, str) or not re.fullmatch(r"[1-9][0-9]{0,4}[mhd]", age)):
        raise ValueError("since must be a positive age in minutes, hours or days")
    return "logs", unit, {"lines": lines, "since": age}


def call(body):
    if body.lstrip().startswith("{"):
        value = json.loads(body)
        if not isinstance(value, dict) or not isinstance(value.get("method"), str):
            raise ValueError("expected a method")
    else:
        words = body.split()
        if not words:
            raise ValueError("expected a method")
        method, words = words[0], words[1:]
        value = {"method": method}
        if method == "logs":
            if words and words[0] != "since" and not words[0].isdigit():
                value["unit"] = words.pop(0)
            if words and words[0].isdigit():
                value["lines"] = int(words.pop(0))
            if words:
                if len(words) != 2 or words[0] != "since":
                    raise ValueError("use logs [unit] [N] [since age]")
                value["since"] = words[1]
        elif method == "list" and len(words) <= 1:
            if words:
                value["type"] = words[0]
        elif method in ("timers", "failed") and not words:
            pass
        elif len(words) == 1:
            value["unit"] = words[0]
        else:
            raise ValueError("use list [type], timers or <method> <unit>; no extra arguments")
    method = value["method"]
    if method not in TIERS:
        raise ValueError("unknown method")
    if method == "logs":
        return logs_options(value)
    if method == "list":
        if set(value) - {"method", "type"}:
            raise ValueError("list accepts only method and optional type")
        if "type" in value and value["type"] not in TYPES:
            raise ValueError("invalid unit type")
        return method, None, {"type": value.get("type")}
    if method in ("timers", "failed"):
        if set(value) != {"method"}:
            raise ValueError(method + " accepts no arguments")
        return method, None, {}
    if set(value) != {"method", "unit"}:
        raise ValueError("only method and the required unit are accepted; roles and arguments are not")
    unit = value["unit"]
    if not valid_unit(unit):
        raise ValueError("invalid unit name: use one exact fully qualified unit")
    return method, unit, {}


def execute(argv, output_limit=None):
    try:
        if output_limit is not None:
            # File-backed capture keeps oversized journal entries out of memory.
            with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
                result = subprocess.run(argv, stdin=subprocess.DEVNULL, stdout=out, stderr=err,
                                        timeout=30, env={**os.environ, "LC_ALL": "C", "SYSTEMD_COLORS": "0"})
                size = out.tell() + err.tell()
                out.seek(0)
                stdout = out.read(output_limit).decode("utf-8", errors="replace").encode("utf-8")[:output_limit]
                stdout = stdout.decode("utf-8", errors="ignore")
                err.seek(0)
                remaining = output_limit - len(stdout.encode("utf-8"))
                stderr = err.read(remaining).decode("utf-8", errors="replace").encode("utf-8")[:remaining]
                stderr = stderr.decode("utf-8", errors="ignore")
                return {"stdout": stdout, "stderr": stderr,
                        "status": result.returncode, "truncated": size > output_limit}

        result = subprocess.run(argv, stdin=subprocess.DEVNULL, capture_output=True, text=True,
                                errors="replace", timeout=30,
                                env={**os.environ, "LC_ALL": "C", "SYSTEMD_COLORS": "0"})
        return {"stdout": result.stdout, "stderr": result.stderr, "status": result.returncode}
    except FileNotFoundError:
        return {"stdout": "", "stderr": argv[0] + " is not installed", "status": 127}
    except subprocess.TimeoutExpired:
        return {"stdout": "", "stderr": argv[0] + " timed out", "status": 124}
    except OSError as error:
        return {"stdout": "", "stderr": str(error), "status": 126}


def describe():
    tier = delivered_tier()
    units = configured_units()
    coverage = "every unit on this host" if units is None else "only these units: " + ", ".join(units)
    lines = ["systemd: inspect and control " + coverage + ". Sends no unsolicited messages.",
             "Only exact, fully qualified unit names are accepted; caller roles control which actions are available.",
             'Text forms below; JSON: {"method":"list"} or {"method":"<verb>","unit":"<unit>"}.',
             "Text returns command output or ok, and error on failure; JSON returns stdout, stderr, status and method, plus error on failure."]
    for method, required in TIERS.items():
        if required > tier:
            continue
        role = "no role" if required == 0 else "operate" if required == 1 else "admin"
        if method == "list":
            lines.append('list [type]: returns all loaded units, including inactive units (no role); optional type is one of ' + ', '.join(TYPES) + '. JSON: {"method":"list","type":"timer"}; type is optional. Example: list timer')
        elif method == "timers":
            lines.append('timers: no arguments; returns timers with next and last runs (no role). JSON: {"method":"timers"}. Example: timers')
        elif method == "failed":
            lines.append('failed: no arguments; returns failed units with unit, load, active, sub and description columns (no role). JSON: {"method":"failed"}; answer adds a units array of those fields. Example: failed')
        elif method == "status":
            lines.append("status <unit>: returns systemctl status for the unit (no role). Example: status nginx.service")
        elif method == "logs":
            lines.append('logs [<unit>] [N] [since <age>]: returns journal entries for one unit, or the whole journal when omitted (operate). N is 1..1000, default 100; age is 1..99999 followed by m, h or d; omitted means no date filter. Output is capped at 256 KiB with truncation noted. Logs may contain secrets. JSON: {"method":"logs","unit":"nginx.service","lines":200,"since":"1h"}; optional keys are unit, lines, since. Example: logs nginx.service 200 since 1h')
        else:
            lines.append(f"{method} <unit>: requests systemctl {method}; returns ok or command output on success ({role}). Example: {method} nginx.service")
    lines.append("help: returns this description (also --help). Examples above are call bodies: agent-bus call '#systemd@<host>' '<body>'.")
    description = "\n".join(lines)
    if units is not None:
        description = description.replace("all loaded units", "listed loaded units").replace("the whole journal when omitted", "only the listed units’ journals when omitted")
    return description


def answer(body):
    if body.strip() == "help":
        return describe()
    is_json = body.lstrip().startswith("{")
    try:
        units = configured_units()
        method, unit, options = call(body)
        if units is not None and unit is not None and unit not in units:
            raise ValueError(f"unit {unit} is outside this instance’s configured units")
        # Application roles are delivered by the bus, never inferred from the
        # caller, OWNER/MANAGER roles, or fields supplied in the body.
        tier = delivered_tier()
        if tier < TIERS[method]:
            raise ValueError("requires " + ("admin" if TIERS[method] == 2 else "operate") + " role")
        if method == "logs":
            argv = ["journalctl", "--no-pager", "-o", "short-iso", "-n", str(options["lines"])]
            if options["since"]:
                age = options["since"]
                word = {"m": "minute", "h": "hour", "d": "day"}[age[-1]]
                argv += ["--since", age[:-1] + " " + word + " ago"]
            for journal_unit in ([unit] if unit is not None else units or []):
                argv += ["-u", journal_unit]
            result = {"method": method, **execute(argv, output_limit=256 * 1024)}
        else:
            argv = ["systemctl", "--no-pager", "--no-ask-password"]
            if method == "list":
                argv += ["list-units", "--all", "--plain", "--no-legend"]
                if options["type"]:
                    argv += ["--type=" + options["type"]]
            elif method == "failed":
                argv += ["list-units", "--failed", "--all", "--plain", "--no-legend"]
            elif method == "timers":
                argv += ["list-timers", "--all"]
            else:
                argv += [method, "--", unit]
            if units is not None and method in ("list", "timers", "failed"):
                argv += ["--", *units]
            result = {"method": method, **execute(argv)}
        if method == "failed" and result["status"] == 0:
            result["units"] = [dict(zip(("unit", "load", "active", "sub", "description"), fields))
                               for line in result["stdout"].splitlines()
                               if len(fields := line.split(None, 4)) == 5]
        if unit is not None:
            result["unit"] = unit
        elif method == "list" and options["type"]:
            result["type"] = options["type"]
        if result["status"] != 0:
            result["error"] = ("journalctl" if method == "logs" else "systemctl") + " refused or failed the request"
    except (ValueError, OSError) as error:
        result = {"error": str(error)}
    if is_json:
        return json.dumps(result)
    if "error" in result:
        return "error: " + result["error"] + (f" (exit {result['status']})" if "status" in result else "") + ("\n" + result["stderr"].rstrip() if result.get("stderr") else "") + ("\n[output truncated]" if result.get("truncated") else "")
    if method == "failed" and not result["units"]:
        return "no failed units"
    return (result["stdout"].rstrip() or "ok") + ("\n[output truncated]" if result.get("truncated") else "")


def main():
    try:
        configured_units()
    except ValueError as error:
        print("error: Configuration units: " + str(error))
        return
    if os.environ.get("AGENT_BUS_ASK") == "help":
        print(describe())
    elif os.environ.get("AGENT_BUS_ASK") == "mcp":
        print('{"tools":[]}')
    elif len(sys.argv) != 2:
        print("error: expected one call body")
    else:
        print(answer(sys.argv[1]))


if __name__ == "__main__":
    main()
