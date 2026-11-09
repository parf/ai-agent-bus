# T-49 part 2: `#rag/mdhouse@parf.us` on `#runner/parf-user@parf.us`

Audience: #cx/mdhouse-cto, who provisions `#rag/mdhouse` and its own `#qdrant/mdhouse` proxy for itself, acting as `parf`. The steps are those an **external bus user takes on its own server and runner**. Nothing here has been run. Sources: `docs/agents/{rag,qdrant}.md`, `src/agents/*/agent.json`, `docs/runner/{setup,security,configuration}.md`, `docs/agents/create-agent-prompt.md#self-serving-credential-handoff`, Q353/Q355, and the live user runner's own files (listed only).

Marks: **[ONE-ACCOUNT]** = parf does it here only because today one account is owner, operator and user. **[GAP]** = something the user runner or its owner does not have today; no workaround, no borrowing of the root `#rag`'s credentials or grants.

## Names (the same as the `#filescan/mdhouse` steps, which is registered and running on this runner)

| | |
|---|---|
| RAG | `#rag/mdhouse@parf.us`, inventory key `rag/mdhouse` |
| Its own block, same runner | `#qdrant/mdhouse@parf.us` (key `qdrant/mdhouse`) |
| Shared blocks (owner settled Q377, Q378) | `#voyage@parf.us` and `#ai@parf.us`: no `#voyage/mdhouse`, no `#ai/mdhouse`, no new keys. `#rag/mdhouse@parf.us` is already on `#ai`'s allow list (read-only check); `#voyage@parf.us` is **not visible to parf as a record** (re-checked 13:55: `ls` → `no such name`), so its allow list cannot be read: check it before the first upsert and report, do not work around it |
| Runner | `#runner/parf-user@parf.us`, parf's user-account runner, inventory `~/.local/state/agent-bus/runner/agents.json`, tokens in `~/.local/state/agent-bus/runner/tokens/` (0700, parf's: **no sudo anywhere**) |
| Sender | `#filescan/mdhouse@parf.us` on the same runner, namespace `mdhouse`, table `docs`; allowed by plain name |
| Collection | `rag_mdhouse` (one per instance) |
| Records | Owner `parf`; Maintainer `#cx/mdhouse-cto` on `#rag/mdhouse` only; nobody else holds a grant. `#mdhouse@parf.us` later: step 9 |

Roles: RAG declares none. `upsert`, `delete`, `get`, `search` need an allow entry only; `keyword_index`, `namespaces`, `drop_namespace` need the delivered `owner` or `maintainer` role. So the sender's entry is plain `'#filescan/mdhouse@parf.us'`; `(write)` exists only on `#qdrant`.

## GAPS today (read first)

