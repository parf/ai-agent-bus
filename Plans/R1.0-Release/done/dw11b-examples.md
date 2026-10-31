# Checkout and PHP example acceptance

📌 **TL;DR:** DW11b tests the documentation's actual commands rather than parallel copies. The two-identity PHP example passes with T-14's no-journal flags. The verbatim checkout install/update blocks pass in a clean systemd container, followed by the installed CLI version and a real call. The doc-command mutant fails on an assertion.

## PHP serve and call

Run on 2026-10-30 from `ab-w1c/DW11b`, source `51bd7468`, using fixture CLI/daemon binaries built from that checkout. `src/clients/php/tests/run.php` extracts both PHP blocks and the expected text under the README's [Try this first](../../../src/clients/php/README.md#try-this-first), registers the two names, and starts one process per identity with its own token/name.

The fixture used an empty `SASL_PATH`, `-d log_errors=0`, `-d display_errors=stderr`, and the attributed journal wrapper. Only fixture credentials were used; there was no live provider call or host-root operation.

```sh
SASL_PATH="$PWD/tmp/php-empty-sasl" CLIENT_BIN="$PWD/tmp/php-bin" timeout --kill-after=5 60 bash src/clients/php/tests/journal-quiet.sh bash -c 'exec bash src/clients/php/tests/daemon.sh php -d log_errors=0 -d display_errors=stderr -d "syslog.ident=$AB_PHP_JOURNAL_ID" src/clients/php/tests/run.php README'
```

The exact asserted call output is:

```text
sent to bob@example.com
1
```

Complete focused check output, exit 0:

```text
== the README example, run as written
  ok   the README has the two scripts and the output they print
  ok   call.php exits 0 and says nothing on stderr
  ok   the two identities print what the README says they print
  ok   serve.php wrote nothing to stderr
passed 4, failed 0
ok PHP journal gained 0 lines
```

The complete owning PHP entry point also passed: `CLIENT_BIN="$PWD/tmp/php-bin" timeout --kill-after=5 120 bash src/clients/php/tests/journal-quiet.sh bash src/clients/check.sh php`, **141 passed, 0 failed**, `ok PHP journal gained 0 lines`. This includes the new example through the same hook used by normal checks.

W2B's handoff records three output-assertion mutants at `6a79f6db`: omitted call topic, altered expected text and altered server answer. The example lives in the existing PHP conformance suite, so normal client checks run it too.

## Checkout container acceptance

Initial test source `b5b39ff2`, Debian 13 systemd fixture, historical pre-review full run **4 passed / 0 failed**, **27.109 s**, exit **0**:

```sh
GOCACHE=/usr/local/src/ai-agent-bus/tmp/gocache bash src/acceptance/install-md-container.sh src/INSTALL.md -f
```

