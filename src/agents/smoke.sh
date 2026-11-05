# The agent pack's own checks, sourced by ../smoke.sh: shard functions only,
# run with its helpers (sec, ok_exit, normal) from the src directory. An agent
# change runs these and the changed agent's shard, never the daemon's checks.

# The pack contract every agent shares: help goldens, the docs each agent owes,
# and the packaged tree built, installed and run outside the checkout.
agent_pack() {
  sec "the agent pack's shared contract"
  local out rc
  out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/shared -p test_help_golden.py -v 2>&1); rc=$?
  ok_exit "every bundled agent's help matches its golden" "$rc"
  [ "$rc" = 0 ] || echo "$out"
  out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/shared -p test_envfile.py -v 2>&1); rc=$?
  ok_exit "the shared env-file reader keeps its rules" "$rc"
  [ "$rc" = 0 ] || echo "$out"
  if normal; then
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents -p agent_docs_test.py -v 2>&1); rc=$?
    ok_exit "every bundled agent has its page, index entry and docs link" "$rc"
    [ "$rc" = 0 ] || echo "$out"
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents -p docs_order_test.py -v 2>&1); rc=$?
    ok_exit "the docs-order checker fails each way a page can break the rule, and the enforced pages follow it" "$rc"
    [ "$rc" = 0 ] || echo "$out"
    out=$(go test -count=1 ./cmd/agent-bus-setup -run '^(TestBundledAgentsRunOutsideCheckout|TestBuildShipsAgentRuntimeAndSetupKeepsItRunnable|TestPackagerOutputIsAcceptedBySetup)$' 2>&1); rc=$?
    ok_exit "the packaged agents run outside the checkout, report the pack version, and setup accepts the package" "$rc"
    [ "$rc" = 0 ] || echo "$out"
    out=$(go test -count=1 ./cmd/agent-bus-runner -run '^TestEveryBundledManifestPassesRunnerReload$' 2>&1); rc=$?
    ok_exit "every bundled manifest passes a runner reload" "$rc"
    [ "$rc" = 0 ] || echo "$out"
  fi
}
