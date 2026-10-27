# AI proxy

Runner-executed Python entry; see the [user guide](../../../docs/agents/ai.md).
Each JSON stdin envelope produces one stdout answer. No bus client, local
history or provider-specific library. The runner generates all help asks from
`agent.json` and `env.dist`. The declared `ask` method accepts prompt text or
a chat request body; bare prompts and chat requests use the same handler.

## Transport and credentials

Only `OPENROUTER_API_KEY` in the host's `runner/ai/env` is used. Never place
it in bus Secret or Configuration. The `# required` declaration in `env.dist`
uses the runner's standard missing-key report: `needs OPENROUTER_API_KEY`.
The entry also exits 78 if invoked directly without a key.


HTTP(S) uses the configured root plus `/chat/completions`, no redirects and
no automatic retries. Request and response bodies are bounded at 1 MiB.
A whole-request alarm expires 100 ms before the delivered deadline, capped
at 60 seconds when the caller supplies no longer deadline; the runner also
enforces its process deadline. Intentional missing-key exit is 78; provider
and validation failures are normal `error:` answers.

JSON object requests retain all fields except that an absent model is filled
from Configuration. `stream:true` is refused. Successful JSON responses retain
their data except any string/key containing the provider credential, which is
masked, including JSON-escaped spellings. Plain text returns only assistant
content. Logs contain only model, byte counts and status; no exception strings,
prompts, provider error messages or answers.

## Package and checks

`package.sh <agent.d/ai>` installs `ai.py`, manifest, env descriptions and the
PolyForm license. The release exporter and setup manifest include the same
runtime files. No dependencies beyond Python 3's standard library.

`python3 -B -m unittest discover -s src/agents/ai -v` uses a private fake
provider, including an outside-checkout package execution. The `agent_ai`
smoke shard runs these checks in fast and normal. The credential mutant disables
`redact`; its owning secrecy test must fail on an assertion.
