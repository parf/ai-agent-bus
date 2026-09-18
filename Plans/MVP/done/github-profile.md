# GitHub profile metadata and local photos

📌 **TL;DR:** GitHub login changes and explicit refresh atomically retain public provider facts and a bounded local thumbnail; pages never contact or hotlink the provider.

## Scope

Version 0.5.71 completes the GitHub-profile and local-photo data dependency for
the remaining User journey. It does not close the complete F.13.4 mixed-directory,
role or installed-browser acceptance.

The trusted adapter reads the public profile only when `github_user` is created
or changed, during GitHub key enrolment, or on the explicit refresh operation.
An unrelated profile edit performs no provider request. A required profile
failure commits nothing. Clearing the login removes provider metadata and photo
but retains Person name and Email after their fill-blank import.

Provider `name` fills blank Person name. Public email fills blank Email only
when its normalized value is not already owned; invalid or duplicate optional
email does not refuse enrolment or an otherwise valid update. Company,
location, Twitter/X, avatar source, Gravatar ID and fetch time are provider
facts replaced by each complete successful profile response.

The adapter permits photo redirects only among its approved GitHub-avatar and
Gravatar hosts, reads at most 2 MiB, accepts at most 4096 pixels per side and
16 Mi pixels, then center-crops and re-encodes a 96-pixel PNG. It tries the
GitHub avatar before Gravatar. Optional fetch/decode failure retains the last
local thumbnail; an answer with neither source clears it. Only the local PNG or
generated initials reach HTML. The User list carries all visible thumbnails in
its one `/users` answer, and service detail uses one caller-visible directory
answer for its owner's photo; a hidden owner yields no image or profile link.

## Checks

The focused Go suites cover profile setup/change/refresh, fill-blank imports,
duplicate-email soft failure, required-profile atomic failure, clearing,
optional-photo retention, source removal, stale concurrent refresh, snapshot
round trip, strict API input, missing-versus-measured fetch timestamps, local
browser rendering, no page-time provider request and owner-photo visibility.

Adapter checks exercise GitHub-avatar preference, Gravatar fallback, an
approved-host redirect refusal with a reachable contrary endpoint, byte and
dimension maxima plus their first rejected values, non-image refusal and local
PNG normalization.

Real Chromium at 375 px rendered current source against the live read-only
daemon. It found local initials in the User list, the User title and the
caller-visible service-owner line, followed the owner link, found no external
image source and found no page overflow. The populated provider-metadata,
local-PNG and hidden-owner states use the isolated browser fixture because the
live directory currently publishes none of those facts.

The first mutation run receives no final credit. Its oversized-body fixture
was non-image data, so decode rejection masked removal of the byte limit; two
web mutants also stopped at compilation before reaching their rendering
assertions. The corrected oversized fixture is a valid PNG with trailing bytes,
and the web mutants compile and fail on rendered behavior. A later claim sweep
found and fixed the change-login path losing the prior photo when optional
image import failed. The final **15/15** mutations catch caller-owned provider
bytes, unrelated refetch, partial required-profile failure, duplicate optional
email, both failed-photo preservation paths, stale refresh, wrong clearing,
Person-name overwrite, redirect broadening, byte or dimension limit removal,
provider hotlinking, missing service-owner presentation and fabricated fetch
times.

Fast smoke passes **488/0** and the complete Go test suite passes.
Documentation validation checks **164 files** and **2,874 local links** with
zero errors. Final slow smoke passes **609/0**, including vet and race checks,
with all **248** frozen source, test and script hashes unchanged. The tracked
and frozen smoke scripts both have SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.

## Boundaries

Remote provider URLs remain data in the daemon answer and are never used as an
HTML image source. The normalized thumbnail is base64-embedded from the
caller-visible directory answer under the page's local-data CSP; no public
profile endpoint was added. GitHub profile availability is still required for
changing the GitHub-login field, while photo availability is optional. The
existing key-possession proof remains the authentication fact. The
[snapshot upgrade note](../../../docs/09-setup.md#github-profile-snapshot-upgrade)
records how an older daemon treats the new optional fields.
