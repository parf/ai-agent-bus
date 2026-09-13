# Encryption carried from MVP

### D — the bus stops reading payloads

Pending R1 work, cut from MVP because its credential mode cannot provide secrecy
from the daemon. The [target](access.md#encrypted-sessions) remains blocked by
[key lifecycle questions](QUESTIONS.md#access-context). Original IDs are retained.

| ID | Task | Dependency |
|---|---|---|
| D.1 | Session key and encryption lifecycle | Approved protocol and key choices |
| D.2 | Delivery to a receiver absent at send time | Offline decryption decision |
| D.3 | Interoperability across faces | D.1 and D.2 |
| D.4 | Explicit development opt-out | [Encryption target](access.md#encrypted-sessions) |

## Acceptance

| Check | Change that must make it fail |
|---|---|
| Exact round trip between Go and bun endpoints | Corrupt either decoder or payload encoding |
| Encryption is default; explicit development opt-out works | Default to plaintext or ignore the explicit opt-out |
| Tampered body rejected, valid control delivered | Disable authentication-tag validation |
| Wrong key rejected | Bypass key verification |
| Replay rejected, a fresh message delivered | Disable replay tracking |
| Captured bodies cannot be decrypted with daemon-held material | Replace endpoint-only material with a daemon-held key |
| Receiver absent at send decrypts after joining | Discard or fail to recover the agreed key material |

The accepted design must specify how the last two checks are demonstrated;
round-trip success alone cannot certify secrecy.
