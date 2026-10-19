#!/usr/bin/env python3
"""Fixed host-fact commands; the dedicated runner owns the bus connection."""
import json
import os
import subprocess
import sys

COMMANDS = {
    "system": [["uname", "-a"]],
    "network": [["ip", "a"], ["ip", "r"]],
    "memory": [["free"]],
    "disks": [["lsblk", "-o", "NAME,TYPE,SIZE,MODEL,SERIAL,MOUNTPOINTS"], ["df", "-h"]],
    "uptime": [["uptime"]],
    "load": [["cat", "/proc/loadavg"]],
    "os": [["cat", "/etc/os-release"]],
    "cpu": [["lscpu"]],
    "inodes": [["df", "-i"]],
    "pressure": [["cat", "/proc/pressure/cpu", "/proc/pressure/memory", "/proc/pressure/io"]],
    "units": [["systemctl", "--failed", "--no-pager"]],
    "time": [["timedatectl", "--no-pager"]],
    "dns": [["cat", "/etc/resolv.conf"]],
    "ports": [["ss", "-tln"]],
    "users": [["who"]],
    "virt": [["systemd-detect-virt"]],
    "health": [],
    "sensors": [["sensors"]],
}


def available():
    return [s for s in COMMANDS if os.geteuid() == 0 or s not in ("health", "sensors")]


def execute(argv):
    try:
        p = subprocess.run(argv, capture_output=True, text=True, errors="replace",
                           timeout=10, env={**os.environ, "LC_ALL": "C"})
        return {"command": argv, "stdout": p.stdout, "stderr": p.stderr, "status": p.returncode}
    except FileNotFoundError:
        return {"command": argv, "stdout": "", "stderr": "command not installed", "status": 127}
    except subprocess.TimeoutExpired:
        return {"command": argv, "stdout": "", "stderr": "command timed out", "status": 124}
    except OSError as e:
        return {"command": argv, "stdout": "", "stderr": str(e), "status": 126}


def collect(section):
    if section != "health":
        return [execute(argv) for argv in COMMANDS[section]]
    # Device paths come only from the host's disk inventory, never the caller.
    inventory = execute(["lsblk", "--nodeps", "--noheadings", "--paths", "--output", "NAME,TYPE"])
    if inventory["status"]:
        return [inventory]
    disks = [row[0] for line in inventory["stdout"].splitlines()
             if len(row := line.split()) == 2 and row[1] == "disk" and row[0].startswith("/dev/")]
    return [execute(["smartctl", "-H", disk]) for disk in disks]


def answer(body):
    is_json = body.lstrip().startswith("{")
    try:
        if is_json:
            call = json.loads(body)
            if not isinstance(call, dict) or set(call) != {"method"} or not isinstance(call["method"], str):
                raise ValueError()
            method = call["method"]
        else:
            method = body.strip()
        if method not in COMMANDS and method != "all":
            raise ValueError()
    except (ValueError, TypeError):
        why = "use one section name or all; arguments are not accepted"
        return json.dumps({"error": why}) if is_json else "error: " + why
    sections = available() if method == "all" else ([method] if method in available() else [])
    result = {s: collect(s) for s in sections}
    if is_json:
        return json.dumps(result)
    return "\n\n".join("## " + s + "\n" + "\n".join(
        "$ " + " ".join(r["command"]) + "\n" + r["stdout"].rstrip()
        + ("\n" + r["stderr"].rstrip() if r["stderr"] else "")
        + (f"\n[exit {r['status']}]" if r["status"] else "") for r in rows)
        for s, rows in result.items())


def main():
    if os.environ.get("AGENT_BUS_ASK") == "help":
        print("server-info: fixed host facts. Send one section name or all, as text or "
              '{"method":"section"}. No arguments. Available: ' + ", ".join(available()))
    elif os.environ.get("AGENT_BUS_ASK") == "mcp":
        print('{"tools":[]}')
    elif len(sys.argv) != 2:
        print("error: expected one call body")
    else:
        print(answer(sys.argv[1]))


if __name__ == "__main__":
    main()
