# Debug tracing idea

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Debug mode

An admin may switch a service into debug mode, which keeps a trace of that
service's messages. Admins only. Bodies in the trace are ciphertext — unless
the service has switched encryption off in its own config
([access § encrypted sessions](../../docs/02-access.md#trust-boundary)), the usual
pairing while developing.
