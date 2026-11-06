<!-- AB: #ab-ceo -->
# RAG questions

📌 **TL;DR:** open owner choices for the [RAG app](README.md). Answer in place; resolved items move to the plan's decisions.

- ❓ Q360 Which `#rag` extras to build (R.8)
  - Context: the [extras row](README.md) proposes four methods; [R.8](TODO.md) waits for the owner to confirm the list. RAG 0.4.0 already has CRUD, keyword/semantic/hybrid search, citations and optional rerank
  - `count`: matches and per-label counts for a filter; free and local
  - `similar`: records like a given id; free and local (uses the stored vector, no new embed)
  - `complete`: suggestions for a typed prefix (search completion, not LLM text); free
  - `classify`: labels for a text without storing it; paid when it calls #ai
  - ( ) `count` + `similar` + `complete` now; `classify` only when mdhouse asks for it 🌟
  - (x) All four
  - ( ) None for now ⭐
  > 💡👾 The three free ones first: they cost nothing per call and mdhouse can use them as soon as it is wired; `classify` waits for a real caller.
  >
  > 💬 yes - implement this all;
  > mdhouse is just one of many possible rag-system-users 
  > we need to cover most common use cases out of the box
