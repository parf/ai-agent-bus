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
        if argv == ["systemd-detect-virt"] and p.returncode == 1 and p.stdout.strip() == "none":
            p.returncode = 0
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


SOURCES = {
    "system": "kernel and host identity from uname -a",
    "network": "interfaces and routes from ip a and ip r",
    "memory": "memory and swap usage from free",
    "disks": "disk inventory from lsblk and filesystem space from df -h",
    "uptime": "uptime and load averages from uptime",
    "load": "load and process counts from /proc/loadavg",
    "os": "distribution identity from /etc/os-release",
    "cpu": "CPU architecture and topology from lscpu",
    "inodes": "filesystem inode usage from df -i",
    "pressure": "CPU, memory and I/O pressure from /proc/pressure",
    "units": "failed systemd units from systemctl --failed",
    "time": "clock and synchronization state from timedatectl",
    "dns": "resolver configuration from /etc/resolv.conf",
    "ports": "listening TCP sockets from ss -tln",
    "users": "logged-in users from who",
    "virt": "virtualization type from systemd-detect-virt; none on bare metal",
    "health": "disk health from smartctl -H for lsblk disk devices (root only)",
    "sensors": "hardware temperatures and other readings from sensors (root only)",
}


def describe():
    return ("server-info: read host facts by section. Sends no unsolicited messages.\n"
            "Text: <section> or all; no arguments. JSON: {\"method\":\"<section>\"} or {\"method\":\"all\"}.\n"
            "Text answers contain command output; JSON maps sections to command, stdout, stderr and status rows.\n"
            "Nonzero command statuses are reported. Health and sensors are available only when this agent runs as root.\n"
            + "\n".join(s + ": " + SOURCES[s] + "." for s in available())
            + "\nall: returns every available section.\nhelp: returns this description (also --help).\n"
            "Examples:\nagent-bus call '#server-info@<host>' network\n"
            "agent-bus call '#server-info@<host>' all\n"
            "agent-bus call '#server-info@<host>' '{\"method\":\"memory\"}'")


def answer(body):
    if body.strip() == "help":
        return describe()
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
        print(describe())
    elif os.environ.get("AGENT_BUS_ASK") == "mcp":
        print('{"tools":[]}')
    elif len(sys.argv) != 2:
        print("error: expected one call body")
    else:
        print(answer(sys.argv[1]))


if __name__ == "__main__":
    main()
