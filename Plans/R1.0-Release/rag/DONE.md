# DONE: RAG service

Completed steps of the [RAG TODO](TODO.md#steps), newest first.

## R.1 Voyage proxy

- **Landed:** shared bus 0.9.123, `ab691607`, 2026-10-27. [Current calls and safety rules](../../../docs/agents/voyage.md#calls).
- **Built:** separate Python runner package; `embed` plus exact-body `/v1/` POST pass-through; fixed verified Voyage origin; required host env key; shared bounded HTTP/masking/deadline helper with AI; runtime/package/setup inventories include all shared files.
- **Checks:** final normal **1207/0 in 45.944s**, under the smoke lock; AI **10/0**, Voyage **8/0**, three exported-runtime/setup owning checks green; changed-doc links **0 bad**. A 12-case comparison with the original AI code found identical request bytes, path/header, response/code and log bytes.
- **Mutants:** **14 assertion-red**, owning tests only: Voyage auth, key leak, body rewrite, path escape, answer cap, ask interception, required-key marker, packaged helper and model-ID/log bound; AI auth, key leak, body rewrite, answer cap and whole-request deadline. All restored before the final gate.
- **Review:** `#claude/ab2-review` approved a read-only static review (did not run tests). Every low note addressed: bounded model IDs prevent prompt-sized log entries; docs explain leading-`{` text; SMS maps its shared metadata dependency. Voyage timing assertions allow scheduler slack. No open finding.
- **Deployment:** only AI and Voyage packages replaced; Voyage enabled in the existing runner inventory after green, other entries and AI's owner/`*` ACL preserved. Runner reload, Voyage live help and sandbox-on idle/on-demand state verified; **check-deploy 6/6**. Installed support VERSION is .123; the unchanged .121 runner stamps intercepted help with its own version. No daemon restart or git-install was needed.
- **Provider safety:** no real Voyage/OpenRouter/SMS/mail request. The installed Voyage key file was never read, printed, copied or changed by this work. The manager/owner's first live call remains theirs.
- **Git:** local commits only; no push.

<details><summary>Approved implementation plan (history)</summary>

- ⏳ R.1 `#voyage`: a transparent Voyage proxy; this is [B.19](../TODO.md#bundled-agents)
  - Acceptance and mutants: B.19.
  - **Plan for manager review (2026-10-27; no code yet):**
    - **Shape:** a separate Python runner package, pooled and sandboxed like `#ai`, with the normal per-name ACL (initially `@owner`). `embed` accepts text, optional model and input type; the default model is `voyage-3.5`. Generic pass-through preserves the request path and body bytes, adds the key, and accepts only paths within `/v1/` on the fixed, certificate-verified `api.voyageai.com` origin. Configuration holds only the default model. Keep `#ai` behavior unchanged while sharing only its small HTTP, masking and deadline helpers, without a new dependency or proxy framework.
    - **Credentials and bounds:** `VOYAGE_API_KEY= # required` in `env.dist`; the runner supplies the existing host `voyage/env` layer. Never read, print, copy or set the installed key file during implementation. No bus Secret or Configuration credential, redirects, retries or streaming; retain `#ai` request/response caps and the earlier message deadline. Provider errors and 429 are normal masked answers. Logs contain only model, byte counts and status.
    - **Files:** add `src/agents/voyage/` (entry point, manifest, env declaration, package script, README and fake-provider tests); extract the common helper into `src/agents/shared/provider_http.py` and include it in both proxy packages. Update `src/agents/package.sh`, setup's package inventory and delivery/runtime fixtures, `src/acceptance/agent-runtime.py`, the smoke shard/map and shared metadata checks. Add `docs/agents/voyage.md` and the agents index row. Include every new runtime helper in exported bundles; verify execution outside the checkout.
    - **Tests and mutants:** loopback fake Voyage only, with external connections blocked. Pin embedding output/defaults, exact pass-through bytes/path/Authorization, errors and 429, missing key, escaped key echoes, path traversal, size/time bounds, one attempt and metadata asks doing no provider work. Mutants: omit Authorization, leak the key in an error, rewrite the outgoing body, escape `/v1/` or remove a bound; each runs only its owning check. Run the existing AI suite to prove shared-code compatibility.
    - **Landing:** after review, take the smoke lock for edits/mutants and the normal gate, bump and changelog, then deploy only the proxy packages changed by shared code. Enable only `voyage` in the existing runner inventory and reload, preserving other entries and grants; verify help, configured state and `check-deploy`. Do not send an embedding or any real provider request: the manager and owner make the first live call. Request a read-only review from `#claude/ab2-review`, fix or answer every finding, and record the version, commit, gate numbers and review result in DONE. Mark R.1 done in TODO and report to the manager before submitting the R.2 plan. All commits stay local; never push.

</details>
