# Trusted person-name imports

📌 **TL;DR:** 0.5.58 fills blank person names from Linux account data and
GitHub enrolment without accepting caller-stated profile text.

## Result

`agent-bus-admin user import-local <user@realm> <account>` reads the account
through the OS account database and copies its person name into an otherwise
blank profile. Setup runs that adapter after it installs the first user's key.
The command accepts no free-form person name and preserves the profile's other
fields.

The GitHub directory adapter returns the public keys and provider-stated person
name in one lookup result. The enrolment challenge retains both. A successful
signature imports the retained name; a wrong signature or invalid name creates
nothing. File-backed directories continue to state keys only.

Both imports fill blanks. An Administrator-entered non-empty name wins over a
later setup run or re-enrolment.

## Checks

Core and adapter tests cover retained provider facts, wrong proof, invalid
provider names, explicit-name precedence, a key-only directory, both GitHub
requests and failures, OS-account lookup, strict command arguments and
preservation of unrelated profile fields. The main smoke runs the real Linux
adapter against `passwd` and verifies that it does not replace an explicit
name.

The final mutation set caught 12/12 named changes across challenge retention,
profile import and validation, explicit-name precedence, GitHub and file
adapters, OS lookup, strict input and field preservation. The first mutation
run receives no credit: its free-form-input mutant reached account lookup but
the test accepted a later unrelated error. The corrected test asserts that
invalid input never reaches the trusted lookup and catches the mutant.

Fast smoke passed 478 checks. The byte-frozen slow run passed 599 checks with
`go vet` and the race run green. All 193 source hashes remained unchanged. The
tracked and copied smoke scripts both had SHA-256
`4a1c40adc49ca000254528dc28c3f746f1e4faabd2d5c409e0f330bd01bad498`.
Documentation validation checked 147 files and 2,821 local links with zero
errors.

## Limits

GitHub enrolment now requires both the public-key and user-profile endpoints to
answer. They are two requests retained as one adapter result, not an atomic
provider transaction. A GitHub profile may publish no person name; enrolment
then succeeds with the field blank.

Automatic Linux import is attached to setup's first-key provisioning. When
setup finds no key, the operator may run `user import-local` later. The current
profile does not expose which trusted source originally supplied a name; no
historical provenance is reconstructed.

## Live postflight

Commit `23d5972` was pushed before deployment. `src/build.sh` stamped the
programs as **0.5.58**, `parf@parf.us 2026-09-17 13:32:32`; the live unit
restarted at 13:33 EDT.

The first unprivileged `systemctl restart` request timed out without a unit
transition: 0.5.57 remained active and no job remained. The authorized
`sudo -n systemctl restart` retry completed and is the deployment credited
here.

The public identity and sign-in page reported 0.5.58. Anonymous `/status`
remained 401 while the Owner's mapped socket received 200. The installed admin
binary exposed the strict `user import-local` grammar. No live profile was
changed; provider and passwd imports were exercised only in disposable tests.

The web child retained zero capabilities, `NoNewPrivs`, exactly
`AGENT_BUS_ADDR=/bus.sock` and `PWD=/`, and its limits of one CPU, 256 MiB
memory, no swap and 64 tasks.
