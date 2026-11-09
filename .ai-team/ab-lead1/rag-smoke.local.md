# #rag post-deploy smoke (from W1B 14:57, written from code, NOT run) — attach to RAG-help deploy-ready
Setup: ns=smoke-$(date +%Y%m%d); read answers with `| tail -1 | jq -r .body`.
0) free: agent-bus call '#rag@parf.us' --status | tail -1 | jq -r .body → `ok: collection verified (RAG 0.2.0)` (or `degraded: collection optimizing`); `failed:` stops.
1) free (KV; upsert refuses without prompts): agent-bus call '#rag@parf.us' '{"method":"prompts","action":"set","namespace":"'$ns'","tableName":"t","data":{"labels":{"drink":"Beverages","other":"Other topics"},"instructions":"Choose the topic."}}' → {"status":"ok"}
2) PAID (1 Voyage embeddings + 1 #ai Jev classification, both before storing): agent-bus call '#rag@parf.us' --wait=120s '{"method":"upsert","namespace":"'$ns'","tableName":"t","id":"tea","data":"Smoke test: green tea is a drink."}' | tail -1 | jq -r .body | jq -c '{id,mime:."mime-type",labels:.classification_ids,dims:(.embeddings[0]|length),n:(.embeddings|length)}' → {"id":"tea","mime":"text/plain","labels":{"drink":0.xx},"dims":1024,"n":1}
3) free: get → '{"method":"get","namespace":"'$ns'","tableName":"t","id":"tea"}' | … | jq -c '{id,data,dims:(.embeddings[0]|length)}' → {"id":"tea","data":"Smoke test: green tea is a drink.","dims":1024}
4) free (no query embedding): '{"method":"search","namespace":"'$ns'","query":"green tea","limit":5}' → {"records":[{…"id":"tea","score":>0,…}]} exactly one row
5) free: '{"method":"delete","namespace":"'$ns'","tableName":"t","id":"tea"}' → {"status":"ok"}
6) free: search again → {"records":[]}
7) free: '{"method":"prompts","action":"delete","namespace":"'$ns'","tableName":"t"}' → {"status":"ok"}
Total cost: one Voyage + one #ai call (step 2). `unknown:` / `incomplete:` = safe to repeat.
