# MVP questions

📌 **TL;DR:** Settle Personal ACL group entries; other recent choices are settled or deferred.

## Open questions

Settled and deferred choices are in the [decision index](../../docs/decisions.md#settled); their IDs remain reserved.

## Personal ACL group entries

**Q71.** Does [Personal’s service-only assignment rule](../../docs/03-services-and-topics.md#personal-and-shared)
exclude group entries, or may an ACL name a group of services? If groups are
allowed, what happens when a user joins one: must the service first become
non-Personal, or does that membership change remove the tag?

This is tag validity, not a separate access policy. Administrator control of
ordinary group membership stays settled; it does not answer which ACL
assignments may coexist with Personal.
