#!/usr/bin/env python3
"""Install/remove a systemd keeper for one unsandboxed agent-bus run."""
import argparse
import fcntl
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


def connection_values():
    values = {k: os.environ[k] for k in ("AGENT_BUS_ADDR", "AGENT_BUS_TOKEN", "AGENT_BUS_TLS_FINGERPRINT") if os.environ.get(k)}
    if not values.get("AGENT_BUS_ADDR", "").startswith(("http://", "https://")) or not values.get("AGENT_BUS_TOKEN"):
        raise ValueError("set AGENT_BUS_ADDR to an HTTP(S) endpoint and AGENT_BUS_TOKEN to the caller's credential")
    if any(not re.fullmatch(r"[A-Za-z0-9_:/@.+=\[\]-]+", v) for v in values.values()):
        raise ValueError("invalid connection environment value")
    return values


def default_user(key, useradd):
    name = "ab-" + key.removeprefix("agent-bus-")
    try:
        account = pwd.getpwnam(name)
    except KeyError:
        creator = shutil.which(useradd)
        nologin = shutil.which("nologin") or "/usr/sbin/nologin"
        if creator and command([creator, "--system", "--no-create-home", "--home-dir", "/nonexistent",
                                "--shell", nologin, name], optional=True).returncode == 0:
            return name
        return "nobody"
    if account.pw_uid == 0 or account.pw_dir != "/nonexistent" or account.pw_shell not in ("/usr/sbin/nologin", "/sbin/nologin", "/bin/false"):
        raise ValueError(f"{name} already exists without the dedicated account restrictions")
    return name


