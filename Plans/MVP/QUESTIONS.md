# MVP questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q7 | What carries a service's method information | owner, with the MVP faces | [services § service and template](../../docs/03-services-and-topics.md#service-and-template) |
| Q9 | How the local user-to-account map is administered; per-record ACL editing is now built in the dashboard | owner | [setup § the programs](../../docs/09-setup.md#the-programs) |
| Q10 | npm install vs Go-first for the first release | owner | [setup § install](../../docs/09-setup.md#install) |
| Q21 | Whether reading an inbox and filtering one become separate options | owner, with the MVP CLI | [messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox) |
| Q30 | How the daemon no-exec target accommodates the current ssh-keygen verifier | owner | [context](../../docs/11-processes.md#nothing-the-daemon-runs-may-exec) |
| Q39 | What durability and recovery guarantee an acknowledged ban, group removal or ACL restriction has across a crash or failed snapshot write | owner, before H.5.3 | [current behavior and required decision](../../docs/04-messaging.md#administrative-crash-recovery) |
| Q40 | How an orphaned record is removed or reclaimed: its owner is a principal nobody holds and it names no maintainers group, so daemon administration reaches neither and the only route found is minting a credential for the dead owner | owner | [identity § groups and maintainers](../../docs/01-identity.md#groups-and-maintainers) |
