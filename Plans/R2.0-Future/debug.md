# Debug tracing idea

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Debug mode

An admin may switch an agent into debug mode, which keeps a trace of that
agent's messages. Admins only. Bodies in the trace are ciphertext — unless
the agent has switched encryption off in its own config
([access § encrypted sessions](../../docs/02-access.md#trust-boundary)), the usual
pairing while developing.
