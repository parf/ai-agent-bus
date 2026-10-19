import importlib.util
import json
import os
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("server_info", Path(__file__).with_name("server-info.py"))
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)


class ServerInfo(unittest.TestCase):
    def test_all_sections_and_commands(self):
        expected = {"system", "network", "memory", "disks", "uptime", "load", "os", "cpu", "inodes", "pressure", "units", "time", "dns", "ports", "users", "virt", "health", "sensors"}
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
        for body in ["network -s", "system ; touch marker", "network\nsh", '{"method":"network","args":["-s"]}', '{"method":"network","format":"json"}', '{"method":"all","root":true}', '{"method":12}', "bogus", ""]:
            with self.subTest(body=body), patch.object(agent, "execute") as execute:
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
