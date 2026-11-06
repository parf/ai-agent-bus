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

    # The mutants: the same real lists with one file taken out must go red, naming that file.
    def real(self):
        return checker.read("smoke.sh"), json.loads(checker.read("smoke-map.json")), checker.test_files()

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

    def test_a_new_unlisted_file_is_named_in_the_failure(self):
        smoke, smoke_map, tests = self.real()
        problems = checker.check(tests + ["mcp/brand-new.test.ts"], smoke, smoke_map)
        self.assertEqual([p.split(":")[0] for p in problems], ["mcp/brand-new.test.ts"])


if __name__ == "__main__":
    unittest.main()
