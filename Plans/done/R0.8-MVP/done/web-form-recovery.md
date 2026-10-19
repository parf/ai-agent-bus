# Web form recovery and keyboard entry

📌 **TL;DR:** 0.5.70 returns refused browser submissions to their form with
safe values and adds a keyboard skip target to public and signed-in pages.

## Result

Service, Channel, User and Group form refusals now render in the shared page
shell. An alert summary links back to the submitted form; line-list ACL,
Maintainer and Group values keep their line breaks. Only fields named by that
form enter temporary presentation state. Unknown submitted fields, credentials
and private configuration do not. Configuration replacement is empty after a
refusal even if presentation state is populated incorrectly.

Daemon JSON envelopes become human error text. A field receives
`aria-invalid` only for a fact the face can attribute without interpreting an
unstructured daemon message. A missing referenced ACL or Maintainer identity
returns the form while its edited record is still visible; a missing or hidden
edited record keeps the ordinary indistinguishable not-found page. The former
Maintainers-only submission for a Generic service recovers into the current
atomic classification form with fresh Personal and Allow values, so retrying
cannot clear either.

Every public and signed-in HTML page has one skip link and one focusable main
content target. Browser-input failures that never reached the daemon say that
on the shared shell instead of attributing the refusal to the daemon.

## Checks

Package and full Go tests cover safe-value retention, unknown-field exclusion,
multiline ACL/Maintainer/Group preservation, duplicate-name and duplicate-email
field attribution, human error text, missing-reference recovery, hidden-target
behavior, configuration clearing, local-action recovery and the public/signed
skip target.

The corrected targeted mutation run caught **12/12** named breaks: loss of each
skip link, all safe values or a Channel ACL; widening temporary form state;
restoring generic not-found or raw JSON; echoing configuration; reverting Group
lines; returning a bare local error; blaming the identity for a duplicate
email; and removing alert semantics.

Two mutation survivors receive no credit. Widening temporary form state did not
reach HTML because no template read the unexpected key; a direct allowlist test
now pins that boundary. Rendering a retained configuration value still produced
nothing because the allowlist had excluded it; a template test now supplies a
deliberately populated state and proves the second, independent secret boundary.
Both original hollows were reproduced before the corrected run.

Real Chromium at 375 px verified that the skip link is the first keyboard stop
on public sign-in and a signed Services page, becomes visible when focused and
moves focus to main content. The page had no page-level horizontal overflow.
The first browser attempt receives no credit because its temporary server exited
with the launching shell; the isolated rerun passed.

Fast smoke passed **488/0**. Final slow smoke passed **609/0**, including vet
and race checks. All **216** frozen source/version hashes matched afterwards;
the tracked and frozen smoke scripts both had SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.
Documentation validation checked **162 files** and **2,870 local links** with
zero errors. The first hash-verification invocation receives no credit because
it ran one directory below the manifest base and therefore looked for
nonexistent `src/src/...` paths; the corrected root-based verification checked
every entry.

## Limits

This is the recovery and keyboard-entry portion of F.13.2. It does not claim the
pending `templ` migration, owner approval of the full redesign, remaining page
journeys or installed multi-role browser acceptance. Same-origin rejection
remains a plain boundary response and avatar 404 remains an image-route response;
neither is an interactive HTML page.

## Live postflight

Commit `27073c6` was pushed, stamped as 0.5.70 with build
`parf@parf.us 2026-09-17 23:23:03` and deployed through the authorized unit
restart. Public identity, supervisor and bus process titles reported 0.5.70.
Live Chromium repeated the public and signed keyboard skip journey at 375 px
without submitting a live form or changing registry data.

The web child retained zero effective capabilities, `NoNewPrivs`, environment
`AGENT_BUS_ADDR=/bus.sock` plus `PWD=/`, and limits of one CPU, 256 MiB memory,
zero swap and 64 tasks. The first unprivileged environment/cgroup read was
denied by the installed protection and receives no measurement credit; the
authorized read-only retry supplied the recorded values. The peer AgentBus
path accepted a post-restart message.
