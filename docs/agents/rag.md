# RAG core (pending implementation)

📌 **TL;DR:** The proposed R.6 contract stores records through the Qdrant proxy,
classifies through AI and embeds through Voyage. Implementation and deployment
are pending; the manifest is being reviewed before parser code.

## Review contract

[agent.json](../../src/agents/rag/agent.json) owns the JSON-only methods, types,
bounds and examples. There are no text aliases. Upsert accepts the listed
record fields only; `classification_ids` is the output spelling of classification
ids, a label-to-probability object derived from Jev, never a caller override.
Get/list return canonical records with vectors and labels. Duplicate keys,
NaN/Infinity, numeric booleans, BOM, NUL and invalid UTF-8 are refused.

Namespace is required for every record operation. A missing, null, empty or
blank namespace, including a table without namespace, is refused. Namespace
alone is an explicit namespace-wide delete. Namespace/table are filters, not
permissions: every caller allowed to use this instance can read/delete all
its namespaces. Keep the instance owner-only unless that is intended.

A record's source is at most 32 KiB UTF-8 and yields at most eight 4 KiB chunks.
There is one Jev call per record and at most two Voyage batch calls (four
chunks per batch); any failure aborts before storage mutation. Supplied vectors
must align with the deterministic chunks. At most 8192 scalar vector values
are accepted in total. HTML extraction drops script/style, never fetches,
and limits nesting; Markdown uses source text, JSON renders canonically.

The complete canonical record, including vectors and labels, is at most
256 KiB measured even with ASCII-escaped JSON. It lives on one stable head
point, not on each chunk. A list page has at most two head records and excludes
backend vectors, leaving substantial headroom under the proxy's 1 MiB answer
cap. Boundary checks happen before writes and on backend replies.

New revision chunks are upserted with `wait=true`, then the canonical head is
published with `wait=true`, then stale IDs for that exact record are deleted
with `wait=true`. A failed new upsert leaves the prior head/record untouched;
a later failure can leave old and new chunks, never an intentional empty gap.
An unknown write says its outcome is unknown; repeating the same upsert/delete
is safe. There are no automatic retries or cross-call transaction claims.

The UUID namespace, tuple encoding and chunk-boundary algorithm become a
storage contract on first release; changing them requires a migration.
List uses Qdrant's point-ID ordering. Backend offsets remain opaque and are
checked against the expected reply shape. Clients receive bounded, expiring
server-issued tokens tied to filters and page size; tokens expire on restart.

Prompt settings are stored in this instance's KV. Hits make no KV call; set or
delete invalidates on success **and** an uncertain timeout/disconnect. The
cache never seeds itself from a write. External KV edits appear after restart.
Keyword encoding stays behind one interface until R.5 chooses it; R.6 claims
neither hybrid search nor summary generation.

## Owner provisions the collection

Run these only as the owner/admin of the configured Qdrant proxy, after R.4 is
installed. They are documentation, not commands the app runs. Substitute the
actual block names and use a separate collection per instance. This example
matches `embedding_model=voyage-3.5`, `dimensions=1024` and collection `notes`:

```sh
agent-bus call '#qdrant@node.example' 'request PUT /collections/notes {"vectors":{"size":1024,"distance":"Cosine","on_disk":false},"strict_mode_config":{"enabled":true},"metadata":{"embedding_model":"voyage-3.5"}}'
for field in namespace tableName id key1 key2 _rag_kind _rag_labels; do
  agent-bus call '#qdrant@node.example' "request PUT /collections/notes/index?wait=true {\"field_name\":\"$field\",\"field_schema\":\"keyword\"}"
done
```

The [collection API](https://api.qdrant.tech/api-reference/collections/create-collection)
supports model metadata; [payload indexes](https://qdrant.tech/documentation/search/text-search/text-filtering/)
are required when strict mode blocks unindexed filtering. At startup the app
uses read-tier collection information to check metadata, vector settings and
all required indexes. Missing or mismatched settings are refused; the app
never creates collections/indexes or escalates its grant.

Grant the app its own downstream access, preserving existing grants:

```sh
agent-bus manage '#qdrant@node.example' --add-to-set-allow '#rag@node.example(write)'
agent-bus manage '#voyage@node.example' --add-to-set-allow '#rag@node.example'
agent-bus manage '#ai@node.example' --add-to-set-allow '#rag@node.example'
```

These grants are prerequisites for live use, not changed by R.6 tests or
deployment. Provider credentials remain in the blocks' host environments.
No real data or provider call runs without owner approval.
