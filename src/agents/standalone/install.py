#!/usr/bin/env python3
"""Install/remove a systemd keeper for one unsandboxed agent-bus run."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import pwd
import re
import shlex
import shutil
import subprocess
import sys
import tempfile


def quote(value):
    if any(ord(c) < 32 for c in value):
        raise ValueError("control characters are not allowed in unit arguments")
    return '"' + value.replace('\\', '\\\\').replace('"', '\\"').replace('%', '%%').replace('$', '$$') + '"'


def atomic(path, content, mode):
    fd, tmp = tempfile.mkstemp(dir=path.parent, prefix=path.name + ".")
    try:
        os.fchmod(fd, mode)
        with os.fdopen(fd, "w") as f:
            f.write(content)
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)


def command(argv, env=None, optional=False):
    p = subprocess.run(argv, env=env, capture_output=True, text=True, timeout=45)
    if p.returncode and not optional:
        # Diagnostics from tools may contain a credential. Never echo them.
        raise RuntimeError(f"{Path(argv[0]).name} {argv[1]} failed (exit {p.returncode})")
    return p


def environment(values):
    env = {k: v for k, v in os.environ.items() if not k.startswith("AGENT_BUS_")}
    env.update(values)
    return env


def install(args):
    if not re.fullmatch(r"#[a-z0-9][a-z0-9/._-]*@[a-z0-9.-]+", args.name) or len(args.name) > 64:
        raise ValueError("name must be a canonical #agent@host name, at most 64 characters")
    key = "agent-bus-standalone-" + hashlib.sha256(args.name.encode()).hexdigest()[:16]
    unit = key + ".service"
    units, config = Path(args.unit_dir).absolute(), Path(args.config_dir).absolute()
    bus = shutil.which(args.bus_binary)
    ctl = shutil.which(args.systemctl)
    if not bus or not ctl:
        raise ValueError("agent-bus and systemctl must be executable")
    units.mkdir(parents=True, exist_ok=True)
    config.mkdir(parents=True, exist_ok=True, mode=0o700)
    unit_file, token_file = units / unit, config / (key + ".env")
    with (config / (key + ".lock")).open("a") as lock:
        os.chmod(lock.name, 0o600)
        fcntl.flock(lock, fcntl.LOCK_EX)
        if args.action == "remove":
            if not unit_file.exists() and not token_file.exists():
                return unit
            if not token_file.exists():
                raise ValueError("token file is missing; restore it before removing the registration")
            values = dict(line.split("=", 1) for line in token_file.read_text().splitlines())
            env = environment(values)
            if unit_file.exists():
                command([ctl, "disable", "--now", unit])
            records = json.loads(command([bus, "ls", "--all"], env).stdout)
            if any(r["name"] == args.name for r in records):
                command([bus, "unregister", args.name], env)
            # Keep the token and unit if removal was refused, so it is retryable.
            unit_file.unlink(missing_ok=True)
            token_file.unlink()
            command([ctl, "daemon-reload"])
            return unit
        if not args.user or not args.script:
            raise ValueError("install requires --user and --script")
        pwd.getpwnam(args.user)
        script = Path(args.script).resolve(strict=True)
        if not script.is_file() or not os.access(script, os.X_OK):
            raise ValueError("script must be an executable file")
        values = {k: os.environ[k] for k in ("AGENT_BUS_ADDR", "AGENT_BUS_TOKEN", "AGENT_BUS_TLS_FINGERPRINT") if os.environ.get(k)}
        if not values.get("AGENT_BUS_ADDR", "").startswith(("http://", "https://")) or not values.get("AGENT_BUS_TOKEN"):
            raise ValueError("set AGENT_BUS_ADDR to an HTTP(S) endpoint and AGENT_BUS_TOKEN to the launching account's token")
        if any(not re.fullmatch(r"[A-Za-z0-9_:/@.+=\[\]-]+", v) for v in values.values()):
            raise ValueError("invalid connection environment value")
        env = environment(values)
        records = json.loads(command([bus, "ls", "--all"], env).stdout)
        existing = next((r for r in records if r["name"] == args.name), None)
        if existing and not token_file.exists():
            raise ValueError("name already exists outside this installation; choose a free name")
        if existing and existing["kind"] != "agent":
            raise ValueError("installed name is no longer an agent")
        argv = [bus, "run", args.name, "--sandbox", "off", "--algo", "args", "exec " + shlex.quote(str(script))]
        if args.allow:
            argv += ["--allow", args.allow]
        content = ("[Unit]\nDescription=Standalone agent-bus agent\nAfter=network.target\n\n[Service]\n"
                   + "User=" + args.user + "\nEnvironmentFile=" + quote(str(token_file))
                   + "\nStateDirectory=" + key + "\nEnvironment=XDG_STATE_HOME=%S/" + key
                   + "\nExecStart=" + " ".join(quote(a) for a in argv)
                   + "\nUMask=0077\nRestart=on-failure\nRestartSec=2\nTimeoutStopSec=45\n\n[Install]\nWantedBy=multi-user.target\n")
        env_text = "".join(k + "=" + v + "\n" for k, v in sorted(values.items()))
        changed = not unit_file.exists() or unit_file.read_text() != content or not token_file.exists() or token_file.read_text() != env_text
        atomic(token_file, env_text, 0o600)
        atomic(unit_file, content, 0o644)
        # Register before systemd starts; rerunning uses the same name and token.
        register = [bus, "register", args.name, "--kind", "agent"]
        if args.allow:
            register += ["--allow", args.allow]
        command(register, env)
        command([ctl, "daemon-reload"])
        active = command([ctl, "is-active", "--quiet", unit], optional=True).returncode == 0
        command([ctl, "enable", unit])
        if active and changed:
            command([ctl, "restart", unit])
        elif not active:
            command([ctl, "start", unit])
        return unit


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("action", choices=["install", "remove"])
    p.add_argument("--name", required=True)
    p.add_argument("--user")
    p.add_argument("--script")
    p.add_argument("--allow", help="record ACL; omitted means the bus's restricted default")
    p.add_argument("--bus-binary", default="agent-bus")
    p.add_argument("--systemctl", default="systemctl")
    p.add_argument("--unit-dir", default="/etc/systemd/system")
    p.add_argument("--config-dir", default="/etc/agent-bus/standalone")
    try:
        print(install(p.parse_args()))
    except (ValueError, KeyError, OSError, RuntimeError, subprocess.TimeoutExpired) as e:
        # Neither command output nor environment-file contents are diagnostics.
        print(f"standalone: {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
