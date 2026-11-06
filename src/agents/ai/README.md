# AI proxy

📌 **TL;DR:** Kept Python processes that serve `#ai`'s own inbox; see the [user guide](../../../docs/agents/ai.md).
Two processes (`serves: itself`, `processes: 2`) each read the shared inbox in the
main thread through the bundled Python client and answer one message at a time.
No local history or provider-specific library. Help asks are answered in-process
from `agent.json` and `env.dist`. The declared `ask` method accepts prompt text or
a chat request body; bare prompts and chat requests use the same handler.

## Transport and credentials

Only `OPENROUTER_API_KEY` in the host's `runner/ai/env` is used. Never place
it in bus Secret or Configuration. The runner does not enforce `env.dist` for a
self-serving agent, so a process without the key logs `OPENROUTER_API_KEY is not
set` and exits 78 before it connects to the bus. Configuration is read once at
start; SIGHUP (a Configuration change) and SIGTERM end the loop after the
current answer, and the runner starts a new process.

HTTP(S) uses the configured root plus `/chat/completions`, `/models`,
`/models/<id>/endpoints`, `/credits`, `/key`, `/embeddings` or `/images`, no
redirects and no automatic retries. A 404 answers `provider has no <method>`; a
401 or 403 names the refused key and a 402 the credit. `balance`, `key` and
`image` need the owner or maintainer role from the daemon-set `roles` of a
message sent to `#ai` itself, before any call; `key` answers only its limit
fields. `image` refuses without a model before any call, checks `#ai`'s bigfile
room (100 keys, 256 MiB, a 32 MiB reserve per requested picture) before the paid
call and again with the decoded sizes, then puts each picture under a random
`image-<16 hex>.<ext>` name with a 24-hour ttl. `embed` keeps each model's
vector size, at most 256 models, in `$AGENT_BUS_WORK/embed-dims.json`, and sizes
its cap from the runner's answer envelope (`answerAs`), an upper bound on the
client's reply request, which the daemon reads up to 1 MiB. Request and response
bodies are bounded at 1 MiB, except an image answer ([image limits](../../../docs/agents/ai.md#images)).
The image cap budgets parsing and decoding for both kept processes sharing the
memory limit. Each provider request has an alarm 100 ms before the delivered
deadline, capped at 60 seconds; image requests leave a further 15 seconds for
storage and the reply. The loop runs in the main thread so SIGALRM works. Provider, bigfile and validation failures are normal `error:`
answers.

JSON object requests retain all fields except that an absent model is filled
from Configuration. `stream:true` is refused. Successful JSON responses retain
their data except any string/key containing the provider credential, which is
masked, including JSON-escaped spellings. Plain text returns only assistant
content. Logs contain only model, byte counts and status; no exception strings,
prompts, provider error messages or answers.

## Contributor package and checks

`package.sh <agent.d/ai>` installs `ai.py`, manifest, env descriptions, the
shared helpers, the Python client with its MIT license, and the PolyForm
license. The release exporter and setup manifest include the same runtime
files. No dependencies beyond Python 3's standard library.

`python3 -B -m unittest discover -s src/agents/ai -v` uses a private fake
provider and a fake bus: every call goes through the kept entry (`handle`) as a
real client `Message`, the loop itself (`serve`) runs over queued messages and
ends on SIGTERM or SIGHUP, and an outside-checkout package run reaches the bus
step. The `agent_ai` smoke shard runs these checks in fast and normal. The
credential mutant disables `redact`; its owning secrecy test must fail on an
assertion. Removing the role gate fails the balance/key role tests; reading
roles without the `original_to` check fails the daemon-set-roles test; removing
the room check before the paid call fails the full-record test.
