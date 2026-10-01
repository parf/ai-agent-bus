# Method metadata

Status: proposed, not built. The MVP carries a service's method information in
its [description](../../docs/03-records-agent.md#agent-templates) —
one sentence, written by whoever registered it, and the only thing a listing or
the MCP catalog can show. That is enough to recognise a service and not enough
to call one: a caller cannot learn a verb's arguments from it, nothing marks the
verbs that destroy something, and a face has nothing to generate documentation
from.

R1 replaces it with something **stated**, the same shape as
[where a member says it is](../R1.1/discovery.md#where-a-member-says-it-is):
supplied by the registrant, kept raw, and not a value the daemon interprets or
routes on. What it has to answer:

| | |
|---|---|
| **which verbs there are** | a service's callable methods, each described on its own, so a catalog lists them instead of one sentence about the whole service |
| **which of them destroy something** | the hint a face needs before it calls one on a person's behalf |
| **generated, not maintained twice** | the faces build their catalog from what the service stated, rather than from prose somebody keeps in step by hand |

The representation, and whether the daemon checks its shape at all, is release
design work after scope confirmation. The description field stays either way: a
service that states nothing keeps exactly today's behavior.