def install(args):
    if not re.fullmatch(r"#[a-z0-9][a-z0-9/._-]*@[a-z0-9.-]+", args.name) or len(args.name) > 64:
        raise ValueError("name must be a canonical #agent@host name, at most 64 characters")
    key = "agent-bus-" + re.sub(r"[^a-z0-9-]+", "-", args.name[1:].split("@", 1)[0])
    unit = key + ".service"
    if not re.fullmatch(r"/[A-Za-z0-9_./-]+", args.config_dir):
        raise ValueError("config-dir must be absolute and contain only letters, digits, /, _, . or -")
    units, config = Path(args.unit_dir).absolute(), Path(args.config_dir)
    bus = shutil.which(args.bus_binary)
    ctl = shutil.which(args.systemctl)
    if not bus or not ctl:
        raise ValueError("agent-bus and systemctl must be executable")
    token_program = shutil.which(args.token_binary) if args.action == "install" else None
    if args.action == "install" and not token_program:
        raise ValueError("agent-bus-token must be executable")
    units.mkdir(parents=True, exist_ok=True)
    config.mkdir(parents=True, exist_ok=True, mode=0o700)
    unit_file, token_file = units / unit, config / (key + ".env")
    with (config / (key + ".lock")).open("a") as lock:
        os.chmod(lock.name, 0o600)
        fcntl.flock(lock, fcntl.LOCK_EX)
        marker = "# agent-bus standalone: " + args.name + "\n"
        for path in (unit_file, token_file):
            if path.is_symlink() or (path.exists() and not path.read_text().startswith(marker)):
                raise ValueError(f"{unit} already belongs to another installation")
        fragment = command([ctl, "show", "--property=FragmentPath", "--value", unit], optional=True).stdout.strip()
        if fragment and Path(fragment) != unit_file:
            raise ValueError(f"{unit} already exists at another unit path")
        if args.action == "remove":
            if not unit_file.exists() and not token_file.exists():
                return unit
            # Removal is an administrative operation by today's caller, never
            # by a credential retained from installation.
            values = connection_values()
            env = environment(values)
            if unit_file.exists():
                command([ctl, "disable", "--now", unit])
            records = json.loads(command([bus, "ls", "--all"], env).stdout)
            if any(r["name"] == args.name for r in records):
                command([bus, "unregister", args.name], env)
            # Keep the token and unit if removal was refused, so it is retryable.
            unit_file.unlink(missing_ok=True)
            token_file.unlink(missing_ok=True)
            command([ctl, "daemon-reload"])
            return unit
        if not args.script:
            raise ValueError("install requires --script")
        if args.user is not None:
            pwd.getpwnam(args.user)
        user = args.user if args.user is not None else default_user(key, args.useradd)
        script = Path(args.script).resolve(strict=True)
        if not script.is_file() or not os.access(script, os.X_OK):
            raise ValueError("script must be an executable file")
        try:
            metadata = json.loads(script.with_name("agent.json").read_text())
            description = metadata.get("description") if isinstance(metadata, dict) else None
        except (OSError, ValueError):
            description = None
        if not isinstance(description, str) or not description.strip():
            raise ValueError(f"{args.name}: agent.json description is required")
        tags = metadata.get("tags", [])
        if not isinstance(tags, list) or not all(isinstance(tag, str) and re.fullmatch(r"[a-z0-9][a-z0-9_-]*", tag) for tag in tags):
            raise ValueError(f"{args.name}: agent.json tags must be a list of simple lowercase labels")
        roles = metadata.get("roles", [])
        if not isinstance(roles, list) or not all(isinstance(role, str) and re.fullmatch(r"[a-z0-9_]+", role) and role not in ("owner", "maintainer") for role in roles):
            raise ValueError(f"{args.name}: agent.json roles must be application role names")
        values = connection_values()
        env = environment(values)
        records = json.loads(command([bus, "ls", "--all"], env).stdout)
        existing = next((r for r in records if r["name"] == args.name), None)
        if existing and not token_file.exists():
            raise ValueError("name already exists outside this installation; choose a free name")
        if existing and existing["kind"] != "agent":
            raise ValueError("installed name is no longer an agent")
        argv = [bus, "run", args.name, "--descr", description, "--stay-registered", "--package-dir", str(script.parent), "--sandbox", "off", "--algo", "args", "exec " + shlex.join([str(script), *args.script_arg])]
        if getattr(args, "serves_itself", False):
            argv = [str(script), *args.script_arg]
        if tags and not getattr(args, "serves_itself", False):
            argv += ["--tags", ",".join(tags)]
        if roles and not getattr(args, "serves_itself", False):
            argv += ["--roles", ",".join(roles)]
        if args.allow and not getattr(args, "serves_itself", False):
            argv += ["--allow", args.allow]
        content = (marker + "[Unit]\nDescription=Standalone agent-bus agent\nAfter=network.target\n\n[Service]\n"
                   + "User=" + user + "\nEnvironmentFile=" + str(token_file)
                   + "\nEnvironment=PYTHONDONTWRITEBYTECODE=1\nStateDirectory=" + key + "\nEnvironment=XDG_STATE_HOME=%S/" + key
                   + "\nExecStart=" + " ".join(quote(a) for a in argv)
                   + "\nUMask=0077\nRestart=on-failure\nRestartSec=2\nTimeoutStopSec=45\n\n[Install]\nWantedBy=multi-user.target\n")
        # Only the installation request uses the person's credential. The
        # supervised process and its on-disk environment hold the agent's own.
        register = [bus, "register", args.name, "--kind", "agent", "--descr", description]
        if tags:
            register += ["--tags", ",".join(tags)]
        if args.allow:
            register += ["--allow", args.allow]
        command(register, env)
        if roles:
            command([bus, "manage", args.name, "--roles", ",".join(roles)], env)
        agent_token = command([token_program, args.name], env).stdout.strip()
        if not re.fullmatch(r"[A-Za-z0-9_-]+", agent_token):
            raise ValueError("agent-bus-token did not return a credential")
        values["AGENT_BUS_TOKEN"] = agent_token
        if getattr(args, "serves_itself", False): values["AGENT_BUS_NAME"] = args.name
        env_text = marker + "".join(k + "=" + v + "\n" for k, v in sorted(values.items()))
        changed = not unit_file.exists() or unit_file.read_text() != content or not token_file.exists() or token_file.read_text() != env_text
        atomic(token_file, env_text, 0o600)
        atomic(unit_file, content, 0o644)
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
    p.add_argument("--user", help="unit account; default: dedicated ab-<agent> system account")
    p.add_argument("--script")
    p.add_argument("--serves-itself", action="store_true", help="run the script directly with its own bus identity")
    p.add_argument("--script-arg", action="append", default=[], help="fixed script argument; repeat as needed")
    p.add_argument("--allow", help="record ACL; omitted means the bus's restricted default")
    p.add_argument("--bus-binary", default="agent-bus")
    p.add_argument("--token-binary", default="agent-bus-token")
    p.add_argument("--systemctl", default="systemctl")
    p.add_argument("--useradd", default="useradd", help="account creation command")
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
