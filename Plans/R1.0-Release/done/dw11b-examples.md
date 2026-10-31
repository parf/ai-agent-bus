# Checkout and PHP example acceptance

📌 **TL;DR:** DW11b tests the documentation's actual commands rather than parallel copies. The two-identity PHP example passes with T-14's no-journal flags. Checkout install/update container acceptance is still pending while this branch is in progress.

## PHP serve and call

Run on 2026-10-30 from `ab-w1c/DW11b`, source `51bd7468`, using fixture CLI/daemon binaries built from that checkout. `src/clients/php/tests/run.php` extracts both PHP blocks and the expected text under the README's [Try this first](../../../src/clients/php/README.md#try-this-first), registers the two names, and starts one process per identity with its own token/name.

The fixture used an empty `SASL_PATH`, `-d log_errors=0`, `-d display_errors=stderr`, and the attributed journal wrapper. No live node, provider, credential or host-root operation was used.

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

W2B's handoff records three output-assertion mutants at `6a79f6db`: omitted call topic, altered expected text and altered server answer. The example lives in the existing PHP conformance suite, so normal client checks run it too.

## Checkout container work

The check extracts `Install`, `Check`, both `Try it` shell blocks and `Update` from [INSTALL.md](../../../src/INSTALL.md#install), then runs them verbatim as an ordinary fixture user with sudo. A private served repository redirects the GitHub clone URL to this branch without editing the command. Go and Bun caches are fixture inputs; container networking is off.

The first bounded run passed Install, then failed Check because the minimal Debian image lacked the system bus used by non-root `systemctl`. The fixture now supplies D-Bus. Test 3's former hang also had an eager `server.stderr.read()` in an assertion message; the live-pipe read is removed, shutdown is bounded, and each block and the full driver have time limits. No product or documentation command was changed to hide either failure.

The second bounded run passed Install and Check. Update pulled the marker commit, rebuilt and restarted both active units, but its immediate CLI probe raced restoration of the per-user socket (24.694 s, three tests, one assertion failure). The fixture now polls the real CLI for at most 15 s, with a 3 s bound per probe; the succeeding version and call still run only after Update. Final rerun and doc-command mutant remain pending.
