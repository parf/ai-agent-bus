# Retained exchange evidence

## Scope

W14 and the envelope portion of [F.13.5](../TODO.md#web-redesign): preserve
message IDs and receipt references without combining unrelated exchanges or
presenting a bounded history as proof of completion. This slice does not close
overview, activity graphs, service/channel journeys or installed browser acceptance.

## Correlation

| Evidence | Presentation |
|---|---|
| Receipt references one retained ordinary message and matches its receiver and full return route | Group with that message; keep the receipt's own ID, reference, timestamp and sender visible |
| Ordinary message matches an earlier message's return route | Keep a separate row; link possible matches without claiming a reply or completion |
| Original absent, ambiguous or inconsistent with the receipt | Keep the receipt separate with its reference and an explanation |
| Reference names a retained receipt | Say it references a receipt, rather than falsely reporting that message absent |
| Redirected destination or topic/tag | Follow the substituted return route; caller-visible evidence determines whether the original is available |
| Topic subscriber or queue worker differs from the addressed topic | Keep its receipt separate; the envelope history does not establish that worker as the original recipient |

## Checks

| Test | Evidence |
|---|---|
| `TestExchangeReceiptsKeepTheirOriginalAndRoute` | Independent participants and retries reuse labels without sharing receipts; redirected labels work; deadline equality is not late |
| `TestUnmatchedReceiptsNeverInventCompletion` | Sender, destination, label, reference and chronology mismatches remain separate; missing, ambiguous and receipt-about-receipt references remain explicit |
| `TestOrdinaryResponsesRemainMessagesWithQualifiedRouteMatches` | Replies are not inferred as completion; ambiguous candidates supply no single deadline; untagged messages get no inferred links |
| `TestExchangeRenderingPreservesEvidenceAndMissingHistory` | IDs, references, scope, offset timestamps and response qualifications survive rendering; bodies and unsafe markup do not; failed load differs from empty history |
| `TestRedirectedExchangeEvidenceIsScopedToTheViewer` | The master sees both sides; requester and third party see different incomplete evidence; an unrelated visitor sees neither |

## Mutations

Each mutation used an isolated Go overlay and failed an assertion, not compilation.
Oab independently reviewed the implementation and checked every failure report.

| Mutation | Failing test |
|---|---|
| Ignore original ID | `TestExchangeReceiptsKeepTheirOriginalAndRoute` |
| Ignore expected route | `TestUnmatchedReceiptsNeverInventCompletion` |
| Ignore redirected route | `TestExchangeReceiptsKeepTheirOriginalAndRoute` |
| Infer completion from route match | `TestOrdinaryResponsesRemainMessagesWithQualifiedRouteMatches` |
| Allow receipt fold targets to be receipts | `TestUnmatchedReceiptsNeverInventCompletion` |
| Mark a receipt without an original as completion | `TestUnmatchedReceiptsNeverInventCompletion` |
| Report a retained referenced receipt absent | `TestUnmatchedReceiptsNeverInventCompletion` |
| Mark deadline equality late | `TestExchangeReceiptsKeepTheirOriginalAndRoute` |
| Lose receipt reference in HTML | `TestExchangeRenderingPreservesEvidenceAndMissingHistory` |
| Present failed history as empty | `TestExchangeRenderingPreservesEvidenceAndMissingHistory` |

## Browser fixture

Disposable envelope fixtures, with long identities, referenced receipts, a
possible response and an absent original. Desktop tables become labelled rows
on narrow screens. At a 390-pixel viewport the document remained 390 pixels
wide and the exchange view fit its 358-pixel content area. The response link
reached the retained message; empty and unavailable fixtures remained distinct.
The initial narrow table was cramped and was replaced before acceptance.

Isolated repository verification on `2471421` plus this slice: `src/smoke.sh
--slow`, **534 passed, 0 failed**, including race and MCP/launcher checks.
The coordinated release still needs a combined run after the concurrent access
change lands, followed by installed verification. The disposable preview was
stopped and removed before the suite; it is not a shipped test or live data.

## Combined checkout verification

After access revision `08c128f`, a frozen checkout plus the `.30` release
metadata and factual unused-credential wording passed the full slow smoke:
**543 passed, 0 failed**, including vet, race and all MCP/launcher sections.
The exchange implementation was unchanged from the isolated evidence above.
Log: `tmp/q57-review/web30-combined.log`. This verifies the combined checkout,
not deployment or closure of the [access audit findings](access-review.md#findings).
