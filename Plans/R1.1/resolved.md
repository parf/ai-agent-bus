

<a id="Q391"></a>

- ✅ Q391 Does `#ai@parf.us` stay open to every caller
  - Context: its allow list is `*`, plus `#rag@parf.us` and `#rag/mdhouse@parf.us`, so any name on the bus can spend the realmo production key …9e4f, which has no spend limit
  - ( ) Remove `*`; keep only named callers (`#rag@parf.us`, `#rag/mdhouse@parf.us`, and any the owner adds) 🌟
  - ( ) Keep `*` ⭐
  > 💡👾 Named callers only: a paid production key should be reachable only by the agents that need it.
  > 💬 👤parf (in chat to #ab-ceo, 2026-11-06 ~14:50): "for now #ai is ACL=*"
  >
  > 💬👾 → `*` stays for now; revisit when the owner asks.

<a id="Q354"></a>

- ✅ Q354 Which image model #ai uses
  - Context: `image` is refused until an image model is configured ([Q333](DECISIONS.md)). #ai now runs on the realmo production key …9e4f (no spend limit), so every picture is paid from the site's account
  - (x) `google/gemini-3.1-flash-image` ($0.5 in / $3 out per million tokens) 🌟
  - ( ) `google/gemini-3.1-flash-lite-image` ($0.25 / $1.5), cheapest ⭐
  - ( ) `openai/gpt-5.4-image-2` ($8 / $15), strongest
  - ( ) None for now: keep `image` refused
  > 💡👾 Gemini 3.1 Flash Image: current, cheap, and good enough for agent use; switch per call if a job needs more.
  > 💬 👤parf (in the document, 2026-11-06 ~14:55): chose `google/gemini-3.1-flash-image`
  >
  > 💬👾 → set as `image_model` in `#ai@parf.us` Configuration at 14:56; `image` now uses it unless a request names another model. Every picture is paid from the realmo key.

<a id="Q359"></a>

- ✅ Q359 Turn on rerank in #rag
  - Context: RAG 0.4.0 (R.7, deployed 2026-11-06 02:40) can reorder a search's hits best first, but only when `rerank_model` is set and the caller asks `"rerank":true`. Each reranked search is one paid Voyage call (up to 64 candidate excerpts plus the query), on top of the query embedding semantic and hybrid already make. It is off today
  - (x) Leave it off until mdhouse is wired and asks for it 🌟
  - ( ) Set `rerank_model` to `rerank-2.5` (Voyage's main rerank model)
  - ( ) Set `rerank_model` to `rerank-2.5-lite` (cheaper, faster)
  > 💡👾 Off for now: nobody searches yet, and callers must opt in per search anyway; turn it on when mdhouse's #rag instance (Q353) exists.
  > 💬 👤parf (in the document, 2026-11-06 ~14:55): chose to leave it off
  >
  > 💬👾 → `rerank_model` stays unset until mdhouse is wired and asks for it.
