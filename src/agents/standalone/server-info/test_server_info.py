import importlib.util
import json
import os
import subprocess
import sys
import tempfile
import hashlib
import ssl
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("server_info", Path(__file__).with_name("server-info.py"))
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)


class ServerInfo(unittest.TestCase):
    def test_tls_public_certificate_and_fixed_helper(self):
        scratch = Path(__file__).resolve().parents[4] / 'tmp'
        scratch.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=scratch) as directory:
            cert = Path(directory) / 'cert.pem'
            subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
                            '-keyout', str(Path(directory)/'key.pem'), '-out', str(cert),
                            '-days', '45', '-subj', '/CN=fixture.test',
                            '-addext', 'subjectAltName=DNS:fixture.test,IP:127.0.0.1'],
                           check=True, capture_output=True)
            expected = 'sha256:' + hashlib.sha256(ssl.PEM_cert_to_DER_cert(cert.read_text())).hexdigest()
            original = agent.execute
            seen = []
            def execute(argv):
                seen.append(argv)
                if argv == ['agent-bus', 'token', '--fingerprint']:
                    return {'command':argv, 'stdout':expected+'\n', 'stderr':'', 'status':0}
                self.assertEqual(argv, agent.COMMANDS['tls'][0])
                return original([str(cert) if value == '/etc/agent-bus/tls/cert.pem' else value for value in argv])
            with patch.object(agent, 'execute', side_effect=execute):
                rows = json.loads(agent.answer('{"method":"tls"}'))['tls']
                self.assertEqual([row['status'] for row in rows], [0, 0])
                facts = rows[0]['data']
                self.assertEqual(facts['pin'], expected)
                self.assertEqual(facts['token_pin'], expected)
                self.assertIn('fixture.test', facts['subject'])
                self.assertIn('DNS:fixture.test', facts['names'])
                self.assertGreater(facts['days_left'], 44.9)
                self.assertLessEqual(facts['days_left'], 45)
                self.assertIn('expires_at:', agent.answer('tls'))
            self.assertIn(['agent-bus', 'token', '--fingerprint'], seen)

    def test_tls_missing_openssl_is_unavailable(self):
        with patch.object(agent, 'execute', side_effect=lambda argv: {'command':argv, 'stdout':'', 'stderr':'command not installed', 'status':127}):
            text = agent.answer('tls')
        self.assertIn('unavailable: openssl not installed', text)
        self.assertNotIn('[exit', text)

    def test_tls_failure_never_becomes_facts(self):
        for status, stdout in [(1, ''), (0, 'not a certificate')]:
            with patch.object(agent, 'execute', side_effect=lambda argv: {'command':argv, 'stdout':stdout, 'stderr':'unavailable', 'status':status}):
                rows = agent.collect('tls')
            self.assertNotIn('data', rows[0])
            self.assertNotEqual(rows[0]['status'], 0)

    def test_bare_metal_is_success_only_for_detect_virt_none(self):
        for argv, stdout, status, expected in [
            (["systemd-detect-virt"], "none\n", 1, 0),
            (["systemd-detect-virt"], "none\n", 2, 2),
            (["systemd-detect-virt"], "broken\n", 1, 1),
            (["sensors"], "none\n", 1, 1),
        ]:
            with self.subTest(argv=argv, stdout=stdout, status=status), patch.object(agent.subprocess, "run", return_value=subprocess.CompletedProcess(argv, status, stdout, "")):
                self.assertEqual(agent.execute(argv)["status"], expected)
                if expected == 0:
                    self.assertNotIn("[exit", agent.answer("virt"))
                    self.assertIn("none", agent.answer("virt"))

    def test_help_forms_and_every_section(self):
        for uid in [0, 1000]:
            with patch.object(agent.os, "geteuid", return_value=uid):
                text = agent.answer("help")
                for section in agent.available():
                    self.assertIn(section + ":", text)
                if uid:
                    self.assertNotIn("health:", text)
                    self.assertNotIn("sensors:", text)
        program = str(Path(__file__).with_name("server-info.py"))
        text = subprocess.check_output([sys.executable, program, "help"], text=True)
        self.assertEqual(text, subprocess.check_output([sys.executable, program], env={**os.environ, "AGENT_BUS_ASK": "help"}, text=True))

    def test_all_sections_and_commands(self):
        expected = {"system", "network", "memory", "disks", "uptime", "load", "os", "cpu", "inodes", "pressure", "units", "time", "dns", "ports", "users", "virt", "health", "sensors", "tls"}
        seen = []
        def execute(argv):
            seen.append(argv)
            return {"command": argv, "stdout": "/dev/sda disk\n/dev/sda1 part\n" if "--nodeps" in argv else "host facts", "stderr": "", "status": 0}
        with patch.object(agent.os, "geteuid", return_value=0), patch.object(agent, "execute", side_effect=execute):
            result = json.loads(agent.answer('{"method":"all"}'))
        self.assertEqual(set(result), expected)
        self.assertIn(["smartctl", "-H", "/dev/sda"], seen)
        self.assertNotIn(["smartctl", "-H", "/dev/sda1"], seen)
        self.assertIn(["sensors"], seen)
        self.assertIn(["ip", "a"], seen)
        self.assertIn(["ip", "r"], seen)
        self.assertIn(["free"], seen)
        self.assertTrue(any("MODEL" in " ".join(a) and "SERIAL" in " ".join(a) for a in seen))

    def test_unprivileged_sections_are_absent(self):
        with patch.object(agent.os, "geteuid", return_value=1000), patch.object(agent, "collect", return_value=[]) as collect:
            self.assertNotIn("health", json.loads(agent.answer('{"method":"all"}')))
            self.assertNotIn("sensors", agent.available())
            self.assertEqual(agent.answer('{"method":"health"}'), "{}")
            self.assertEqual(agent.answer("sensors"), "")
            self.assertNotIn("health", [c.args[0] for c in collect.call_args_list])
            self.assertNotIn("sensors", [c.args[0] for c in collect.call_args_list])

    def test_no_arguments_ever_reach_a_command(self):
        row = {"command": ["uname", "-a"], "stdout": "host facts", "stderr": "", "status": 0}
        for body in ["network -s", "system ; touch marker", "network\nsh", '{"method":"network","args":["-s"]}', '{"method":"network","format":"json"}', '{"method":"all","root":true}', '{"method":12}', "bogus", ""]:
            with self.subTest(body=body), patch.object(agent, "execute", return_value=row) as execute:
                self.assertIn("error", agent.answer(body))
                execute.assert_not_called()

    def test_text_and_json_describe_the_same_fixed_commands(self):
        row = {"command": ["uname", "-a"], "stdout": "actual host\n", "stderr": "", "status": 0}
        with patch.object(agent, "execute", return_value=row):
            self.assertIn("actual host", agent.answer("system"))
            self.assertEqual(json.loads(agent.answer('{"method":"system"}')), {"system": [row]})

    def test_command_refusal_is_an_answer(self):
        row = {"command": ["sensors"], "stdout": "", "stderr": "No sensors found", "status": 1}
        with patch.object(agent.os, "geteuid", return_value=0), patch.object(agent, "execute", return_value=row):
            self.assertEqual(json.loads(agent.answer('{"method":"sensors"}'))["sensors"][0]["status"], 1)
            self.assertIn("No sensors found", agent.answer("sensors"))


if __name__ == "__main__":
    unittest.main()
