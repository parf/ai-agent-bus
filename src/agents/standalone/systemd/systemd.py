#!/usr/bin/env python3
"""B.2: fixed systemctl verbs, delivered roles, and a declared unit set."""
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys

TIERS = {"list": 0, "status": 0, "start": 1, "restart": 1, "reload": 1,
         "stop": 2, "enable": 2, "disable": 2}
UNIT = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.:-]*(?:@[A-Za-z0-9_.:-]+)?\.(?:service|socket|target|timer|path|slice|scope|mount|automount|swap|device)\Z")


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


def call(body):
    if body.lstrip().startswith("{"):
        value = json.loads(body)
        if not isinstance(value, dict) or not isinstance(value.get("method"), str):
            raise ValueError("expected a method")
        method = value["method"]
        if set(value) != ({"method"} if method == "list" else {"method", "unit"}):
            raise ValueError("only method and the required unit are accepted; roles and arguments are not")
        unit = value.get("unit")
    else:
        words = body.split()
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
    return method, unit


def execute(argv):
    try:
        result = subprocess.run(argv, stdin=subprocess.DEVNULL, capture_output=True, text=True,
                                errors="replace", timeout=30,
                                env={**os.environ, "LC_ALL": "C", "SYSTEMD_COLORS": "0"})
        return {"stdout": result.stdout, "stderr": result.stderr, "status": result.returncode}
    except FileNotFoundError:
        return {"stdout": "", "stderr": "systemctl is not installed", "status": 127}
    except subprocess.TimeoutExpired:
        return {"stdout": "", "stderr": "systemctl timed out", "status": 124}
    except OSError as error:
        return {"stdout": "", "stderr": str(error), "status": 126}


def answer(body, units_file):
    is_json = body.lstrip().startswith("{")
    try:
        method, unit = call(body)
        units = configuration(units_file)
        # Application roles are delivered by the bus, never inferred from the
        # caller, OWNER/MANAGER roles, or fields supplied in the body.
        tier = 2 if os.environ.get("AB_ROLE_ADMIN") == "1" else 1 if os.environ.get("AB_ROLE_OPERATE") == "1" else 0
        if tier < TIERS[method]:
            raise ValueError("requires " + ("admin" if TIERS[method] == 2 else "operate") + " role")
        if unit is not None and unit not in units:
            raise ValueError("unit is outside this instance's declared set")
        argv = ["systemctl", "--no-pager", "--no-ask-password"]
        argv += ["list-units", "--all", "--plain", "--no-legend", "--", *units] if method == "list" else [method, "--", unit]
        result = {"method": method, **execute(argv)}
        if unit is not None:
            result["unit"] = unit
        else:
            result["units"] = units
        if result["status"] != 0:
            result["error"] = "systemctl refused or failed the request"
    except (ValueError, OSError) as error:
        result = {"error": str(error)}
    if is_json:
        return json.dumps(result)
    if "error" in result:
        return "error: " + result["error"] + (f" (exit {result['status']})" if "status" in result else "") + ("\n" + result["stderr"].rstrip() if result.get("stderr") else "")
    return result["stdout"].rstrip() or "ok"


def main():
    if os.environ.get("AGENT_BUS_ASK") == "help":
        print('systemd B.2: list or <method> <unit>; JSON {"method":"list"} or {"method":"status","unit":"name.service"}. '
              'Read: list,status; operate: start,restart,reload; admin: stop,enable,disable. Declared units only.')
    elif os.environ.get("AGENT_BUS_ASK") == "mcp":
        print('{"tools":[]}')
    elif len(sys.argv) != 4 or sys.argv[1] != "--units-file":
        print("error: installation requires --units-file and one call body")
    else:
        print(answer(sys.argv[3], sys.argv[2]))


if __name__ == "__main__":
    main()
