# Git autopull results

📌 **TL;DR:** the channel-driven core and post-attempt scripts are source-landed; the
owner's end-to-end rollout remains separate. The [git guide](../../../docs/agents/git.md#autopull)
owns the shipped interface.

## Scripts 0.9.162

Step 6: **`666671da`**, fast-forward and main ancestry independently verified,
VERSION **0.9.162**. The current [script guide](../../../docs/agents/git.md#after-an-attempt)
owns the accepted behaviour; no new runtime bundle paths were needed.

| Evidence | Result |
|---|---|
| Final normal under slow | **241 passed, 0 failed, 44.059 s**; lock released before any deployment |
| Restored complete Git suite | **66/0, 11.000 s** |
| Focused script/core fixtures | **25/0, 3.218 s**, including real local Git attempts and publication/checkout-lock ordering |
| Owning mutants | **35** compiled, assertion-red, production restored; no test/build-error or tool-timeout credit |
| Docs structure/index and changed links | **5/0**, **bad 0** |

Real scripts proved exact JSON stdin, cwd/euid/umask and environment isolation;
ELF and shebang fd execution; rename replacement before/after recheck; trust,
alias and interpreter checks; stdout/stderr floods; signal/timeout/group cleanup;
escaped stderr holders; empty first stderr line and 200-character sanitisation;
optional/refused logs; correlated success/fail chaining without recursion; and
per-script/shared budgets. A drifted script in repository A still permits its Git
update and emits a script failure, while repository B updates and runs its good
script. Strict explicit Configuration validation rejects that unsafe file;
runtime consumers defer file revalidation to the selected hook so all repositories
are not disabled by one drifted file. Script names remain syntactically strict.

The obsolete deploy/automatic branch was removed. The role/cap regression now
uses the real native-answer/core deploy entry, rather than a mock of that dead
operation path. Test corrections before acceptance included explicit private
parent modes, a real third upstream revision, and rendering a signal enum as its
integer value; those earlier error/failure runs were not credited as green.

Review: `#claude/ab2-review` closed on **`666671da`**, no open finding, checking
main ancestry, VERSION and production byte-identity against the reviewed
`16757231` (initial `84e64a1d`, intermediate `f2a80bb9`). Scope: static runtime
hunks in autopull.py and git_agent.py only; tests, guide/metadata and mutant runs
were not reviewed/run. All check numbers above are implementer evidence.

Deployment remains the manager's held own-reader reinstall/channel setup; AB's
owner-flow check after the webhook change remains open. No real host script,
service/unit, live checkout update or push was performed by xAB2. Actual scripts
under the unit, systemd resource behaviour and a live traffic flood are unproven.
The process-group boundary does not confine admin code that deliberately changes
session or gains privileges. The guide records inherited limits and fd/interpreter
residuals. This source landing does not claim the rollout is complete.

<details><summary>Named owning mutation checks</summary>

All owners are in test_autopull_scripts.py. Labels grouped on a row ran that
individual test, never the whole suite.

| Mutations | Assertion owner |
|---|---|
| stderr ignored, empty line ignored, unbounded line | `ScriptTests.test_any_stderr_including_empty_line_fails_and_first_line_is_200_characters` |
| stdout captured in an undrained pipe | `ScriptTests.test_stdout_flood_and_stdin_never_read_do_not_block_or_fail` |
| group kill omitted, no new session | `ScriptTests.test_normal_exit_also_cleans_background_pipe_holders` |
| deadline ignored | `ScriptTests.test_timeout_kills_group_and_fires_fail_script_with_null_exit` |
| wait for escaped holder's EOF | `ScriptTests.test_clean_leader_with_escaped_stderr_holder_is_not_a_timeout` |
| queued stderr not drained | `ScriptTests.test_signal_failure_and_buffered_stderr_of_already_exited_leader` |
| clean exit treated as timeout, inherited env, wrong cwd, wide umask | `ScriptTests.test_success_runs_after_publication_with_exact_record_cwd_uid_and_no_tokens` |
| recheck omitted | `ScriptTests.test_swap_after_open_is_refused_by_runtime_recheck` |
| name exec replaces fd exec | `ScriptTests.test_script_fd_survives_name_swap_after_final_recheck` |
| file/parent write bits, privilege bits, containment or strict config ignored | `ScriptTests.test_configuration_rejects_paths_escape_permissions_and_privilege_bits` |
| foreign owner, special file or dotdot name accepted | `ScriptTests.test_foreign_owner_special_file_and_dotdot_name_are_refused` |
| interpreter alias namespace unchecked | `ScriptTests.test_interpreter_alias_in_untrusted_directory_is_refused` |
| one drifted file rejects all runtime config | `CoreScriptFlow.test_drifted_script_does_not_block_git_or_another_repository` |
| skip runs success | `ScriptTests.test_skip_has_no_script_and_failed_attempt_runs_only_fail` |
| fail recursively fires again | `ScriptTests.test_failing_fail_script_does_not_fire_a_second_time` |
| fail stdin, published record or correlation wrong | `ScriptTests.test_both_failed_scripts_publish_once_each_and_preserve_correlation` |
| refused log suppresses fail | `ScriptTests.test_no_log_or_refused_log_never_suppresses_scripts` |
| shared limit ignored or per-script budget doubled | `ScriptTests.test_two_script_deadlines_share_one_hook_budget` |
| stdin limit ignored | `ScriptTests.test_native_elf_and_large_input_bounds` |
| script before attempt publication | `CoreScriptFlow.test_real_attempt_publishes_then_runs_script_under_locked_checkout` |

</details>

### Script design history

<details><summary>Owner-approved script design before implementation</summary>

Historical wording below predates the optional-log delivery clarification;
the current guide owns the rule that one refused/uncertain publication does not
suppress a script.

#### After an attempt

Each repository's Configuration may name two scripts, run on that server after
the attempt (owner: "autopull-success-script & autopull-fail-script"):

| Key | Runs when |
|---|---|
| `autopull-success-script` | `result` is `ok`: reload php-fpm, run tests, warm caches |
| `autopull-fail-script` | `result` is `fail`, including a failed success script: alert, roll back |

Neither runs on `skip`. A script gets the result record on stdin and runs in the
checkout as `#git`'s own user, with a time limit, after the record is published,
so a long test run delays nobody. A script **fails** when it exits non-zero,
times out, or writes anything to stderr, even with exit 0 (owner: "check STDERR
from scripts — the same fail"). A failed script adds one record to the log: `action: script`, `result: fail`, `error.code: script`,
its [`script`](README.md#result-record) outcome, and the attempt's `id`, `from` and `to`
(owner: "like success post-deploy script failed"). A failed success script also
runs the fail script. Exit 0 adds nothing.

⚠️ A script is code run on the server. Configuration is written by the record's
Owner and Maintainers alike and records no setter, so it only **chooses** a
script; the server decides which exist. Each key holds a bare file name inside
`autopull.d/` in `#git`'s state directory, put there by whoever administers the
host. A path, `..`, a symlink out of that directory, or a file or directory
writable by anyone but root or `#git`'s user is refused at configuration time
and again before each run. A script there may call the repository's own test
script, as realty's does.


</details>

## Core 0.9.160

Steps 1–5, 7 and 8: `7ea886eb`, verified fast-forward, ancestor of main and
VERSION 0.9.160. No push by xAB; the manager pushed the source.

| Evidence | Result |
|---|---|
| Final normal under slow | 1223 passed, 0 failed, 47.463 s |
| Earlier normal before manager's guide landing | 1223/0, 50.099 s |
| Focused local suite | 40/0; ~7 s unloaded, 11.500 s during another gate |
| Owning mutants | 58 compiled, each failed its named assertion; restored source |
| Docs structure / links | 5/0; 0 bad links |
| Installed private-daemon/unit fixture | Delivered roles and token-free children; direct event refused, same channel event accepted; exact own-identity records, dirty ff, mirror cleanup, Configuration refresh, log refusal with pure JSON answer, one startup catch-up, two concurrent publications |
| SSH fixture | Configured key only, no fallback, account default when no key configured; final normal passed |

Six initial mutant attempts exposed weak checks (duplicate JSON, pin refusal,
revision reachability/target, failure-path redaction, and channel realm); those
earned no credit until the checks were strengthened and rerun assertion-red.
The ignored-file mutant first caused a missing-file test error; an explicit
existence assertion was added and that owning check rerun assertion-red.
Final evidence above counts only assertion failures, never test/build errors.

Review: `#claude/ab2-review` closed on `7ea886eb`, no open HIGH/MEDIUM.
Scope: static full read of autopull.py at `acb4ff51`, git_agent/install/timers
diffs, fix round `8f03f98c`, then verified landed ancestry/version/byte identity
except the reviewed startup guard. The reviewer read no tests, mutant results,
manifest or full guide and ran no checks. Post-landing guide lows are corrected
here; dead legacy deploy/automatic cleanup was handed to xAB2 for step 6.

Deployment: the manager refreshed source and restarted the existing per-call
unit. A read-only check confirmed that it still runs `agent-bus run ... --algo
args`; this does not provide startup/event reading. The exact own-reader
installer command was sent to the manager. The manager explicitly holds that
host reinstall until scripts land and the owner configures channels. No live
autopull, actual hosting-service origin, real host timer retirement, script
execution or flood test is claimed; the owner's flow remains AB's check.
No live checkout was reset/cleaned/pushed by xAB, and no host unit was changed
by xAB. At the core landing, `after_attempt` was still the step-6 no-op extension point.

### Named owning mutants

| Mutation | Assertion owner |
|---|---|
| mirror-cap | `test_autopull.Core.test_config_mode_follow_caps_and_channels` |
| follow-cap | `test_autopull.Core.test_config_mode_follow_caps_and_channels` |
| follow-needs-channel | `test_autopull.Core.test_config_mode_follow_caps_and_channels` |
| follow-type | `test_autopull.Core.test_config_mode_follow_caps_and_channels` |
| mode-enum | `test_autopull.Core.test_config_mode_follow_caps_and_channels` |
| event-bool-version | `test_autopull.Core.test_event_parser_strict_fields_types_and_hex` |
| event-version | `test_autopull.Core.test_event_parser_strict_fields_types_and_hex` |
| event-extra-field | `test_autopull.Core.test_event_parser_strict_fields_types_and_hex` |
| push-short-sha | `test_autopull.Core.test_event_parser_strict_fields_types_and_hex` |
| event-id | `test_autopull.Core.test_event_parser_strict_fields_types_and_hex` |
| event-type | `test_autopull.Core.test_event_parser_strict_fields_types_and_hex` |
| duplicate-json | `test_autopull.Core.test_event_parser_strict_fields_types_and_hex` |
| branch-option | `test_autopull.Core.test_deploy_call_grammar_and_options` |
| rev-option | `test_autopull.Core.test_deploy_call_grammar_and_options` |
| now-type | `test_autopull.Core.test_deploy_call_grammar_and_options` |
| dirty-ff | `test_autopull.Core.test_dirty_ff_preserves_all_files_and_diverged_preserves_head` |
| diverged-ff | `test_autopull.Core.test_dirty_ff_preserves_all_files_and_diverged_preserves_head` |
| operation-state | `test_autopull.Core.test_untracked_only_and_operation_state_are_dirty` |
| mirror-clean | `test_autopull.Core.test_mirror_discards_two_files_counts_commits_and_keeps_ignored` |
| mirror-reset | `test_autopull.Core.test_mirror_discards_two_files_counts_commits_and_keeps_ignored` |
| discard-files | `test_autopull.Core.test_mirror_discards_two_files_counts_commits_and_keeps_ignored` |
| discard-commits | `test_autopull.Core.test_mirror_discards_two_files_counts_commits_and_keeps_ignored` |
| pin-bypass | `test_autopull.Core.test_deploy_branch_pinning_reachability_and_exact_revision` |
| revision-ancestry | `test_autopull.Core.test_deploy_branch_pinning_reachability_and_exact_revision` |
| revision-target | `test_autopull.Core.test_deploy_branch_pinning_reachability_and_exact_revision` |
| push-rolls-back | `test_autopull.Core.test_push_moves_tip_publishes_once_and_never_rolls_back` |
| publish-count | `test_autopull.Core.test_push_moves_tip_publishes_once_and_never_rolls_back` |
| publish-identity | `test_autopull.Core.test_push_moves_tip_publishes_once_and_never_rolls_back` |
| correlation-id | `test_autopull.Core.test_push_moves_tip_publishes_once_and_never_rolls_back` |
| remote-url-leak | `test_autopull.Core.test_records_and_answers_do_not_leak_checkout_url_or_subject` |
| error-leak | `test_autopull.Core.test_records_and_answers_do_not_leak_checkout_url_or_subject` |
| publish-failure-hooks | `test_autopull.Core.test_optional_log_failure_does_not_suppress_hook_or_corrupt_json` |
| publish-failure-result | `test_autopull.Core.test_optional_log_failure_does_not_suppress_hook_or_corrupt_json` |
| hook-before-publish | `test_autopull.Core.test_hook_runs_after_publish_under_lock_and_no_hook_without_lock` |
| hook-without-lock | `test_autopull.Core.test_hook_runs_after_publish_under_lock_and_no_hook_without_lock` |
| branch-mismatch | `test_autopull.Core.test_served_invalid_events_and_unknown_remote_or_branch` |
| unknown-remote | `test_autopull.Core.test_served_invalid_events_and_unknown_remote_or_branch` |
| channel-realm | `test_autopull.Core.test_runtime_channel_authority_both_directions_and_roles` |
| operator-role | `test_autopull.Core.test_runtime_channel_authority_both_directions_and_roles` |
| startup-check | `test_autopull.Core.test_startup_one_record_only_followed_repos` |
| startup-follow | `test_autopull.Core.test_startup_one_record_only_followed_repos` |
| child-token | `test_autopull.Core.test_child_env_token_free_and_roles_only_from_message` |
| child-ambient-role | `test_autopull.Core.test_child_env_token_free_and_roles_only_from_message` |
| retire-unlink | `test_autopull.Retirement.test_reinstall_retires_only_owned_timer_units_idempotently` |
| retire-foreign-unit | `test_autopull.Retirement.test_reinstall_retires_only_owned_timer_units_idempotently` |
| retire-order | `test_autopull.Retirement.test_reinstall_retires_only_owned_timer_units_idempotently` |
| old-timer-flag | `test_autopull.Retirement.test_removed_flag_refused_before_any_installer_action` |
| checkout-lock | `test_autopull.Core.test_hook_runs_after_publish_under_lock_and_no_hook_without_lock` |
| deploy-discard-report | `test_autopull.Core.test_every_destructive_deploy_reports_discarded_files_and_commits` |
| equal-channels | `test_autopull.Core.test_equal_channels_and_self_delivered_results_are_refused` |
| own-forward-loop | `test_autopull.Core.test_equal_channels_and_self_delivered_results_are_refused` |
| late-snapshot | `test_autopull.Core.test_discard_snapshot_taken_after_fetch` |
| startup-failure-aborts | `test_autopull.Core.test_startup_failure_does_not_abort_remaining_repositories` |
| retire-reload-idempotence | `test_autopull.Retirement.test_reinstall_retires_only_owned_timer_units_idempotently` |
| clean-ignored | `test_autopull.Core.test_mirror_discards_two_files_counts_commits_and_keeps_ignored` |
| hex-ref-shadow | `test_autopull.Core.test_hex_named_ref_does_not_override_revision_object` |
| direct-event-authority | `test_autopull.Core.test_runtime_channel_authority_both_directions_and_roles` |
| publish-without-channel | `test_autopull.Core.test_optional_log_failure_does_not_suppress_hook_or_corrupt_json` |

### Reviewed plan history

Historical implementation plan, superseded by the shipped guide above.

#### Core implementation plan

- Configuration: each `repos` entry adds `mode` (`ff` default, `mirror` only with access ≥ deploy) and `follow` (boolean, false default). Agent-wide optional `autopull-cmd` and `autopull-log` are channel names; following requires the command channel and deploy access. Only following repositories get a startup check or channel events; explicit operator attempts do not require follow. Script keys remain xAB2's separate step and are not silently accepted before it lands.
- Runtime: make the standalone keeper serve its own identity with the existing Python client (shared installer's `--serves-itself`), so it can run startup catch-up and distinguish authenticated `original_to` channel delivery from ordinary calls. Reload Configuration for each message. Legacy methods/help run in a bounded child with only the delivered roles and current Configuration added to its stripped caller environment; never change global process roles. Event and operator updates share one engine in `autopull.py`.
- Parsing: strict JSON rejects duplicate keys/non-finite values, unknown fields and malformed ids/commits/version before any write. A valid, unambiguous remote hint routes invalid served events to `invalid_event`; unknown remotes produce no record. Match origin's hosting project path (URL/SCP, stripping one `.git` suffix); never publish its URL or checkout path. Branch mismatch is ignored. Text `autopull <repo> now` and `deploy <repo> [<branch> [<rev>]]` have strict JSON equivalents with `now:true`, `branch`, `rev`, and optional `id` on deploy.
- Operations: acquire the existing per-checkout file lock with a bounded wait; budget lock + git at 60 seconds, each git process group uses the remaining budget. Fetch only origin and verify reachability before checkout/reset/clean. At most three short fetch attempts inside that budget for an announced revision not yet visible. ff refuses any dirty state and divergence before a fast-forward merge; mirror/deploy validate first, count unique dirty/untracked paths and discarded local commits, then checkout/reset/clean with fixed argv. All operations on the same checkout share that lock, including legacy calls.
- Review pins: only authenticated `original_to` equal to the canonical configured command channel takes the event path; direct JSON events stay unknown methods. Push always targets the fetched branch tip; its announced commit must be an ancestor, so old push replay cannot roll back. Strict JSON is UTF-8, <=16 KiB/depth 8, rejects duplicates/non-finite/bool version; branches use check-ref-format-compatible rules, revisions canonical lower-case hex. Mirror leaves ignored files/nested repos and refuses in-progress operations; ff treats untracked and operation state as dirty. Effective branch is stored atomically under the checkout lock; a Configuration pin takes precedence. Origins publish project paths only and errors come from a fixed table.
- Concurrency/authority: an own-identity reader dispatches to independent bounded update and legacy/read pools (two workers/eight admitted each); excess work is refused before attempt admission, no duplicate coalescing. No Git work on the reader. Legacy child env contains only authenticated AB_ROLE_* and no AGENT_BUS_TOKEN/other inherited AGENT_BUS_* credentials. Tests assert exact channel realm routing, direct-event refusal, no-role now/deploy refusal, child token absence, old-push no rollback, no hook without lock, and ignored-file counting.
- Results/publication: one exact design record per attempt, persisted atomically as last result and published immediately through the agent's own bus identity; no configured log means a no-op. Fixed safe error text only, full commit ids and project paths only. A configured log refusal/timeout never changes the git result or suppresses scripts, and is never retried. Persist a separate safe local failure note for autopull status; direct answers remain exactly the unchanged record JSON; only status and content-free stderr carry the failure note. Direct deploy answers the record JSON (callers check `.result`), not a fabricated CLI exit convention.
- Frozen step-6 seam: `state_directory()` returns `XDG_STATE_HOME/agent-bus-git` (default `~/.local/state/agent-bus-git`). `after_attempt(repo, checkout, record, publish)` is a no-op extension point, called after the single publication attempt (or no-op) while the checkout lock is STILL held, only when lock and verified checkout are available; `repo` is the validated repository entry, `checkout` a Path, `record` the complete result dict, and `publish(record)` a callable returning True for accepted/no-op and False for delivery failure, recording that failure locally. xAB2 may run at most success then fail, 30 seconds each / 60 seconds total, kill/reap the entire script process group. Its fail script receives the just-published script-failure record. Scripts select host-installed bare names under `state_directory()/autopull.d`; Owner and Maintainers may select them. The hook must not acquire another checkout lock. The core's 60-second git budget ends before the separately bounded hook budget.
- Files/checks: update git_agent.py, autopull.py, install.py and timer-retirement helper; add separate test_autopull.py plus installed private-bus fixture coverage in check.py; preserve xAB2's test_autopull_scripts.py. Extend package/setup/fixture inventories only if new runtime files are needed. Remove timer creation and replace old timer tests with owned-unit retirement/idempotence/foreign-unit refusal and old-option rejection before side effects. Update manifest args/help, end-user git guide, smoke fast allowlist and implementation README.
- Evidence: real local bare-repo and private-daemon fixtures prove every core acceptance line, exact publication counts/metadata, role/channel separation, config refresh/startup, two serialized attempts, and hook ordering/lock/rejected-publication behaviour. Named compiled assertion-red mutants cover parser fields/hex/id, remote/branch routing, cap and role boundaries, dirty/diverged ff, mirror cleanup/counts, revision reachability, startup/follow, publication optionality/counts/redaction, locking and retirement. Review source before final normal under slow; take the next free shared patch with one changelog entry, verify fast-forward ancestry, report the core hash to xAB2 immediately, manager deploys; no push.

