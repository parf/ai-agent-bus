# Default GitHub profile adapter

📌 **TL;DR:** 0.5.76 makes optional public GitHub profile metadata available
without changing which identity realms require key-possession enrolment.

## Result

The daemon now installs its trusted GitHub public-profile adapter on every
run. Setting or changing a GitHub login and **Refresh GitHub profile** can
therefore fetch public metadata on an ordinary installation; operators do not
need to turn the `github` realm into a directory-backed enrolment realm.

The two choices remain independent. `-directory realm=github` still means
that names in `realm` must prove possession of a key published by GitHub.
Merely reading optional profile metadata backs no realm and changes no
registration or authentication rule. A configured GitHub-backed realm reuses
the same constrained adapter.

Provider failure keeps the 0.5.75 behavior: a valid unique login is retained
with provider fields absent, while an explicit refresh reports failure and
changes nothing. Browser pages still make no provider request and use only the
daemon-normalized local thumbnail or initials.

## Checks

Focused daemon tests attach the default profile adapter with no configured
realm, import a profile through an administrative login update, and prove a
GitHub-realm challenge remains unbacked without `-directory`. A second check
configures a differently named realm with GitHub and proves enrolment reuses
the same adapter. Existing core, API and web profile tests retain provider
failure, uniqueness, refresh atomicity, local-photo and no-hotlink behavior.

The corrected mutation set catches **4/4** changes: removing the default
profile attachment, making the adapter implicitly back the `github` realm,
replacing an explicitly GitHub-backed realm with the file adapter, and
silently accepting a malformed directory setting. The first mutation command
was run from the wrong directory and changed no source; a later mutant removed
a redundant call-count assertion without changing behavior. Both runs are
retained and receive no credit.

Fast smoke passes **489/0**. Documentation validation checks **173 files** and
**2,910 local links** with zero errors. Final slow smoke passes **610/0**,
including vet, race and launcher/MCP checks, with all **401** frozen tracked
and candidate files unchanged. This measured result was filled into the
evidence after the immutable run.

## Boundaries

This changes adapter availability, not trust. Public profile facts remain
optional metadata; GitHub key possession remains a separate enrolment proof.
No public endpoint, wire field, browser fetch or automatic enrolment realm was
added.

## Live postflight

Commit `3f38751` was built and deployed as 0.5.76 with build
`parf@parf.us 2026-09-18 14:07:20`. Explicitly refreshing the already-saved
`parf` login for `parf@parf` fetched the real public profile, a fetch timestamp
and a normalized local GitHub photo; the User page no longer shows the
unfetched warning or lookup error.

A public enrolment challenge for a fresh `@github` name still returns 403 with
`nothing backs the realm "github"`, proving that profile availability did not
enable GitHub-backed enrolment. The peer session reconnected through the
restart.
