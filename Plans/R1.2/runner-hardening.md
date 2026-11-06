# Runner hardening

📌 **TL;DR:** M.14 is proposed R1.2 work: moved from R1 to R1.1 on 2026-11-01 and on to R1.2 by the owner on 2026-11-06.
No mechanism or implementation is approved; five policy questions ([Q276–Q280](#questions)) are open.
The [acceptance](TODO.md#runner-hardening) is in the TODO.

## Runner hardening proposal

**M.14 / T-18b — plan only, owner approval required before code.** The owner named boundaries 1–3. Items 4 and 5 are **added from T-18 review, owner to confirm**. No implementation, mechanism, new configuration or wire format is approved here. The [current isolation contract](../../docs/runner/security.md#isolation-today) owns today's facts; this section defines proposed changes, not shipped protections.

| Boundary | Today → goal |
|---|---|
| 1. Runtime files | The whole read-only `/usr` exposes readable sibling bundles under `/usr/local` → narrower read-only runtime access, with sibling package contents excluded unless explicitly approved ([Q276](#questions)) |
| 2. Self-serving bus access | A self-serving child receives its own token and the shared bus socket → only the bus access approved for that managed agent, without an unrestricted transport grant merely to serve its inbox ([Q277](#questions)) |
| 3. Inbox sharing | Runner-issued consume uses the host entry's `share`; self-serving code issues its own reads → every consumption path covered by the approved policy obeys the host's intended exclusive/shared setting ([Q278](#questions)) |
| 4. Work-size checks — **added from T-18 review, owner to confirm** | A running `jsonl` child has no post-message work-size check → an approved work-size boundary also applies while that process remains alive ([Q279](#questions)) |
| 5. CPU default — **added from T-18 review, owner to confirm** | The default CPU quota is unlimited (`cpu.max` is written as unlimited) → a finite default CPU bound when the host supplies no CPU setting ([Q280](#questions)) |

Open policy choices remain in [Q276–Q280](#questions); [M.14 acceptance](TODO.md#runner-hardening) belongs to the execution plan.

## Questions

Policy choices for the [M.14 hardening proposal](#runner-hardening-proposal), separate from [current isolation](../../docs/runner/security.md#isolation-today). All await the owner; no mechanism or implementation is selected. №Q279 and №Q280 are **added from T-18 review, owner to confirm**, not part of the owner's original three boundaries.

- ❓ Q276 Which runtime files and shared public code must remain visible after narrowing `/usr`
  - Decide the supported agent/runtime compatibility set and any explicit exception for sibling public code; the goal is to exclude unrelated package contents.
  - Impact: determines the positive and negative read fixtures, without choosing how the view is constructed.

- ❓ Q277 Which bus operations may a managed self-serving agent perform
  - Decide the required inbox/reply and supporting operations, any deliberately allowed broader authority, and whether the same boundary applies over IP as over a local socket.
  - Impact: defines the allowed and denied calls; no transport, token or enforcement mechanism is chosen here.

- ❓ Q278 Where the host's `share` policy must apply
  - Decide whether it governs a self-serving agent's own consume calls as well as runner-issued reads, and what a changed setting means for active readers and in-flight messages on reload.
  - Impact: preserves the approved exclusive/shared semantics while making the scope and transitions explicit; no consumer, daemon or protocol change is selected.

- ❓ Q279 Whether to add live `jsonl` work-size control — **added from T-18 review, owner to confirm**
  - First approve or decline this addition. If approved, choose the limit semantics, maximum response delay, over-limit action and treatment of in-flight work.
  - Impact: supplies the measurable bound and expected outcome for the live-pool test, without selecting a checking or enforcement mechanism.

- ❓ Q280 Whether to give agents a finite CPU default — **added from T-18 review, owner to confirm**
  - First approve or decline this addition. If approved, choose the default aggregate CPU bound, measurement window/tolerance and permitted override/reload policy.
  - Impact: defines a falsifiable busy-agent test and compatibility requirements; no new setting or scheduling mechanism is selected.
