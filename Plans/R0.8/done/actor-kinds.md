# Actor kinds

📌 **TL;DR:** 0.7.5 makes the name say the kind — an Agent's begins with `#` —
and makes every record a User's. Personal becomes valid on every kind and
scoped to the Owner's cohort, a User hears only from Agents that admit it, and
a start ignores and reports what this version could not have written instead of
deleting it. Evidence for [K.4, K.8 and K.20](../0.7.0-TODO.md#storage-and-identity).

## Result

| Area | Built |
|---|---|
| Names | `kind` is `agent` exactly when the name begins with `#`; `ParseName` strips and re-renders it; URLs carry `%23`; the CLI's `--agent X` adds one `#` |
| Ownership | the stored owner is the User the caller is or acts for; an Agent creating a record gains no authority over it; transfer requires a User; a User cannot take a `#` name |
| User records | every User has exactly one 👤 record, always Personal, never transferred or removed |
| Personal | valid on every kind; `allow` and `maintainers` name only the Owner, the Owner's agents, `@owner` and `@agent` |
| Runtime terms | `@owner` and `@agent` in both lists; `@agent` only on an 👾; neither may be a stored group |
| Delivery | a User takes a direct send from an Agent whose ACL admits it and from nobody else; Deliver-To names agents and groups; fan-out skips a recipient whose Owner is suspended |
| Private values | `config` and `secret` on 👾 and 📡; an agent's secret is read by that agent alone |
| Loading | incorrect records, Users without a user record and queues without a proper record are ignored and reported to the error log, and stay in the database; the old orphan sweep is gone |
| Freed names | a record born under a freed name drops any stored queue of that name in its first commit |
| Faces | web hides Personal records of any kind from the main pages; launchers register Personal session agents; bare-MCP `ab_rename` is refused pending [Q105](../QUESTIONS.md#open-questions) |

## Checks

| Check | Mutation that fails it |
|---|---|
| `core` TestOnlyAnAgentsNameBeginsWithHash | dropping either direction of the `#` kind check |
| `core` TestAUserHearsOnlyFromAgentsThatAdmitIt | letting any agent reach a User; letting a User send to a User |
| `core` TestADeliverToListCannotNameAUser | accepting a User as a recipient |
| `core` TestPersonalAdmitsTheOwnersCohortAndNothingWider and three more | skipping the cohort check |
| `core` TestANameFreedByAnIgnoredRecordInheritsNoStoredQueue | not dropping a born record's stored queue |
| `core` TestTheTwoSweepsAgreeAboutOneName, TestEveryPathThatStoresARecordAsksTheSameShapeQuestion | loading every record; accepting a non-User owner |
| `core` TestAnAgentThatCreatesARecordOwnsNothingAndManagesNothing | storing the creating Agent as owner |
| `core` TestRestoreKeepsAServiceAndAnAgentSecret | letting an agent's ACL read its secret |
| `core` TestAPublicationADisabledRecipientCannotTakeCountsAsItsDrop | fan-out asking only the recipient's own user state |
| `smoke.sh` "a start ignores and reports the records it could not have written" | the load, credential-sweep and freed-queue rules end to end |

`@agent` resolving to nobody survives by construction: the record's own Agent
already manages its record ([authority rules](../../../docs/constitution.md#authority-rules)),
so the term is an alias and the mutant is equivalent.
