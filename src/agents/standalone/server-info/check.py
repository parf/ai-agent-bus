#!/usr/bin/env python3
"""Integration checks against daemon.sh's disposable daemon, never system units."""
import json
import os
from pathlib import Path
import pwd
import re
import signal
import subprocess
import sys
import time
import unittest

HERE = Path(__file__).resolve().parent
STARTER = HERE.parent / "install.py"
FIXTURE = HERE.parent / "systemctl-fixture.py"
BIN = Path(os.environ["CLIENT_BIN"])
ROOT = Path(os.environ["AB_TEST_DIR"]) / "standalone"
ROOT.mkdir()
SCRIPT = ROOT / "host facts.py"
SCRIPT.write_bytes((HERE / "server-info.py").read_bytes())
SCRIPT.chmod(0o700)
ENV = {k: v for k, v in os.environ.items() if not k.startswith("AGENT_BUS_")}
ENV.update(AGENT_BUS_ADDR=os.environ["AB_TEST_TCP"], AGENT_BUS_TOKEN=os.environ["AB_TEST_OWNER_TOKEN"], STANDALONE_TEST_ROOT=str(ROOT))
NAME = "#server-info@localhost"


def cli(*args):
    return subprocess.run([str(BIN / "agent-bus"), *args], env=ENV, capture_output=True, text=True, timeout=30)


def starter(action):
    return subprocess.run([sys.executable, str(STARTER), action, "--name", NAME,
        "--script", str(SCRIPT),
        "--bus-binary", str(BIN / "agent-bus"), "--token-binary", str(BIN / "agent-bus-token"), "--systemctl", str(FIXTURE), "--useradd", str(FIXTURE),
        "--unit-dir", str(ROOT / "units"), "--config-dir", str(ROOT / "config")],
        env=ENV, capture_output=True, text=True, timeout=60)


