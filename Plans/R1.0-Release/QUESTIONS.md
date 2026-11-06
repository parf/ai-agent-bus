# R1 questions

<!-- AB: #ab-ceo -->

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## Upgrading pre-1.0 data

- ❓ 🔴 Q338 How does a pre-1.0 store reach 1.0.x
  - Context: on 2026-11-05 the 1.0.9 daemon refused to start on this node's store (`record #ab-ceo: unsupported field "personal"`); the node runs 0.9.303 again until this is settled. 1.0 dropped every pre-1.0 form (CLAUDE.md "one canonical interface"), and 0.9.303 wrote some of them: `personal` and `reading` members, maintainers as a string, non-canonical tags, credentials bound lazily, and old runner inventory members
  - Context: the `docker.io/parf/agent-bus` images were pushed with 0.9.x tags since 10-30, so other people may hold 0.9 stores that 1.0.x refuses
  - ( ) A one-time offline conversion: run with the daemon stopped, after a backup; for this node now, and shipped as a documented `agent-bus-admin` upgrade step for 0.9 users 🌟
  - ( ) A one-time conversion for this node only; 0.9 container users start fresh ⭐
  - ( ) A loader that accepts the old forms again and reports them: a permanent exception to "one canonical interface"
  > 💡👾 The upgrade step: it keeps one canonical interface, costs about 30 minutes to build and 10 to run here, and helps anyone on a 0.9 image.
  > 
  > 💬👾 First step either way: a names-only check of a copy of this node's store, to see every refused form. The script is ready (`tmp/scripts/j98-store-check.sh`, read by me), but this session is not allowed to run it as root because it copies the credential store; run it yourself: `! sudo bash /usr/local/src/ai-agent-bus/tmp/scripts/j98-store-check.sh`
  >
  > 💬 👤parf (in chat) we do not care about pre 1.0 at all
  >
  > 💬👾 Understood: no upgrade step for 0.9 users and no compatible loader. Your check of this node found one form only, the retired `personal` member on ~120 records, plus 3 records with no owner (`agent-bus-runner`, `newcomer@h`, `rtest`). One choice left for this node:
  - ( ) Convert this node once, offline after a backup: drop `personal`, delete the 3 ownerless test leftovers; every record, token, KV value and queue is kept 🌟
  - ( ) Start this node on an empty 1.0 store: every agent, user, token and stored value is lost and set up again ⭐

- ❓ Q340 Which legacy records to delete in the same offline pass as №Q338
  - Context: owner, 2026-11-05: "clean up all legacy records if we have some". These have no reader now. Deleting is done in the offline conversion, after the backup; this session may not unregister records itself. AI-session names come back by themselves the next time that session starts
  - [ ] The old AB team (13): `#claude/ab-dvp-b`, `#claude/ab-dvp`, `#claude/ab`, `#claude/ab2`, `#claude/ab2-review`, `#codex/xab`, `#codex/xab2`, `#codex/ab-dvp`, `#codex/ab-dvp-3`, `#opencode/oab`, `#opencode/oc-ab`, `#opencode/oc-ab2`, and W1A's stale forks `#ab-w1a.job`, `#ab-w1a.job.2` 🌟
  - [ ] Idle AI sessions of other projects, unused for 7 days or more (about 30: the `hbfr-*`, `rd*`, `home-parf-*`, `radaris`, `realmomove`, `spartan-test`, `cod-misc`, `run-dnf-update`, `j60-live`, `random`, `oc-main` names) 🌟
  - [ ] Demo and example agents from September and October: `#demo@localhost`, `#stats-go`, `-java`, `-php`, `-python`, `-rust`, `-ts@localhost`, `#kv-example`, `#agent1@parf-dvp.difive.com`, `#dev-runner@rdvp` 🌟
  - [ ] Test users and fixtures: `alice@`, `bob@`, `carol@`, `dave@`, `eve@fresh`, `newcomer@h`, `rtest`, the Star Wars and Spaceballs fixture records (`luke@tatooine`, `leia@alderaan`, `han@corellia`, `vader@empire`, `yoda@dagobah`, `dark-helmet@spaceball`, `lone-starr@druidia`, `vespa@druidia`, `yogurt@vega`), and the ownerless `agent-bus-runner` 🌟
  - [ ] Unclear, keep unless you say: the user `maxavg`, the service `hdb3`, the group `@ops`
  > 💡👾 The first four groups; the fifth only if you know they are unused. Recent sessions (used in the last week) and `#db`, `#discord` (disabled, configured) stay.

