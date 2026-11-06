# The agent pack's own checks, sourced by ../smoke.sh: shard functions only,
# run with its helpers (sec, ok_exit, normal) from the src directory. An agent
# change runs these and the changed agent's shard, never the daemon's checks.

# `^(TestA|TestB)$` for every test function in <package>/<name>_test.go.
agent_go_tests() {
  local dir=$1; shift
  local names; names=$(for f in "$@"; do sed -n 's/^func \(Test[A-Za-z0-9_]*\)(t \*testing\.T).*/\1/p' "$dir/${f}_test.go"; done | paste -sd'|')
  printf '^(%s)$' "$names"
}

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
  # The agent shards' own Bun wrappers keep a failure's diff and location.
  out=$(python3 -B smoke_bun_test.py 2>&1); rc=$?
  ok_exit "the agent shards' Bun failures retain their diff and source location" "$rc"
  [ "$rc" = 0 ] || echo "$out"
  if normal; then
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents -p agent_docs_test.py -v 2>&1); rc=$?
    ok_exit "every bundled agent has its page, index entry and docs link" "$rc"
    [ "$rc" = 0 ] || echo "$out"
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents -p docs_order_test.py -v 2>&1); rc=$?
    ok_exit "the docs-order checker fails each way a page can break the rule, and the enforced pages follow it" "$rc"
    [ "$rc" = 0 ] || echo "$out"
    # The Go tests that run agent code: an agent-only change selects no Go
    # check, so they run here, every test of each file that executes src/agents.
    out=$(go test -count=1 ./cmd/agent-bus-setup -run "$(agent_go_tests cmd/agent-bus-setup agent_runtime package_agents_listed package_delivery package_install review_fixes)" 2>&1); rc=$?
    ok_exit "the packaged agents run outside the checkout, report the pack version, and setup accepts the package" "$rc"
    [ "$rc" = 0 ] || echo "$out"
    out=$(go test -count=1 ./cmd/agent-bus-runner -run "$(agent_go_tests cmd/agent-bus-runner bundled_metadata checkout_bun checkout_sandbox debug_doctor imap_poll install manifest_golden receipt_deadline secret_read)" 2>&1); rc=$?
    ok_exit "the runner starts, sandboxes, deploys and masks the bundled agents" "$rc"
    [ "$rc" = 0 ] || echo "$out"
    out=$(go test -count=1 ./cmd/agent-bus-runner -run '^TestEveryBundledManifestPassesRunnerReload$' 2>&1); rc=$?
    ok_exit "every bundled manifest passes a runner reload" "$rc"
    [ "$rc" = 0 ] || echo "$out"
    out=$(go test -count=1 ./internal/agentmeta ./internal/envfile 2>&1); rc=$?
    ok_exit "agent metadata help and the Python env-file vectors" "$rc"
    [ "$rc" = 0 ] || echo "$out"
  fi
}

# Each agent's own shard, moved from ../smoke.sh unchanged.