class InstalledAgent(unittest.TestCase):
    def test_default_run_still_unregisters_on_graceful_stop(self):
        name = "#default-info@localhost"
        child = subprocess.Popen([str(BIN / "agent-bus"), "run", name, "--sandbox", "off", "--algo", "args", "/bin/echo"], env=ENV, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                records = json.loads(cli("ls", "--all").stdout)
                if any(r["name"] == name and r.get("readers", 0) for r in records):
                    break
                self.assertIsNone(child.poll(), "default runner exited before serving")
                time.sleep(.02)
            else:
                self.fail("default runner did not serve")
            child.terminate()
            self.assertEqual(child.wait(timeout=10), 0)
            self.assertFalse(any(r["name"] == name for r in json.loads(cli("ls", "--all").stdout)), "default run must still unregister on graceful stop")
        finally:
            if child.poll() is None:
                child.kill()
                child.wait()
            cli("unregister", name)

    def test_install_host_facts_refusal_and_remove(self):
        try:
            first = starter("install")
            self.assertEqual(first.returncode, 0, first.stderr)
            unit = first.stdout.strip()
            self.assertEqual(unit, "agent-bus-server-info.service")
            pidfile = ROOT / (unit + ".pid")
            pid = pidfile.read_text()
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                records = json.loads(cli("ls", "--all").stdout)
                if any(r["name"] == NAME and r.get("readers", 0) for r in records):
                    break
                time.sleep(.05)
            # The call itself is authoritative: a booted process alone is not service.
            p = cli("call", NAME, "--wait", "20s", '{"method":"network"}')
            self.assertEqual(p.returncode, 0, p.stderr)
            body = json.loads(p.stdout)["body"]
            network = json.loads(body)["network"]
            self.assertEqual(network[0]["status"], 0)
            interfaces = set(re.findall(r"^\d+: ([^:@]+)", network[0]["stdout"], re.M))
            self.assertTrue(interfaces - {"lo"}, "host-facts: ip a must show more than lo, not a sandbox network")
            self.assertEqual(network[1]["command"], ["ip", "r"])
            for request in ["network -s", "system ; echo caller-command", '{"method":"system","args":["-s"]}']:
                p = cli("call", NAME, "--wait", "10s", request)
                self.assertEqual(p.returncode, 0, p.stderr)
                self.assertIn("error", json.loads(p.stdout)["body"], "no-arguments refusal")
            p = cli("call", NAME, "--wait", "10s", "system")
            self.assertIn(os.uname().nodename, json.loads(p.stdout)["body"])
            p = cli("call", NAME, "--wait", "30s", '{"method":"all"}')
            all_sections = json.loads(json.loads(p.stdout)["body"])
            self.assertTrue({"system", "network", "memory", "disks", "uptime", "load", "os", "cpu", "inodes", "pressure", "units", "time", "dns", "ports", "users", "virt"} <= set(all_sections))
            if os.geteuid() != 0:
                self.assertNotIn("health", all_sections)
                self.assertNotIn("sensors", all_sections)
            second = starter("install")
            self.assertEqual(second.returncode, 0, second.stderr)
            self.assertEqual(second.stdout, first.stdout)
            self.assertEqual(pidfile.read_text(), pid, "idempotent install must not restart an unchanged unit")
            self.assertEqual(len(list((ROOT / "units").glob("*.service"))), 1)
            self.assertEqual(sum(r["name"] == NAME for r in json.loads(cli("ls", "--all").stdout)), 1)
            tokens = list((ROOT / "config").glob("*.env"))
            self.assertEqual(len(tokens), 1)
            stored = dict(line.split("=", 1) for line in tokens[0].read_text().splitlines() if not line.startswith("#"))
            self.assertNotEqual(stored["AGENT_BUS_TOKEN"], ENV["AGENT_BUS_TOKEN"], "installer credential must never be stored")
            identity = subprocess.run([str(BIN / "agent-bus"), "status"], env={**ENV, **stored}, capture_output=True, text=True, timeout=10)
            self.assertEqual(identity.returncode, 0, identity.stderr)
            self.assertEqual(json.loads(identity.stdout)["you"], NAME, "stored credential must belong to the agent")
            self.assertEqual(tokens[0].stat().st_mode & 0o777, 0o600)
            unit_text = (ROOT / "units" / unit).read_text()
            self.assertIn('"--sandbox" "off"', unit_text)
            self.assertIn("EnvironmentFile=", unit_text)
            self.assertIn("User=ab-server-info\n", unit_text)
            self.assertTrue(ENV["AGENT_BUS_TOKEN"] not in unit_text + first.stdout + first.stderr + second.stdout + second.stderr, "credential must not be printed or embedded in the unit")
            verified = subprocess.run(["systemd-analyze", "verify", "--man=no", str(ROOT / "units" / unit)], capture_output=True, text=True, timeout=20)
            self.assertEqual(verified.returncode, 0, verified.stderr)
            self.assertEqual(verified.stdout + verified.stderr, "", "systemd verification must have no warnings")
            stopped = subprocess.run([str(FIXTURE), "disable", "--now", unit], env=ENV, capture_output=True, text=True, timeout=45)
            self.assertEqual(stopped.returncode, 0, stopped.stderr)
            self.assertTrue(any(r["name"] == NAME for r in json.loads(cli("ls", "--all").stdout)), "--stay-registered must retain the record on graceful stop")
            restarted = subprocess.run([str(FIXTURE), "restart", unit], env=ENV, capture_output=True, text=True, timeout=45)
            self.assertEqual(restarted.returncode, 0, restarted.stderr)
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                if any(r["name"] == NAME and r.get("readers", 0) for r in json.loads(cli("ls", "--all").stdout)):
                    break
                time.sleep(.05)
            self.assertIn(os.uname().nodename, json.loads(cli("call", NAME, "--wait", "10s", "system").stdout)["body"])
            removed = starter("remove")
            self.assertEqual(removed.returncode, 0, removed.stderr)
            self.assertFalse(list((ROOT / "units").glob("*.service")))
            self.assertFalse(list((ROOT / "config").glob("*.env")))
            self.assertFalse(any(r["name"] == NAME for r in json.loads(cli("ls", "--all").stdout)))
            self.assertFalse(pidfile.exists())
            self.assertEqual(starter("remove").returncode, 0)
            # A killed runner leaves a real orphan: removal must unregister it,
            # rather than relying only on the runner's graceful-stop cleanup.
            self.assertEqual(starter("install").returncode, 0)
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                if any(r["name"] == NAME and r.get("readers", 0) for r in json.loads(cli("ls", "--all").stdout)):
                    break
                time.sleep(.05)
            os.kill(int(pidfile.read_text()), signal.SIGKILL)
            deadline = time.monotonic() + 5
            while time.monotonic() < deadline:
                records = json.loads(cli("ls", "--all").stdout)
                if any(r["name"] == NAME and not r.get("readers", 0) for r in records):
                    break
                time.sleep(.05)
            self.assertTrue(any(r["name"] == NAME for r in records), "crash fixture must leave a registration")
            self.assertEqual(starter("remove").returncode, 0)
            self.assertFalse(any(r["name"] == NAME for r in json.loads(cli("ls", "--all").stdout)), "removal must leave no orphan record")
            self.assertFalse(list((ROOT / "units").glob("*.service")))
            self.assertFalse(list((ROOT / "config").glob("*.env")))
        finally:
            removed = starter("remove")
            if removed.returncode:
                # Only a fixture-owned child may need a last-resort stop.
                for pidfile in ROOT.glob("*.pid"):
                    subprocess.run([str(FIXTURE), "disable", "--now", pidfile.name[:-4]], env=ENV, timeout=45)


if __name__ == "__main__":
    unittest.main(verbosity=2)
