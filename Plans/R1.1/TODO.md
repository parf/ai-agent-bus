# TODO R1.1

## Objective

Prepare the [tools stage](README.md#scope). Not started; R1 prerequisites are not built.

## Next step

Resolve [catalogue and scope questions](QUESTIONS.md#open-questions), then select the first tools with the owner.

## Dependencies

| Candidate | Prerequisite | Acceptance to carry into its implementation task |
|---|---|---|
| Ordinary catalogue services | Managed runner, ACL and configuration contracts | Deploy and call a service using the published ordinary service interface; remove a required interface capability and the exercise fails. Review the daemon diff to detect hidden special handling |
| Service credentials | R1 token scoping | A narrowed credential calls its target and is refused elsewhere; disable target enforcement and the negative check fails |
| Record expiry | Lifetime and peer clock decisions | An inactive ephemeral record expires while a served record and a kept control survive; removing expiry or the served-record guard fails the appropriate check |
| Container image | Package and sandbox decision | A clean image boots and calls a bundled service; omit a required installed artifact and the call fails |
| Contact routing | MVP profiles, contact visibility and identity linkage decisions | Reach the configured contact using its selected route; deny a caller outside its visibility. Bypass visibility and the negative check fails |

The no-daemon-change criterion applies to catalogue entries; whether the whole stage can meet it is an [open scope conflict](QUESTIONS.md#open-questions).