The check extracts the actual fenced shell blocks from [INSTALL.md](../../../src/INSTALL.md#install), then runs them verbatim as an ordinary fixture user with sudo. A private served repository redirects the GitHub clone URL to this branch without editing that command. Go and Bun caches supply prerequisites; container networking is off. D-Bus provides the normal unprivileged systemctl transport. There is no preinstalled bus, live daemon or host-root action.

| Document acceptance | Owning assertion |
|---|---|
| Install builds, sets up and links programs | `InstallFromACheckout.test_1_the_install_block_builds_and_sets_up_a_node` |
| Check sees the daemon and web face active | `test_2_the_check_block_finds_both_services_active` |
| Try it serves and calls the script | `test_3_the_try_it_blocks_serve_a_script_and_answer_a_call`; installed version/stamp and exact `#hi` / `hi: ping` reply |
| Update pulls, rebuilds, restarts and still serves a call | `test_4_the_update_block_pulls_builds_and_restarts`; marker presence/content, changed daemon activation time, both units active, bounded CLI readiness, then the same version/stamp and real call assertions |

The CLI exposes its version through `--version`; no `version` verb was added. The revised fixture follows the document order (Install, Check, Try it, Update) and repeats the version and call **after** Update. The historical output below predates that ordering fix. The tracked document was unchanged: SHA256 `32de56a0b95f16b4cbd107c92ea5fabe542d4154f51701af2367a91cf3d4edc6`.

Complete check output:

```text
INSTALL.md sha256=32de56a0b95f16b4cbd107c92ea5fabe542d4154f51701af2367a91cf3d4edc6
test_1_the_install_block_builds_and_sets_up_a_node (__main__.InstallFromACheckout.test_1_the_install_block_builds_and_sets_up_a_node) ... ok
test_2_the_check_block_finds_both_services_active (__main__.InstallFromACheckout.test_2_the_check_block_finds_both_services_active) ... ok
test_3_the_update_block_pulls_builds_and_restarts (__main__.InstallFromACheckout.test_3_the_update_block_pulls_builds_and_restarts) ... ok
test_4_the_try_it_blocks_serve_a_script_and_answer_a_call (__main__.InstallFromACheckout.test_4_the_try_it_blocks_serve_a_script_and_answer_a_call) ... Installed CLI: 0.9.239
build_info: dev@install-md.test 2026-10-31 01:08:49
Documented call: {"message_id":"19be753d80d756d1","from":"#hi","to":"dev","topic":"call","tag":"3eea88c0e220","body":"hi: ping","at":"2026-10-31T01:08:52.598825036Z","expires":"0001-01-01T00:00:00Z","deadline":"0001-01-01T00:00:00Z"}
ok

----------------------------------------------------------------------
Ran 4 tests in 27.109s

OK
```

## Doc-command mutant

A copy of INSTALL.md replaces only Update's `git pull` with `true`; installation, compilation and restart still succeed. The checker reads that copy as its input, with no checker or production-code mutation:

```sh
bash src/acceptance/install-md-container.sh tmp/INSTALL-no-pull.md -f
```

The copy's SHA256 is `1cfc75150bdb92bfe7e41a76cdea78aedb3d4cea903d60e1d57edb2f4a8065d1`. Install and Check pass. Update fails on **`AssertionError: False is not true : the update did not pull the newer commit`**, at the marker-presence assertion: **3 tests, failures=1, no ERROR**, **23.390 s**, container exit **1**. This is an assertion failure after a successful build, not credit for a compilation or setup error. The baseline and mutant ran under one 15-minute smoke-gate hold; each container was removed and the lock released immediately afterward.

## Bounds and scope

- Whole driver: 900 s, then 20 s kill grace; container check exec: 800 s. Each shell block: 300 s; individual setup commands: 20 s; service queries: 10 s.
- CLI restart readiness: 15 s deadline, 3 s per probe. systemd active state alone is insufficient because the child's per-user socket returns afterward.
- A timed-out block kills its process group and bounds pipe collection; the fixture never reads a live server's stderr while constructing an assertion message. Server shutdown and pipe collection have 5 s bounds.
- The runtime and the source INSTALL.md/PHP README are unchanged. J62 is closed by `e83851fb` (snapshot before signalling) and `9abc42d2` (recognize helper-prefixed watchers), independently of J64. No bump or worker normal gate.

Initial verification: full fast smoke **27/0**, **8.091 s** wall; the three fast Go shards take **4.924 / 2.595 / 4.449 s**, all below 15 s. Changed and inbound links, scope/status and whitespace checks pass. This is fast verification, not a worker normal gate.


## Review follow-up on J64

Rebased on main containing J64 `8d0437d1` and J62/J62b `e83851fb` / `9abc42d2`. J64 fixes the independently reproduced unsafe cleanup: an unreadable live PID stat is unknown, not proof of death. It is a **possible cause**, not a proven attribution, of the reviewer's one missing `/source.git` in six runs; that original run had no guard diagnostics.

Test source `35dfc8f45cf4822f9a0f97836a10a020d293fd89` adds bounded source-mount diagnostics immediately before the fixture clone (`ls -ld /source.git` and `git -C /source.git rev-parse HEAD`), then captures the clone result in an assertion with those diagnostics. No clone retry masks a missing mount. The revised order is Install, Check, Try it, Update, followed by another version/stamp check and documented call after Update. A third agent-bus block in Try it now reports the section, expected count and actual count instead of a bare unpacking error; two parser checks cover the accepted pair and the extra block.

Six consecutive fresh Debian 13 containers passed under one bounded slow-lock batch, with no overlap with another worker's container or mutant:

| Run | Checks | Python seconds | Whole driver seconds |
|---|---|---|---|
| 1 | 6/0 | 31.310 | 33.534 |
| 2 | 6/0 | 30.760 | 33.122 |
| 3 | 6/0 | 30.628 | 32.815 |
| 4 | 6/0 | 30.059 | 32.221 |
| 5 | 6/0 | 30.080 | 32.269 |
| 6 | 6/0 | 29.948 | 32.372 |

Every run recorded the unchanged INSTALL.md SHA256 above, both source diagnostics returning 0, the expected source HEAD, and two asserted `#hi` / `hi: ping` replies. Installed version: `0.9.241`, with `dev@install-md.test` build stamps. The final run's complete output (fixture-only message identifiers) was:

```text
test_third_agent_bus_block_names_the_document_problem (__main__.DocumentBlocks.test_third_agent_bus_block_names_the_document_problem) ... ok
test_try_it_accepts_the_serve_and_call_blocks (__main__.DocumentBlocks.test_try_it_accepts_the_serve_and_call_blocks) ... ok
INSTALL.md sha256=32de56a0b95f16b4cbd107c92ea5fabe542d4154f51701af2367a91cf3d4edc6
test_1_the_install_block_builds_and_sets_up_a_node (__main__.InstallFromACheckout.test_1_the_install_block_builds_and_sets_up_a_node) ... ok
test_2_the_check_block_finds_both_services_active (__main__.InstallFromACheckout.test_2_the_check_block_finds_both_services_active) ... ok
test_3_the_try_it_blocks_serve_a_script_and_answer_a_call (__main__.InstallFromACheckout.test_3_the_try_it_blocks_serve_a_script_and_answer_a_call) ... Installed CLI: 0.9.241
build_info: dev@install-md.test 2026-10-31 01:52:09
Documented call: {"message_id":"22f80de7c4614120","from":"#hi","to":"dev","topic":"call","tag":"00d992355d47","body":"hi: ping","at":"2026-10-31T01:52:28.26602342Z","expires":"0001-01-01T00:00:00Z","deadline":"0001-01-01T00:00:00Z"}
ok
test_4_the_update_block_pulls_builds_and_restarts (__main__.InstallFromACheckout.test_4_the_update_block_pulls_builds_and_restarts) ... Update source preflight:
['ls', '-ld', '/source.git'] rc=0
drwxrwsr-x. 6 root nogroup 4096 Oct 31 01:52 /source.git

['git', '-C', '/source.git', 'rev-parse', 'HEAD'] rc=0
35dfc8f45cf4822f9a0f97836a10a020d293fd89

Installed CLI: 0.9.241
build_info: dev@install-md.test 2026-10-31 01:52:30
Documented call: {"message_id":"190e27820a10261a","from":"#hi","to":"dev","topic":"call","tag":"5fdc486a906f","body":"hi: ping","at":"2026-10-31T01:52:33.528122812Z","expires":"0001-01-01T00:00:00Z","deadline":"0001-01-01T00:00:00Z"}
ok

----------------------------------------------------------------------
Ran 6 tests in 29.948s

OK
```

The document mutant still changes only Update's `git pull` to `true`. Install, Check and Try it pass; Update fails on `the update did not pull the newer commit`: six tests, **failures=1, no ERROR**, 26.658 s Python / 28.821 s whole driver, exit 1. The mutant document has the same SHA256 recorded above. Independently removing the Try-it block-count validation fails `DocumentBlocks.test_third_agent_bus_block_names_the_document_problem` on `AssertionError not raised` (one failure, no ERROR); restoration passes both parser checks. The lock was released immediately after these checks and all containers were removed by the driver.

Revised fast verification: **27/0**, **9.064 s** wall; Go shards **5.373 / 2.555 / 4.959 s**, each below 15 s. Changed/inbound links and whitespace checks pass. No shipped behavior or documentation commands changed; no bump. J62's issue entry is closed with its independent observation-race cause and both landed commits.
