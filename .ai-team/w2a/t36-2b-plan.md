# T-36 step 2b plan: #ai serves itself, images go to its bigfile

Author #ab-w2a, 2026-11-05. Plan only, no code yet. Base: main `c722581c` (1.0.4, step 2a).

## Shape

| Part | Change |
|---|---|
| `agent.json` | add `"serves": "itself"`; `processes` `"0..3"` becomes `2` (lead decided, 2026-11-05); `form` and `deadline: 60s` stay |
| `ai.py main()` | replaced by an imap-style loop: `with Bus(timeout=5) as bus`, `bus.consume(share=True)` per process, `dispatch` for help asks (an agent that serves itself answers help in its own process), `message.reply(answer(...))`, SIGTERM ends the loop |
| roles | `answer(..., roles=frozenset(message.roles) & MANAGERS)`; `delivered_roles(os.environ)` is removed. The daemon sets `roles`; a sender cannot |
| key / config | read once at start; a missing key exits 78 in-process (env.dist required-key checks do not run for serves-itself) |
| deadline | each message still gets 60 s: one `arm_deadline` per message around `answer()`, cancelled after the reply, so a stuck call answers "error: … deadline" and the process lives on |
| `generate_images` | the only new behaviour, below. Every other method calls the same `answer()` with the same arguments and gives the same text, byte for byte |

## Image answer

0. No `image_model` configured and no `model` in the request: answer `error: no image model configured` before any provider call; never guess a model (lead, Q337). One test: the provider sees no request.
1. Provider call as in 2a (`image_request` limits unchanged). Decode each returned `b64_json` / `data:` URL; a non-image or undecodable item is "error: provider returned no image".
2. Before writing: `bigfile.list()` of `#ai` itself. If keys + n > 100 or bytes + total > 256 MiB, answer `error: #ai bigfile record is full (<keys> keys, <bytes> bytes of 100 keys / 256 MiB); try again after old images expire (24 h)` and write nothing. A `Refusal` with status 413 from `put` (a race with the other process) gives the same answer; images already written in this call stay listed by name.
3. Each image: `put(name, bytes, ttl=86400)`, `name = "image-" + secrets.token_hex(8) + "." + ext` (ext from the media type: png, jpeg, webp). One image over 32 MiB is refused by name, nothing written for it.
4. Answer, one line per image plus the usage line already used for text:
   `#ai@parf.us/image-3f9c…​.png image/png 1234567 bytes, kept 24 h` … `[cost 0.04 USD]`.
5. A 403/503 from bigfile answers `error: #ai cannot store images (<status>: <message>)`.

The cost is paid before the write; a full record refuses after the provider call. To avoid paying for a refused write, step 2 runs before the provider call with the request's `n` and a 32 MiB-per-image reserve, and again after decoding with the real sizes.

## Tests (fixtures only, no live bus, no provider)

| Test | Fixture | Proves |
|---|---|---|
| image writes n keys, answer names each with type, size, cost | fake provider + fake bigfile (in-memory `put/list`) | names random, ttl 86400, sizes match bytes |
| full record refused by name, nothing written | fake bigfile with 100 keys; second case 256 MiB−1 | **mutant**: drop the pre-write check → test fails on the "record is full" assertion (and on the put count) |
| 413 from put mid-call | fake raises `Refusal(413, …)` | answer names the written ones and the refusal |
| role gate on the kept path | fake message with `roles=()` and `("maintainer",)`, and a body carrying a forged `roles` field | **mutant**: gate reads all roles / ignores `message.roles` → fails on "needs the owner or maintainer role" |
| every other method unchanged | the 40 existing tests rerun through the kept-loop entry with a fake message | same answers |
| missing key exits 78; SIGTERM ends loop | subprocess with a fake Bus module | exit codes |

Mutants run under the smoke-gate lock, owning test only.

## Deploy for #ab-ceo, in order (after a green normal gate and review)

Resolve, not guess: `RUNNER=$(agent-bus ls --tag=runner | grep parf)` (expected `#runner/parf@parf.us`); `RUNNER_DIR` from its status (expected `/var/lib/agent-bus/parf-ab-runner`, user `parf-ab-runner`).

1. Token handoff ([self-serving credential handoff](../../docs/agents/create-agent-prompt.md#self-serving-credential-handoff)):
   ```
   sudo install -d -m 700 -o "$RUNNER_USER" "$RUNNER_DIR/tokens"
   sudo install -m 600 -o "$RUNNER_USER" /dev/null "$RUNNER_DIR/tokens/%23ai@parf.us"
   agent-bus token '#ai@parf.us' | sudo tee "$RUNNER_DIR/tokens/%23ai@parf.us" >/dev/null
   ```
2. Inventory: `runner/agents.json` is an object keyed by package directory (`ai`, or `ai/<instance>`); an entry is optional, and with none an installed, not bundled package is enabled (`src/cmd/agent-bus-runner/config.go:69-74`). Check `jq '.ai.enabled // "no entry: enabled by default"'`, never `jq .ai` alone; a `false` there is the only thing that stops it. Confirm with the runner's own list (`agent-bus call "$RUNNER" list`, state of `#ai@parf.us`). No field change (`serves` and `processes` come from agent.json).
3. Package: back up `agent.d/ai` to `agent.d/pkg-backup-<date>-ai`, then `package.sh` into `/var/lib/agent-bus/agent.d/ai`.
4. `agent-bus call "$RUNNER" reload`.
5. `src/check-deploy.sh '#ai@parf.us'`, then `agent-bus send '#ai@parf.us' models`. No live paid `image` check (lead, Q333).

## Rollback

1. Move `agent.d/ai` aside, restore `agent.d/pkg-backup-<date>-ai` to `agent.d/ai`.
2. `agent-bus call "$RUNNER" reload` (the runner stops the kept processes and goes back to a script per message).
3. `src/check-deploy.sh '#ai@parf.us'`.
4. Remove the token file: `sudo rm "$RUNNER_DIR/tokens/%23ai@parf.us"`.
5. Rotate #ai's token so the handed-off one is dead: `agent-bus token --rotate '#ai@parf.us' >/dev/null` (the new value is not stored; a later 2b redeploy hands off afresh). Check: the old value is refused.
6. Images already in the bigfile expire in 24 h; nothing to clean.

## Docs

`docs/agents/ai.md` (image method, where images go, limits), `src/agents/ai/README.md`, regenerated `docs/agents/README.md` and `help.golden.json`; R1.1 TODO T-36 row: 2b done.

## Readers

Two: #ab-w2b (knows ai.py from L1–L5) and one Claude reader — #ab-qa2 if it is Claude, otherwise #ab-w2c.

## ETA and fit

Code + tests ~45 min, mutants ~10 min after the lock, docs ~10 min: ready about 70 min after start. Context: this plan was written after a compaction; 2b fits in one window if I start fresh from this file. If my context passes 70 % before the mutants, I checkpoint and hand off from this file.

## Decided (lead, 2026-11-05)

- processes 2; no live paid check at deploy; rollback removes the token file and rotates #ai's token.