agent_packages() {
  if normal; then
    sec "agent package replacement, import and rollback contract"
    local out rc
    out=$(python3 -B ./agents/package_tree_test.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "agent packages replace old code trees and preserve the previous tree on failure" "$rc"
  else skipped=$((skipped+1)); fi
}

# The Slack agent's own suite (src/agents/slack): a real daemon from this
# build, the actual agent, a fake Slack with a Socket Mode websocket
# (Plans/R1.2/done/slack/TODO.md S.7).
agent_telegram() {
  if normal; then
    sec "telegram agent"
    local out rc
    local pattern=()
    [ -z "${SMOKE_TEST_PATTERN:-}" ] || pattern=(--test-name-pattern "$SMOKE_TEST_PATTERN")
    out=$(SMOKE_TEST_SET="$(level)" SMOKE_BIN="$D" timeout 240 python3 ./smoke-agent-pool.py ./agents/telegram "${pattern[@]}" 2>&1); rc=$?
    bun_test_text "$rc" <<<"$out"
    ok_exit "telegram agent tests" "$rc"
  else
    skipped=$((skipped+1))
  fi
}

agent_discord() {
  if normal; then
    sec "Discord methods, gateway and shared subscriptions on private fixtures"
    local out rc
    local pattern=()
    [ -z "${SMOKE_TEST_PATTERN:-}" ] || pattern=(--test-name-pattern "$SMOKE_TEST_PATTERN")
    out=$(SMOKE_BIN="$D" timeout 60 bun test "${pattern[@]}" ./agents/discord 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "Discord agent fake API, gateway, bus and package checks" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_slack() {
  if normal; then
    sec "slack agent"
    local out rc
    local pattern=()
    [ -z "${SMOKE_TEST_PATTERN:-}" ] || pattern=(--test-name-pattern "$SMOKE_TEST_PATTERN")
    out=$(SMOKE_TEST_SET="$(level)" SMOKE_BIN="$D" timeout 240 python3 ./smoke-agent-pool.py ./agents/slack "${pattern[@]}" 2>&1); rc=$?
    bun_test_text "$rc" <<<"$out"
    ok_exit "slack agent tests" "$rc"
    out=$(timeout 60 bun test "${pattern[@]}" ./agents/shared/subscriptions.test.ts 2>&1); rc=$?
    bun_test_text "$rc" <<<"$out"
    ok_exit "shared subscription engine tests" "$rc"
  else
    skipped=$((skipped+1))
  fi
}

agent_server_info() {
  if normal; then
    sec "server-info and standalone installation"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/standalone/server-info -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "server-info fixed sections and argument refusal" "$rc"
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/debug -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "debug agent doctor and required-settings tests" "$rc"
    out=$(PYTHONDONTWRITEBYTECODE=1 CLIENT_BIN="$D" timeout 120 ./clients/java/tests/daemon.sh python3 ./agents/standalone/server-info/check.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "standalone install, host facts and removal" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_systemd() {
  if normal; then
    sec "systemd roles, declared units and standalone privilege"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/standalone/systemd -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "systemd role tiers and exact unit scope" "$rc"
    out=$(PYTHONDONTWRITEBYTECODE=1 CLIENT_BIN="$D" timeout 120 ./clients/java/tests/daemon.sh sudo -n --preserve-env=AB_TEST_TCP,AB_TEST_OWNER,AB_TEST_OWNER_TOKEN,AB_TEST_DIR,CLIENT_BIN,PYTHONDONTWRITEBYTECODE python3 ./agents/standalone/systemd/check.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "systemd standalone install and real privilege loss" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_git() {
  if normal; then
    sec "git declared repositories, roles and SSH identity"
    local out rc
    local fixture; fixture=$(client_fixture) || { ok_exit "autopull bus fixture is present" 1; return; }
    out=$(AUTOPULL_BUS_FIXTURE="$fixture" SMOKE_TEST_SET="$(level)" PYTHONDONTWRITEBYTECODE=1 AGENT_BUS_TMP="$D" python3 -m unittest discover -s agents/standalone/git -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "git operations and refusal boundaries" "$rc"
    out=$(sudo -n env AB_TEST_DIR="$D" PYTHONDONTWRITEBYTECODE=1 python3 ./agents/standalone/git/ssh_check.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "git configured SSH key and account fallback" "$rc"
    out=$(PYTHONDONTWRITEBYTECODE=1 CLIENT_BIN="$D" timeout 120 ./clients/java/tests/daemon.sh sudo -n --preserve-env=AB_TEST_TCP,AB_TEST_OWNER,AB_TEST_OWNER_TOKEN,AB_TEST_DIR,CLIENT_BIN,PYTHONDONTWRITEBYTECODE python3 ./agents/standalone/git/check.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "git standalone installation and delivered roles" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_logwatch() {
  if normal; then
    sec "logwatch ring, sources and subscriptions"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/standalone/logwatch -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "logwatch bounded history, cleaning, rescans and journals" "$rc"
    out=$(PYTHONDONTWRITEBYTECODE=1 CLIENT_BIN="$D" timeout 120 ./clients/java/tests/daemon.sh python3 ./agents/standalone/logwatch/check.py 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "logwatch standalone serving, ACL and destination subscriptions" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_sessions() {
  if normal; then
    sec "configured session set and detached tmux startup"
    local out rc
    local systemd_check=0
    if slow; then systemd_check=1; fi
    out=$(AB_SESSIONS_SYSTEMD_CHECK=$systemd_check PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/standalone/tmux-sessions -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "session argv, configuration, collisions and real sleeping stub" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_ai() {
  if normal; then
    sec "AI proxy pass-through and private provider credential"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/ai -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "AI proxy provider transport, bounds and key secrecy" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_voyage() {
  if normal; then
    sec "Voyage proxy byte preservation and private provider credential"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/voyage -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "Voyage transport, bounds, path and key secrecy" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_db() {
  if normal; then
    sec "Database proxy: read enforced by the database on MySQL, PostgreSQL and ClickHouse"
    local out rc
    # The whole run, under journal-quiet: no fixture container or podman event of it reaches the host journal (J32).
    out=$(PYTHONDONTWRITEBYTECODE=1 ./journal-quiet.sh ab-db-fixture python3 -m unittest discover -s agents/db -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "database proxy roles, read-only enforcement, bounds and password secrecy" "$rc"
    has "the database fixtures wrote nothing to the host journal" "$out" "^journal-quiet: no journal entry for ab-db-fixture$"
    # A fixture container whose test process is gone was leaked (J32); a neighbour's live run is not one.
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 agents/db/fixtures.py leaked 2>&1)
    is_empty "no database fixture container outlives its test" "$out"
  else skipped=$((skipped+1)); fi
}

agent_imap() {
  if normal; then
    sec "IMAP bounded private UID cache and raw external publication"
    local out rc
    local fixture; fixture=$(client_fixture) || { ok_exit "IMAP bus fixture is present" 1; return; }
    out=$(IMAP_BUS_FIXTURE="$fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/imap -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "IMAP TLS/auth, bounded literals/cache, raw publication and exported reader" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_eventlog() {
  if normal; then
    sec "eventlog rings, persistent alerts, roles and once-only subscriptions"
    local out rc
    local fixture; fixture=$(client_fixture) || { ok_exit "eventlog bus fixture is present" 1; return; }
    out=$(EVENTLOG_BUS_FIXTURE="$fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/eventlog -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "eventlog bounds, fold/restart, pull/clean, direct roles, subscription-first and package" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_subscriptions() {
  sec "shared subscription vectors, authority and bounded delivery"
  local out rc
  out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/shared -p test_subscriptions.py -v 2>&1); rc=$?
  echo "$out" | sed 's/^/  /'
  ok_exit "subscription parity and bounded delivery" "$rc"
  out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/shared -p test_change_sync.py -v 2>&1); rc=$?
  echo "$out" | sed 's/^/  /'
  ok_exit "change-sync core: state file, authority, confirmed delivery, poll timer" "$rc"
  out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest agents.cron.test_scheduler_state -v 2>&1); rc=$?
  echo "$out" | sed 's/^/  /'
  ok_exit "canonical scheduler state and interrupted-send recovery" "$rc"
}

agent_cron() {
  if normal; then
    sec "cron subscriptions and one-shots: destination authority, single firing, RD138 delivery"
    local out rc
    local fixture; fixture=$(client_fixture) || { ok_exit "cron bus fixture is present" 1; return; }
    out=$(CRON_BUS_FIXTURE="$fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/cron -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "cron parser, /manages at add and firing, one sender per slot, restarts and unconfirmed pauses" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_at() {
  if normal; then
    sec "at: deferred one-shot calls on the shared scheduler engine"
    local out rc
    local fixture; fixture=$(client_fixture) || { ok_exit "at bus fixture is present" 1; return; }
    out=$(CRON_BUS_FIXTURE="$fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/at -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "at grammar and HH:MM, one send per call, restarts, ACL pauses, shared engine packaged unchanged" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_smtp() {
  if normal; then
    sec "SMTP verified TLS, account isolation and one submission"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/smtp -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "SMTP recipients, TLS/AUTH, MIME, bounds and delivery outcomes" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_qdrant() {
  if normal; then
    sec "Qdrant configured URL, application roles and private key"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/qdrant -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "Qdrant tiers, exact API routes, transport bounds and key secrecy" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_rag() {
  if normal; then
    sec "RAG core, prompt cache, disposable bus block identities and keyword search on a real Qdrant"
    local out rc
    local fixture; fixture=$(client_fixture) || { ok_exit "RAG bus fixture is present" 1; return; }
    # The real-Qdrant check starts a container like the database proxy's fixtures: under the same journal-quiet wrapper (J32),
    # bounded so a missing podman fails instead of hanging, and its own run's containers are counted by label at the end.
    out=$(RAG_BUS_FIXTURE="$fixture" PYTHONDONTWRITEBYTECODE=1 timeout 300 ./journal-quiet.sh ab-db-fixture python3 -m unittest discover -s agents/rag -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "RAG scoped storage, provider preparation, cache, identity and BM25 keyword search" "$rc"
    has "the RAG fixtures wrote nothing to the host journal" "$out" "^journal-quiet: no journal entry for ab-db-fixture$"
    has "and no container of the RAG run remains" "$out" "RAG fixture containers left: 0"
  else skipped=$((skipped+1)); fi
}

agent_fetch() {
  if normal; then
    sec "Fetch pinned public addresses, verified TLS, bounded workers and readable text"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -B -m unittest discover -s agents/fetch -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "fetch SSRF, redirects, TLS, framing, deadlines, extraction and offline package" "$rc"
  else
    fast_check agent_fetch
  fi
}

agent_webhook() {
  if normal; then
    sec "Webhook authenticated routes, bounded HTTP and private configured outbound"
    local out rc
    local fixture; fixture=$(client_fixture) || { ok_exit "webhook bus fixture is present" 1; return; }
    out=$(WEBHOOK_BUS_FIXTURE="$fixture" PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/webhook -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "webhook auth, exact delivery, concurrency, TLS, deadlines, caps and package" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_sms() {
  if normal; then
    sec "SMS provider at-most-once transport and private key"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/sms-telnyx -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "SMS validation, provider boundary and key secrecy" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_filescan() {
  if normal; then
    sec "filescan: link-free walk, guard, inotify, RAG form, confirmed delivery"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s agents/filescan -p 'test_*.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "filescan scan, watch, guard, roles, RAG form and package" "$rc"
  else skipped=$((skipped+1)); fi
}

agent_dbsync() {
  if normal; then
    sec "dbsync: statements per dialect, cursor and paging, lookback, soft delete, confirmed delivery, real MySQL and PostgreSQL"
    local out rc
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 -m unittest agents.dbsync.test_dbsync agents.dbsync.test_statements_golden -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "dbsync statements, cursor, lookback, soft delete, park and confirmed delivery against a fake DB agent" "$rc"
    # The real engines through the DB agent's own code, under journal-quiet like the database proxy's fixtures (J32).
    out=$(PYTHONDONTWRITEBYTECODE=1 timeout 300 ./journal-quiet.sh ab-db-fixture python3 -m unittest discover -s agents/dbsync -p 'test_dbsync_engines.py' -v 2>&1); rc=$?
    echo "$out" | sed 's/^/  /'
    ok_exit "dbsync pages, catches a late commit once and sends a soft delete on MariaDB and PostgreSQL" "$rc"
    has "the dbsync engine fixtures wrote nothing to the host journal" "$out" "^journal-quiet: no journal entry for ab-db-fixture$"
    out=$(PYTHONDONTWRITEBYTECODE=1 python3 agents/db/fixtures.py leaked 2>&1)
    is_empty "no dbsync engine container outlives its test" "$out"
  else skipped=$((skipped+1)); fi
}
