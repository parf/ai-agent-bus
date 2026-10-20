# Telnyx SMS live verification

Completed 2026-10-20 in `8a86a0b9` (RD112 and provider rename),
`43483348` (CLI examples) and `0ca3c599` (sandbox endpoint guard).
Current usage lives in the [SMS guide](../../../docs/agents/sms.md).

- Normal gates: 1181/0 in 46.521 s; final SMS-only landing 13/0.
- Owning mutants failed on assertions for required-marker parsing, agent-wide
  environment, default flags, native TS/Python declarations, real-host refusal,
  and the sandbox's absent account database. Fake-provider checks use loopback
  only and reject every external test connection.
- The final external `/rd` wrapper was checked with the real CLI against a fake
  bus: exactly one JSON body, one answer; dry-run makes no call. `call` takes
  JSON as one argument; a dash is literal text.
- Live `#sms-telnyx@parf.us` has tag `sms`, sandbox on, ACL `*`, private host
  credential and public sender/alias Configuration. The old SMS record,
  package, host directory name and token were retired.
- Exactly two real provider POSTs, both accepted: direct
  `4031a15b-4d54-458a-b342-cc087e8588a7`, wrapper
  `4031a15b-4d57-4154-adac-f4651e615627`. No retries. Two earlier bus probes
  were rejected before provider transport. Acceptance does not prove handset
  delivery. A root-only attempt ledger prevents accidental repetition.
- Runner journal scan found neither API key. The tracked-file recipient guard
  and touched-link checks passed. Post-deploy selftest 6/6.
- AI required-key handling now works with the agent-wide host file and ACL `*`.
  With model Configuration unset, text answers the model-needed error before
  any provider request.
