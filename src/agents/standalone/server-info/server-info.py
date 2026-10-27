#!/usr/bin/env python3
"""Fixed host-fact commands; the dedicated runner owns the bus connection."""
import json
import os
import re
from datetime import datetime, timezone
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'shared'))
from metadata import answer as metadata_answer, ASKS, dispatch
PACKAGE = Path(__file__).resolve().parent

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
    "tls": [["openssl", "x509", "-in", "/etc/agent-bus/tls/cert.pem", "-noout", "-subject", "-nameopt", "RFC2253", "-ext", "subjectAltName", "-enddate", "-fingerprint", "-sha256"], ["agent-bus", "token", "--fingerprint"]],
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


def collect_tls():
    cert, token = [execute(argv) for argv in COMMANDS["tls"]]
    if cert["status"] == 127:
        cert.update(status=0, stdout="unavailable: openssl not installed\n", stderr="")
        return [cert]
    if cert["status"] or token["status"]:
        return [cert, token]
    try:
        lines = cert["stdout"].splitlines()
        fields = dict(line.split("=", 1) for line in lines if "=" in line)
        pin = "sha256:" + fields["sha256 Fingerprint"].replace(":", "").lower()
        token_pin = token["stdout"].strip()
        if not all(re.fullmatch(r"sha256:[0-9a-f]{64}", value) for value in (pin, token_pin)):
            raise ValueError("invalid certificate fingerprint")
        expires = datetime.strptime(fields["notAfter"], "%b %d %H:%M:%S %Y %Z").replace(tzinfo=timezone.utc)
        names = next((lines[i+1].strip() for i, line in enumerate(lines[:-1]) if "Subject Alternative Name:" in line), "")
        cert["data"] = {"pin": pin, "token_pin": token_pin, "subject": fields["subject"],
                        "names": names, "expires_at": expires.isoformat(),
                        "days_left": (expires - datetime.now(timezone.utc)).total_seconds() / 86400}
        cert["stdout"] = "\n".join(f"{key}: {value}" for key, value in cert["data"].items()) + "\n"
    except (KeyError, ValueError) as error:
        cert.update(status=1, stderr="cannot read TLS certificate facts: " + str(error))
    return [cert, token]


def collect(section):
    if section == "tls":
        return collect_tls()
    if section != "health":
        return [execute(argv) for argv in COMMANDS[section]]
    # Device paths come only from the host's disk inventory, never the caller.
    inventory = execute(["lsblk", "--nodeps", "--noheadings", "--paths", "--output", "NAME,TYPE"])
    if inventory["status"]:
        return [inventory]
    disks = [row[0] for line in inventory["stdout"].splitlines()
             if len(row := line.split()) == 2 and row[1] == "disk" and row[0].startswith("/dev/")]
    return [execute(["smartctl", "-H", disk]) for disk in disks]


def describe():
    return metadata_answer(PACKAGE, allowed=set(available()) | {'all'})


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
            if method == 'help': return metadata_answer(PACKAGE, allowed=set(available()) | {'all'}, as_json=True)
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
    result = dispatch(PACKAGE, os.environ.get('AGENT_BUS_ASK'), handlers={'help': describe})
    if result is not None:
        print(result)
    elif len(sys.argv) != 2:
        print("error: expected one call body")
    else:
        print(answer(sys.argv[1]))


if __name__ == "__main__":
    main()
