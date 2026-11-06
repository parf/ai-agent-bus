import importlib.util
import json
import os
import re
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("check_test_lists", os.path.join(HERE, "check-test-lists.py"))
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class TestLists(unittest.TestCase):
    def test_every_test_file_in_the_tree_runs_somewhere(self):
        self.assertEqual(checker.check(), [])

    def test_a_sourced_shard_file_counts_and_its_loss_goes_red(self):
        # The agent shards live in agents/smoke.sh, sourced by smoke.sh (T-39).
        text = checker.smoke_text()
        self.assertIn(checker.read("agents/smoke.sh"), text)
        self.assertNotRegex(checker.read("smoke.sh"), r"agent_slack\(\)")
        original = checker.read
        try:
            checker.read = lambda path: "" if path == "agents/smoke.sh" else original(path)
            problems = [p.split(":")[0] for p in checker.check()]
        finally:
            checker.read = original
        self.assertIn("agents/slack/package.test.ts", problems)

    # The mutants: the same real lists with one file taken out must go red, naming that file.
    def real(self):
        return checker.smoke_text(), json.loads(checker.read("smoke-map.json")), checker.test_files()

    def without(self, text, name):
        return re.sub(r"(?<![\w-])" + re.escape(name) + r"(?![\w-])", "gone", text)

    def test_a_file_dropped_from_the_mcp_list_goes_red(self):
        smoke, smoke_map, tests = self.real()
        smoke = self.without(smoke, "pin.test.ts")
        smoke_map = json.loads(self.without(json.dumps(smoke_map), "pin.test.ts"))
        self.assertIn("mcp/pin.test.ts", [p.split(":")[0] for p in checker.check(tests, smoke, smoke_map)])

    def test_a_pattern_narrowed_to_one_name_goes_red(self):
        smoke, smoke_map, tests = self.real()
        smoke = smoke.replace("discover -s maintenance -p 'test_*.py'", "discover -s maintenance -p test_cleanup.py")
        self.assertIn("maintenance/test_observe.py", [p.split(":")[0] for p in checker.check(tests, smoke, smoke_map)])

    def test_a_name_another_directory_shares_does_not_stand_for_it(self):
        # web/test/tags.test.ts is a directory run; mcp/tags.test.ts must be listed for itself.
        smoke, smoke_map, tests = self.real()
        smoke = self.without(smoke, "tags.test.ts")
        smoke_map = json.loads(self.without(json.dumps(smoke_map), "tags.test.ts"))
        smoke += "\n# a mention elsewhere: web/test/tags.test.ts\n"
        self.assertIn("mcp/tags.test.ts", [p.split(":")[0] for p in checker.check(tests, smoke, smoke_map)])

    def test_a_directory_rule_whose_command_is_gone_goes_red(self):
        smoke, smoke_map, tests = self.real()
        original = checker.read
        try:
            checker.read = lambda path: "" if path == "clients/check.sh" else original(path)
            self.assertTrue(any("DIRECTORY_RULES" in p for p in checker.check(tests, smoke, smoke_map)))
        finally:
            checker.read = original

    def test_an_exemption_whose_runner_forgot_the_file_goes_red(self):
        smoke, smoke_map, tests = self.real()
        original = checker.read
        try:
            checker.read = lambda path: "" if path == "npm/container-check.sh" else original(path)
            self.assertTrue(any(p.startswith("EXEMPT npm/container_test.py") for p in checker.check(tests, smoke, smoke_map)))
        finally:
            checker.read = original

    def names(self, problems):
        return [p.split(":")[0] for p in problems]

    # The mentions that are not runs (J110 review): each of these used to be credited.
    def test_a_name_only_in_a_comment_goes_red(self):
        smoke, smoke_map, tests = self.real()
        smoke = self.without(smoke, "pin.test.ts") + "\n# mcp/pin.test.ts is listed in the mcp line\n"
        smoke_map = json.loads(self.without(json.dumps(smoke_map), "pin.test.ts"))
        self.assertIn("mcp/pin.test.ts", self.names(checker.check(tests, smoke, smoke_map)))

    def test_a_trailing_comment_on_a_command_line_is_not_a_run_either(self):
        smoke, smoke_map, tests = self.real()
        smoke = self.without(smoke, "inbox.test.ts") + "\ntrue # mcp/inbox.test.ts\n"
        smoke_map = json.loads(self.without(json.dumps(smoke_map), "inbox.test.ts"))
        self.assertIn("mcp/inbox.test.ts", self.names(checker.check(tests, smoke, smoke_map)))

    def test_a_discover_line_in_a_function_nothing_calls_goes_red(self):
        smoke, smoke_map, tests = self.real()
        smoke = smoke.replace("discover -s maintenance -p 'test_*.py'", "discover -s maintenance -p test_cleanup.py")
        smoke += "\nnever_called() {\n  python3 -m unittest discover -s maintenance -p 'test_*.py' -v\n}\n"
        self.assertIn("maintenance/test_observe.py", self.names(checker.check(tests, smoke, smoke_map)))

    def test_a_function_a_check_calls_does_count(self):
        smoke, smoke_map, tests = self.real()
        smoke = smoke.replace("discover -s maintenance -p 'test_*.py'", "discover -s maintenance -p test_cleanup.py")
        smoke += "\nmaintenance_all() {\n  python3 -m unittest discover -s maintenance -p 'test_*.py' -v\n}\ncore_1() { maintenance_all; }\n"
        self.assertNotIn("maintenance/test_observe.py", self.names(checker.check(tests, smoke, smoke_map)))

    def test_a_name_only_in_the_maps_checks_paths_goes_red(self):
        smoke, smoke_map, tests = self.real()
        smoke = self.without(smoke, "pin.test.ts")
        smoke_map = json.loads(self.without(json.dumps(smoke_map), "pin.test.ts"))
        smoke_map["checks"]["container_image"]["paths"].append("src/mcp/pin.test.ts")  # paths select; they do not run
        self.assertIn("mcp/pin.test.ts", self.names(checker.check(tests, smoke, smoke_map)))

    def test_an_exemption_whose_runner_only_comments_on_the_file_goes_red(self):
        smoke, smoke_map, tests = self.real()
        original = checker.read
        try:
            checker.read = lambda path: "# runs container_test.py somewhere\ntrue\n" if path == "npm/container-check.sh" else original(path)
            self.assertTrue(any(p.startswith("EXEMPT npm/container_test.py") for p in checker.check(tests, smoke, smoke_map)))
            checker.read = lambda path: "// exec.Command is mentioned: test_fresh_install.py\npackage main\n" if path == "cmd/agent-bus-setup/fresh_install_test.go" else original(path)
            self.assertTrue(any(p.startswith("EXEMPT acceptance/test_fresh_install.py") for p in checker.check(tests, smoke, smoke_map)))
        finally:
            checker.read = original

    def test_a_script_the_gate_does_not_run_does_not_cover_what_it_runs(self):
        smoke, smoke_map, tests = self.real()
        smoke = self.without(smoke, "guard.test.sh")
        self.assertIn("acceptance/guard_liveness_test.py", self.names(checker.check(tests, smoke, smoke_map)))

    def test_a_new_unlisted_file_is_named_in_the_failure(self):
        smoke, smoke_map, tests = self.real()
        problems = checker.check(tests + ["mcp/brand-new.test.ts"], smoke, smoke_map)
        self.assertEqual([p.split(":")[0] for p in problems], ["mcp/brand-new.test.ts"])


if __name__ == "__main__":
    unittest.main()
