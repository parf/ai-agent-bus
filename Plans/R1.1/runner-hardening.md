# Runner hardening

📌 **TL;DR:** M.14 is proposed R1.1 work, moved from R1 by the owner on 2026-11-01.
No mechanism or implementation is approved; Q276–Q280 remain open.

## Runner hardening proposal

**M.14 / T-18b — plan only, owner approval required before code.** The owner named boundaries 1–3. Items 4 and 5 are **added from T-18 review, owner to confirm**. No implementation, mechanism, new configuration or wire format is approved here. The [current isolation contract](../../docs/runner/security.md#isolation-today) owns today's facts; this section defines proposed changes, not shipped protections.

| Boundary | Today → goal |
|---|---|
| 1. Runtime files | The whole read-only `/usr` exposes readable sibling bundles under `/usr/local` → narrower read-only runtime access, with sibling package contents excluded unless explicitly approved ([Q276](QUESTIONS.md#runner-hardening)) |
| 2. Self-serving bus access | A self-serving child receives its own token and the shared bus socket → only the bus access approved for that managed agent, without an unrestricted transport grant merely to serve its inbox ([Q277](QUESTIONS.md#runner-hardening)) |
| 3. Inbox sharing | Runner-issued consume uses the host entry's `share`; self-serving code issues its own reads → every consumption path covered by the approved policy obeys the host's intended exclusive/shared setting ([Q278](QUESTIONS.md#runner-hardening)) |
| 4. Work-size checks — **added from T-18 review, owner to confirm** | A running `jsonl` child has no post-message work-size check → an approved work-size boundary also applies while that process remains alive ([Q279](QUESTIONS.md#runner-hardening)) |
| 5. CPU default — **added from T-18 review, owner to confirm** | The default CPU quota is unlimited (`cpu.max` is written as unlimited) → a finite default CPU bound when the host supplies no CPU setting ([Q280](QUESTIONS.md#runner-hardening)) |

Open policy choices remain in [Q276–Q280](QUESTIONS.md#runner-hardening); [M.14 acceptance](TODO.md#runner-hardening) belongs to the execution plan.
