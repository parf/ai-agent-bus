# F3 bounded JSONL answers

📌 **TL;DR:** An oversized JSONL answer gets a request-specific error, while
other messages and the same child remain usable. An oversized line with no
parsable top-level message ID kills the child and fails its pending calls.

## Behaviour

The fix is `0ce4e774`. A `bufio.Reader` retains at most the runner's existing
1 MiB answer limit and discards the remainder through the newline. The
ordinary daemon JSON request cap is also 1 MiB; bigfile's separate transfer
limit does not enlarge message answers.

The retained JSON prefix must contain a top-level string `message_id` before
any truncated value. An ID inside nested JSON or a body is not used. A known
request receives `error: answer too large (limit 1048576)` through the normal
reply route. Other pending requests keep their slots and receive their own
answers. A later request uses the same child. Without a parsable ID, every
pending request receives an explicit output failure and that child is killed.
The existing request deadline also bounds a child that never finishes its line.

## Proof

The real-caller regression `TestJsonlOversizedAnswerIsReturnedToTheCaller`
runs a real disposable API and a shell child emitting 2 MiB. Against main's
production files it fails on its caller-answer assertion in 1.810 s: no answer
arrives. The fixed caller receives the named error through the bus.

Three compiling mutants ran under a separately acquired, confirmed slow lock,
each running only its owning test:

| Mutant | Assertion-red test | Result |
|---|---|---|
| Skip oversized-line handling | `TestJsonlOversizedAnswerFailsOnlyItsRequestAndKeepsProcess` | Deadline error instead of explicit size error; 0.906 s |
| Do not kill the child without an ID | `TestJsonlOversizedAnswerWithoutIDFailsAllPendingAndKillsChild` | Child was not killed; bounded assertion, 1.006 s |
| Drop the caller-facing error reply | `TestJsonlOversizedAnswerIsReturnedToTheCaller` | Caller receives no reply; 0.922 s |

Sources were restored before the lock was released. All five new focused tests
pass (0.163 s; race run 1.395 s). They also cover another request already in
flight, exact-limit boundaries, line alignment after discard, and top-level ID
parsing. The same-process test checks exactly one child start and a successful
next request within a 900 ms context. No host service or live daemon is used.

The full owning runner package passes under `-race` (33.411 s); `go vet` and
changed-document links pass. Initial package runs exposed missing worktree
Node dependency links; after linking the shared dependencies, the three affected
packaging checks pass (1.996 s) and the complete race run is green.

## Review follow-up

`5ae703f5` quotes the agent-chosen ID with `%q` and limits it to its first
128 bytes before logging. `TestJsonlOversizedAnswerLogsOnlyAQuotedCappedID`
uses an ID with a newline and more than 128 bytes; independently removing
quoting or the cap compiles and fails its exact-log assertion (0.010 s each).
The restored test passes (0.010 s), and all six focused tests pass under race
checking (1.764 s). The worker-held lock was released after restoring sources.
Other ID logs in this path use pending/request envelope IDs from the daemon,
not the agent's parsed output ID.

Per the narrowed review direction, duplicate-ID handling is unchanged: the
oversized prefix uses the first top-level `message_id`, while normal complete
JSON unmarshalling uses the last. The source comment states that distinction.
