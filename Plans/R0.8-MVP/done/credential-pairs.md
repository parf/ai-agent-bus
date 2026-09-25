# Credential pairs

📌 **TL;DR:** 0.7.6 binds every credential to the User it acts for and, for an
agent, the Agent, by internal ID. Every call checks that pair against the
registry; a transfer rebinds and a removal deletes credentials in its own
commit; a stored pair that disagrees is ignored and alerted to both logs.
Evidence for [K.5 and K.21](../0.7.0-TODO.md#storage-and-identity).

## Result

| Area | Built |
|---|---|
| Pair | `user_id` and `agent_id` on every credential row; zero means issued before binding, bound on first check |
| Issue | only a User or an Agent; a queue, topic or service is refused for its kind |
| Every call | the pair must equal who the name is now, else `401` as a bad token |
| Transfer | the agent's credential rows are rebound in the transfer's transaction; a failed commit moves neither record nor rows |
| Removal | the removal's transaction deletes the credential rows and every ACL, Maintainer, Group-member and Deliver-To reference |
| Recreation | a recreated name has a new ID, so the former holder's bytes answer for nothing, before or after restart |
| Load | a mismatched pair, or a credential on a kind holding none, is ignored, left in the store and reported as an alert naming User, Agent and current Owner, never the token |
| Last use | persisted in one batch with the queue flush |
| Store | the token port writes one row at a time, so no write rewrites or resurrects another principal's credential; schema 3 |

## Checks

| Check | Mutation that fails it |
|---|---|
| `api` TestATransferRebindsTheAgentsCredentialInItsOwnCommit | not staging the rebind; SQLite ignoring `Change.Credentials` |
| `api` TestAFailedTransferKeepsTheOwnerAndTheCredential | — the failure half of the same rule |
| `api` TestARecreatedAgentNameIsNotAnsweredForByTheOldCredential, `core` TestRemovingAnAddressAndItsCredentialIsOneOperation | keeping the credential row on removal |
| `api` TestEveryCallChecksThePairAgainstTheRegistry | skipping the per-call pair check |
| `api` TestOnlyUsersAndAgentsAreIssuedCredentials | issuing a queue a credential |
| `api` TestAMismatchedStoredCredentialIsIgnoredAndReported | reporting the mismatch below alert |
| `api` TestRemovingARecordTakesEveryReferenceToIt | leaving ACL and Maintainer references |
| `api` TestLastUseSurvivesARestart | not flushing last use; not loading it |
| `store/sqlite` TestCommitCarriesCredentials | a failed transaction keeping its rebind |
| `smoke.sh` "a corrupt credential pair is reported to both logs, and a refusal to neither" | dropping the syslog copy; logging an unknown-receiver refusal |
