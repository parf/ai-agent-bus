#!/usr/bin/env python3
"""B.2: fixed systemctl verbs, delivered roles, and a declared unit set."""
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tempfile

TIERS = {"list": 0, "status": 0, "start": 1, "restart": 1, "reload": 1,
         "stop": 2, "enable": 2, "disable": 2, "logs": 1}
UNIT = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.:-]*(?:@[A-Za-z0-9_.:-]+)?\.(?:service|socket|target|timer|path|slice|scope|mount|automount|swap|device)\Z")


def delivered_tier():
    return 2 if os.environ.get("AB_ROLE_ADMIN") == "1" else 1 if os.environ.get("AB_ROLE_OPERATE") == "1" else 0


def valid_unit(unit):
    return isinstance(unit, str) and len(unit) <= 255 and UNIT.fullmatch(unit) is not None


def configuration(path):
    with Path(path).open() as source:
        info = os.fstat(source.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
            raise ValueError("unit-set file must be a root-owned regular file, not group/world writable")
        value = json.load(source)
    if not isinstance(value, dict) or set(value) != {"units"} or not isinstance(value["units"], list):
        raise ValueError('unit-set file must contain only {"units":[...]}')
    units = value["units"]
    if not units or not all(valid_unit(u) for u in units) or len(set(units)) != len(units):
        raise ValueError("unit set must contain distinct, exact, fully qualified unit names")
    return units


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
        method = value["method"]
        if method == "logs":
            return logs_options(value)
        if set(value) != ({"method"} if method == "list" else {"method", "unit"}):
            raise ValueError("only method and the required unit are accepted; roles and arguments are not")
        unit = value.get("unit")
    else:
        words = body.split()
        if words and words[0] == "logs":
            value, words = {"method": "logs"}, words[1:]
            if words and words[0] != "since" and not words[0].isdigit():
                value["unit"] = words.pop(0)
            if words and words[0].isdigit():
                value["lines"] = int(words.pop(0))
            if words:
                if len(words) != 2 or words[0] != "since":
                    raise ValueError("use logs [unit] [N] [since age]")
                value["since"] = words[1]
            return logs_options(value)
        if len(words) == 1 and words[0] == "list":
            method, unit = "list", None
        elif len(words) == 2:
            method, unit = words
        else:
            raise ValueError("use list or <method> <unit>; no extra arguments")
    if method not in TIERS:
        raise ValueError("unknown method")
    if method != "list" and not valid_unit(unit):
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


def answer(body, units_file):
    is_json = body.lstrip().startswith("{")
    try:
        method, unit, options = call(body)
        units = configuration(units_file)
        # Application roles are delivered by the bus, never inferred from the
        # caller, OWNER/MANAGER roles, or fields supplied in the body.
        tier = delivered_tier()
        if tier < TIERS[method]:
            raise ValueError("requires " + ("admin" if TIERS[method] == 2 else "operate") + " role")
        if unit is not None and unit not in units:
            raise ValueError("unit is outside this instance's declared set")
        if method == "logs":
            argv = ["journalctl", "--no-pager", "-o", "short-iso", "-n", str(options["lines"])]
            if options["since"]:
                age = options["since"]
                word = {"m": "minute", "h": "hour", "d": "day"}[age[-1]]
                argv += ["--since", age[:-1] + " " + word + " ago"]
            for selected in [unit] if unit else units:
                argv += ["-u", selected]
            result = {"method": method, **execute(argv, output_limit=256 * 1024)}
        else:
            argv = ["systemctl", "--no-pager", "--no-ask-password"]
            argv += ["list-units", "--all", "--plain", "--no-legend", "--", *units] if method == "list" else [method, "--", unit]
            result = {"method": method, **execute(argv)}
        if unit is not None:
            result["unit"] = unit
        else:
            result["units"] = units
        if result["status"] != 0:
            result["error"] = ("journalctl" if method == "logs" else "systemctl") + " refused or failed the request"
    except (ValueError, OSError) as error:
        result = {"error": str(error)}
    if is_json:
        return json.dumps(result)
    if "error" in result:
        return "error: " + result["error"] + (f" (exit {result['status']})" if "status" in result else "") + ("\n" + result["stderr"].rstrip() if result.get("stderr") else "") + ("\n[output truncated]" if result.get("truncated") else "")
    return (result["stdout"].rstrip() or "ok") + ("\n[output truncated]" if result.get("truncated") else "")


def main():
    if os.environ.get("AGENT_BUS_ASK") == "help":
        tier = delivered_tier()
        print("systemd methods: " + ", ".join(method for method, required in TIERS.items() if required <= tier))
        print('Text: list or <method> <unit>; JSON: {"method":..., "unit":...}. Declared units only.')
        if tier >= 1:
            print('Logs: logs [unit] [N] [since age]; JSON logs accepts unit, lines, since.')
    elif os.environ.get("AGENT_BUS_ASK") == "mcp":
        print('{"tools":[]}')
    elif len(sys.argv) != 4 or sys.argv[1] != "--units-file":
        print("error: installation requires --units-file and one call body")
    else:
        print(answer(sys.argv[3], sys.argv[2]))


if __name__ == "__main__":
    main()