| # | Gap | Why it blocks |
|---|---|---|
| G1 | **OWNER-ONLY WAIT, [Q376](../../Plans/R1.1/QUESTIONS.md): which Qdrant store.** No store of its own and no key for one. The existing store belongs to the root `#qdrant@parf.us`; its address and `QDRANT_API_KEY` sit in the root runner's private env, which parf's user runner cannot read (and borrowing it is excluded) | `#qdrant/mdhouse` needs `url` and `QDRANT_API_KEY` ([qdrant.md](../../docs/agents/qdrant.md#installing-it)). Owner decision: a new Qdrant (the doc's Podman example, `127.0.0.1:6333`, a port the live store does not use) or a remote store with a key issued to mdhouse |
| G2 | **Resolved (Q377, Q378 settled).** `#rag/mdhouse` uses the shared `#voyage@parf.us` and `#ai@parf.us`; no keys to provide and no helper to provision | the shared `#ai` is production: real application use only, never tests. Open check: `#voyage@parf.us` is not visible to parf (`ls` → `no such name`), so `#rag/mdhouse`'s grant on it is unconfirmed |
| G4 | **No admin on the Qdrant proxy.** Collection and index creation need role `admin`; a caller admitted with no role gets `write` | the owner term `@owner(admin)` is added to the new proxy only, for step 4, then removed |
| G5 | **Package source for the user runner.** Its `~/.local/share/agent-bus/agent.d/` holds only `tmux-sessions` and the shared `agent.d` is invisible to it | `rag` and `qdrant` must be bundled-enabled (if `status` shows them `available`) or placed in that directory. Step 1 decides; if neither works it is a gap, not a copy from the root runner's `agent.d` |
| G6 | **Bus socket for a self-serving agent in the user runner is unverified** for `rag` and filescan (security.md refuses an account socket or an unverified listener) | the only agent proven there, `tmux-sessions`, is a per-message `args` agent, not self-serving; step 7 shows the answer, a refusal is reported, not worked around |
| G7 | **Network inside the sandbox.** `qdrant` declares `needs.network`, so it runs in the host network namespace and reach a loopback store or the internet; `rag` declares none and reaches only the bus. No gap for the design, but a remote Qdrant is reachable only if host policy permits | none expected for a loopback store |

Steps 2–6 and 8 cannot complete before the owner answers Q376 (G1), the only owner wait left; do not guess a store or a key. Steps 1, 7 and 9 can be done meanwhile and show the refusals by name.

## Steps

All commands as parf with `env -u AGENT_BUS_SESSION_FILE`. `R='#runner/parf-user@parf.us'`.

### 1. Package on the runner (user's own runner) [G5]

```sh
agent-bus call "$R" status | grep -E '"(key|status|agent_dir)"' | grep -E 'rag|qdrant'
```

- A row `available` (bundled) means enable it as below, and no base entry is needed. No row (`create` is refused for a package not found): the runner does not see the package; report G5.
- Inventory, as for any user: the two entries, keeping the existing `tmux-sessions` ones. Use the runner's calls, which write `agents.json`:

```sh
for k in rag/mdhouse qdrant/mdhouse; do agent-bus call "$R" "create $k"; done
```

Resulting entries (sandbox stays **on**: neither needs the home or tmux):

```json
"rag/mdhouse":    {"enabled": true},
"qdrant/mdhouse": {"enabled": true}
```

If a base entry (`rag`, `qdrant`) is also enabled for a bundled package, disable it with `{"enabled": false}`, as the existing `tmux-sessions` entry does, so only the instance runs. No `roots` (no folder), no `limits` unless wanted. Default names `#<package>/mdhouse@parf.us` are the wanted ones.

### 2. The store key into the runner's private env file [G1]

Only the store key; no provider key is needed (Voyage and `#ai` are the shared records). The file is parf's, 0600, `~/.local/state/agent-bus/runner/qdrant/mdhouse/env` (the `create` made it empty):

```
qdrant/mdhouse/env : QDRANT_API_KEY=<the store key issued for mdhouse>
```

Write it with the runner's `write` call or by hand with `umask 077`; never in a command argument, a record Configuration, a transcript or a commit. **[ONE-ACCOUNT]** this file is readable by parf's AI sessions (setup.md: "her secrets are readable by her own processes"); a separate user's server would not have that. RAG's own env stays empty.

### 3. Records, Configuration, token handoff

**Order:** `qdrant/mdhouse` exists only after step 1's `create` has applied (the runner registers it). Before the `config` and `manage` calls on it (here and in steps 4-5), `agent-bus ls '#qdrant/mdhouse@parf.us'` must answer.

Which records the runner makes: `qdrant/mdhouse` is a per-message pool, so the runner registers it itself (Owner `parf`). `rag/mdhouse` serves itself, so **the user registers it and hands its token once**:

```sh
for n in '#rag/mdhouse@parf.us'; do
  agent-bus register "$n" --kind agent --allow '@owner'
  f="$HOME/.local/state/agent-bus/runner/tokens/$(printf '%s' "$n" | sed 's/#/%23/; s#/#%2F#')"
  install -m 600 /dev/null "$f"
  agent-bus token "$n" | tee "$f" >/dev/null
done
```

File names follow the live ones (`%23tmux-sessions%2Fab-dvp@parf.us`). **[ONE-ACCOUNT]** parf registers and holds the token because it is both the user and the Owner; an external user does exactly this with its own account. Then the Configuration, each by its record's Owner:

```sh
agent-bus config '#qdrant/mdhouse@parf.us' '{"url":"http://127.0.0.1:6333"}'            # G1: the store's real address
agent-bus config '#rag/mdhouse@parf.us'    '{"qdrant":"#qdrant/mdhouse@parf.us","voyage":"#voyage@parf.us","ai":"#ai@parf.us","collection":"rag_mdhouse","embedding_model":"voyage-3.5","dimensions":1024,"decision_model":"typesafe/jev-1.14","keyword_search":"builtin","classify":false}'
```

The shared blocks keep their own Configuration. `classify` stays off (paid per request); `rerank_model` unset.

### 4. Collection `rag_mdhouse` by the store's operator [G1, G4]

The app never creates storage. Grant the owner `admin` on mdhouse's own proxy only, create, then remove:

```sh
agent-bus manage '#qdrant/mdhouse@parf.us' --add-to-set-allow '@owner(admin)'
Q='#qdrant/mdhouse@parf.us'
agent-bus call "$Q" 'request PUT /collections/rag_mdhouse {"vectors":{"size":1024,"distance":"Cosine","on_disk":false},"sparse_vectors":{"text":{"modifier":"idf"}},"strict_mode_config":{"enabled":true},"metadata":{"embedding_model":"voyage-3.5"}}'
for field in namespace tableName id key1 key2 _rag_kind _rag_labels _rag_rev; do
  agent-bus call "$Q" "request PUT /collections/rag_mdhouse/index?wait=true {\"field_name\":\"$field\",\"field_schema\":\"keyword\"}"
done
agent-bus manage "$Q" --remove-allow '@owner(admin)'
```

Creating it with the `idf` sparse vector means step 8 adds only the prefix index. **[ONE-ACCOUNT]** parf is the operator; an external user is its own.

### 5. Block grants

```sh
agent-bus manage '#qdrant/mdhouse@parf.us' --add-to-set-allow '#rag/mdhouse@parf.us(write)'
```

`#ai@parf.us` already admits `#rag/mdhouse@parf.us`; on `#voyage@parf.us` the same grant is `agent-bus manage '#voyage@parf.us' --add-to-set-allow '#rag/mdhouse@parf.us'`, only if the record is visible to parf and the entry is missing (today it is not visible: report it). Nothing is granted on the root `#qdrant` or on `#rag@parf.us`.

### 6. Maintainer and callers

**Order:** the allow entry for `#filescan/mdhouse@parf.us` is refused unless that record exists: run it after `#filescan/mdhouse@parf.us` is registered (it already is, and live: 194 files tracked, sandbox bwrap, no inotify or socket gap). The `#filescan/mdhouse` steps contain the same command; run it once.

```sh
agent-bus manage '#rag/mdhouse@parf.us' --add-to-set-maintainers '#cx/mdhouse-cto'
agent-bus manage '#rag/mdhouse@parf.us' --add-to-set-allow '#filescan/mdhouse@parf.us'
```

- `#cx/mdhouse-cto` (Maintainer): Configuration, access list, `keyword_index`, `namespaces`, `drop_namespace`. No other session gets a grant.
- Access list afterwards: `@owner`, `#filescan/mdhouse@parf.us`. If the Maintainer's own call is refused as `acl`, add that one name.
- parf stays Owner because a record can only be owned by a User **[ONE-ACCOUNT]**; on a separate server the user is that Owner.

### 7. Reload and look

```sh
agent-bus call "$R" reload
agent-bus call "$R" status      # the two rows
```

Expected: `rag/mdhouse` `serves:itself`, 1/1 process; `qdrant/mdhouse` a pool; sandbox on; no refusal text; `tmux-sessions` rows unchanged. A refused row names its reason: `needs QDRANT_API_KEY`, a missing token, G6.

### 8. Keyword and prefix index (Owner, once, before ingestion)

```sh
agent-bus call '#rag/mdhouse@parf.us' --wait=120s '{"method":"keyword_index","namespace":"mdhouse"}'
```

Adds the `text` prefix-tokenizer index `complete` needs (the `idf` vector is already there). Expected `{"status":"ok","indexed":0}`; `incomplete` → repeat; a proxy refusal names the exact `request PUT …` command to run with admin on `#qdrant/mdhouse`. No provider call, no ingestion.

### 9. Expected answers, and adding `#mdhouse@parf.us` later

```sh
agent-bus call '#rag/mdhouse@parf.us' --status
agent-bus help '#rag/mdhouse@parf.us'
```

- `status` runs `verify()` against the collection and answers one line: **`ok: collection verified (RAG 0.4.1)`** (`degraded: collection optimizing (RAG 0.4.1)` for a moment after creation). Failures say why: `failed: collection verification failed` (wrong size or model metadata, a missing keyword payload index, or a grant missing on `#qdrant/mdhouse`), `failed: …` naming a missing Configuration key, or a block refusing RAG's own grant (an alert in its warnings).
- It needs `#qdrant/mdhouse` running, so it also proves G1 is closed; `#voyage` and `#ai` are not called by `status`, so a missing grant on `#voyage@parf.us` shows only at the first upsert. `{"method":"capabilities"}` is free and safe.
- `agent-bus ls '#rag/mdhouse@parf.us'`: `allow` = `@owner`, `#filescan/mdhouse@parf.us`; `maintainers` = `#cx/mdhouse-cto`; `readers: 1`.
- Add `#mdhouse@parf.us` when it exists (it is not registered today, so it is left out):

```sh
agent-bus manage '#rag/mdhouse@parf.us' --add-to-set-allow '#mdhouse@parf.us'
agent-bus manage '#rag/mdhouse@parf.us' --add-to-set-maintainers '#mdhouse@parf.us'   # only if the owner wants it to run keyword_index itself
```

## Not done here

No ingestion, no `upsert`, no `search`, no call to the root `#rag`, `#voyage`, `#ai` or `#qdrant`, no `sudo`, no provider spend. The shared `#ai` is production: real application use only, never tests. The first paid embed and classification happen only when mdhouse starts the `as rag` subscription on `#filescan/mdhouse`.

## Rollback

`delete` `rag/mdhouse` and `qdrant/mdhouse` with the runner call (keeps tokens and records), `agent-bus unregister` the two names, remove the token file and the env file, and `request DELETE /collections/rag_mdhouse` on `#qdrant/mdhouse` with `@owner(admin)` added again. The root runner and its records are never touched.
