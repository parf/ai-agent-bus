# R1 → 1.0.0 release runbook (DRAFT v2, untracked, for #ab-ceo and #ab-dir review)

Drafted by #ab-w2b, 2026-11-01. Sources: CLAUDE.md § Versioning and § Verification, CLAUDE.local.md, `.github/workflows/release.yml`, `.ai-team/gate.sh`, `docs/07-setup.md` (deploy), `Plans/R1.0-Release/distribution.md` (npm, image), `DECISIONS.md` (Q296, Q300, "R1 releases as 1.0.0").
Nothing here is run; every step names who, which lock, which check, and how to back out. v2 folds #ab-dir's and #ab-ceo's decisions of 2026-11-01 17:07: one STABLE commit landed by the gate's `stable` mode, the Fable pass run by #ab-ceo, a one-arch image, an annotated unsigned tag, and an absolute freeze. Q313 is settled: both macOS archive editions and the two macOS client npm packages ship.

## Ground rules for the whole run

| Rule | Why (source) |
|---|---|
| **Absolute freeze from step 0 until the tag exists** (decided). No landing, no worker mutants or gates, no lock use by anyone but the actor of the current step | the slow run and the reviewed tree must be the same tree; the gate bumps PATCH on every `bump` landing, which would move VERSION off 1.0.0 |
| Only `#ab-gate` runs gates, as `parf`; a worker never runs `--slow all` | CLAUDE.md § Verification (lock owner `parf`; J79 is post-release) |
| `df -h /` ≥ 30 GB free before step 1; clean own `tmp/` first | CLAUDE.md § Clean up after yourself |
| Every rebase: `git grep -nE '^(<<<<<<<|\|\|\|\|\|\|\||=======|>>>>>>>)' -- <changed files>` prints nothing | standing rule 2026-11-01 |
| Nothing pushed or tagged by anyone but `#ab-ceo`; fast-forward only, never forced | CLAUDE.local.md |

## Steps

### 0. Freeze, gate swap and baseline — `#ab-dir` announces, `#ab-ceo` confirms and swaps
- `#ab-ceo`, while the gate is idle (the J66 pattern): `cp .ai-team/gate.sh.stable .ai-team/gate.sh` (atomic: temp + rename), after reading `.ai-team/gate.sh.stable.diff` and the dry-harness output in `tmp/gate-stable-harness/out.txt`. The old forms behave the same; the changelog file name now follows VERSION's major.minor; `stable` is the new form.
- Check: `git log main --oneline -1` is the candidate tip **T0**; `git status -s` clean in the main checkout; every open branch landed or parked (list from `git worktree list`, `ls tmp`); R1 TODO has no ☐/⏳ row that is meant to block (see questions 1–3).
- Check: `src/check-links.py` on the tree; `git grep` marker rule on all of `Plans docs src`.
- Back out: lift the freeze (`release`) — nothing changed yet.

### 1. `--slow all` on T0 — `#ab-gate`, lock `smoke-gate@parf.us` name `slow`, as `parf`
- Command: in a fresh worktree of T0: `src/smoke.sh --slow all` (CLAUDE.md: a release or STABLE commit uses `--slow all`); capture `rc=$?`, never chain with `;`.
- Lock plan: take with a 15-minute ttl in its own call and read the answer; extend before expiry (at ~10 min, by the same actor, `agent-bus extend smoke-gate@parf.us slow --ttl=15m` or `ab_lock_extend`) as many times as the run needs; release the moment it ends, pass or fail; nobody else takes it meanwhile. If the gate record shows "you already hold it" and you did not take it, the gate or a worker has it: wait, never extend.
- Check: final line `passed N, failed 0`, `rc=0`, no `FAST OVER 15s`, no "(skipped)" check, log kept in `tmp/gate-logs/` (evidence goes to the R1 DONE entry, not left only in tmp/).
- A red run: one rerun is the gate's rule; a second red = stop. The failing check is named (test, file, error) and goes to step 3's block; a flake already filed (J-number) is named, not hidden.
- Back out: nothing to undo; release the lock; the freeze stays.

### 2. Final full Fable review of the slow-green T0 — run by `#ab-ceo` once step 1 is green (decided)
- Who and how: `#ab-ceo` launches the Fable pass (Agent tool, `model: fable`) on the slow-green T0: at most 10 agents, two per ⚠️ subsystem (A, B, C, D, F, G, Q) plus one mixed, all read-only, using `.claude/skills/find-issues` and `.claude/skills/review-checklist.md`; each reproduces a finding before reporting it; none edits, none takes the gate lock (scratch daemons only).
- Brief (saved under `Plans/R1.0-Release/`): scope is the whole tree at T0 against the 1.0 contract (docs/09 wire contract, docs/02 access, the process boundary, docs/07 setup, release.yml and the packagers).
- Output: `Plans/issues/2026-11/<date>.md` under `# Issues — <date> · <first>..<T0>` in the find-issues markup, severity first, and it **names the tree it read** (`tree <hash of T0>`).
- Findings → one R1 TODO block "1.0 final review" with every 🔴 and 🟠 as a row with an owner; `#ab-dir` decides each 🟠 (fix, or "not fixing" with a reason); ⚪/🔵 do not block. Fixes land through the normal gate during the freeze exception `#ab-dir` grants for them; then re-run step 1 if code or packaging changed, or step 2 on the delta if docs only; T0 becomes the new tip.
- Check: no open 🔴, every 🟠 closed or accepted, the report's tree equals the T0 the STABLE branch is cut from.
- Back out: nothing is committed yet; loop.

### 3. The STABLE commit — prepared by `#ab-ceo` on a branch, landed by `#ab-gate` in its `stable` mode (decided: one commit)
- Branch `ab-ceo/stable` cut from T0, **one commit** whose tree is T0 plus exactly: `src/internal/version/VERSION` = `1.0.0` (`printf '1.0.0\n' >`), `git mv CHANGELOG.0.9.md Plans/CHANGELOG.0.9.md`, a new root `CHANGELOG.1.0.md` (TL;DR block, link to the 0.9 file, then `## 1.0.0 — <date>` with the one- or two-line summary; 1.0.0 is the first entry), and the docs that name the moved file or quote the current line (`grep -rn "CHANGELOG.0.9" .`; CLAUDE.md "What this repo is" / Versioning rows are updated after the release, see below). `src/check-links.py` rc 0 (it reads every link to a moved path).
- Message to the gate: `ab-ceo/stable T-<n> stable -- tree <T0 tree hash>; slow <log path>; review <report path>`. The line is required and becomes the commit body.
- What the gate does (`gate.sh.stable`): refuses unless the branch is exactly one commit, its VERSION is X.Y.0 and higher than main's X.Y, its `CHANGELOG.X.Y.md` has `## X.Y.0 `, and the line is present; it never combines with `bump`. It then runs the same normal gate (lock `smoke-gate@parf.us` slow, one rerun on red), raises nothing, lands the commit with the subject **exactly** `STABLE - passed all tests and reviews` and the line as its body, and fast-forwards main. Selection ignores VERSION and root CHANGELOG, so the run is fast but real.
- Check: `git log -1 --format=%s main` equals the subject; `git diff T0 main --stat` lists only the files above; the receipt says `STABLE 1.0.0`; `git rev-parse T0^{tree}` and the reviewed report agree except for those files.
- Back out (before the push): the commit is local on main; `#ab-ceo`, with the owner's OK, `git reset --hard T0` removes only it. After the push there is no back-out: step 5.

### 4. VERSION and the changelogs — part of step 3's one commit (no separate step)
- The one canonical VERSION literal is `1.0.0`; the gate derives later changelog files from it: after this lands, `bump` writes `CHANGELOG.1.0.md` (verified in the dry harness, case D).
- Check: `cat src/internal/version/VERSION` is `1.0.0`; `go test ./internal/version`; `Plans/CHANGELOG.0.9.md` exists and root `CHANGELOG.0.9.md` does not.

### 5. Fold for the push, then the push — fold: `#ab-ceo` (commit alone, shown to the owner); push: `#ab-ceo`, fast-forward only
- Fold (CLAUDE.md § Versioning "Push"; decided): the unpushed 0.9.x entries become one summary of the sixteenth published changeset (the fifteenth is `35959628`): heading `<last 0.9 version> — <date>`, "16th published changeset: <first>–<last>, squashed", a table by area, written in `Plans/CHANGELOG.0.9.md` (the file moved in step 3); `CHANGELOG.1.0.md` is not folded. Commit it alone on top of the STABLE commit, show it to the owner, then push. The range comes from `git log origin/main..main`.
- Command (only `#ab-ceo`): `git push origin main` after: VERSION `1.0.0`, the squashed changelog head, no conflict marker (`git grep` rule), `git merge-base --is-ancestor origin/main main`, and the reported pushed range (`git log --oneline origin/main..main`). Check the unreviewed-commit memory rule: nothing in the range that did not pass the gate.
- Check: `git ls-remote origin refs/heads/main` equals local `main`; the pushed range is reported to `#ab-dir`.
- Back out: **none after the push** (no force). A defect found later is fixed forward (1.0.1) with a new tag; the tag and release (step 7) must not exist yet, so do not create them before the owner's go.

### 6. Node deploy — `#ab-ceo` under the standing OK, gate idle, leads held
- Authority: the owner's standing OK for dev restarts covers it (decided), backup first. Commands: `sudo src/git-install.sh` (links every program and launcher from the checkout); daemon restart with a backup of `/var/lib/agent-bus/daemon.backup/` first (CLAUDE.md § Clean up: backups live only there); runner reload; then `src/check-deploy.sh` as an allowed caller (optional target `'#debug@<node>'`).
- Check: check-deploy "6/6", `agent-bus --version` shows `1.0.0` with the build stamp (CLAUDE.md "Build evidence"), and the live daemon answers `GET /identity` with `protocol` 1.
- Back out: `sudo src/release.sh --rollback` is for a node running a release copy; on this node (checkout links) roll back with `git -C <checkout> checkout <T0>` + `sudo src/git-install.sh` + restore the daemon backup, only if the daemon failed to start; the owner is asked before a second restart.

### 7. The release tag — `#ab-ceo`, **only with the owner's explicit go**
- Command: an annotated, unsigned tag (decided, RD178): `git tag -a v1.0.0 <STABLE sha> -m "agent-bus 1.0.0"` then `git push origin v1.0.0`. The workflow's first step requires `v$(cat VERSION)` = the tag **and** the commit to be an ancestor of `origin/main`, so the push (step 5) comes first and the tag must name the pushed STABLE commit (not a later one: VERSION would differ).
- What runs: `release.yml` has two Linux `build` matrix runs (`ubuntu-24.04`, `ubuntu-24.04-arm`, Python 3.14, Go from go.mod, Bun 1.4.3) and two `darwin` cross-build runs on `ubuntu-24.04` (`GOARCH=arm64` → archive `aarch64`, `amd64` → `x86_64`). Each builds and verifies the full and client archives. The `release` job needs both matrices.
- Assets: **eight** archives: `agent-bus-1.0.0-{linux,darwin}-{x86_64,aarch64}.tar.gz` and `agent-bus-client-1.0.0-{linux,darwin}-{x86_64,aarch64}.tar.gz`, plus `SHA256SUMS` for exactly those eight archives. The release job also attaches `ai-agent-bus-client-darwin-arm64-1.0.0.tgz` and `ai-agent-bus-client-darwin-x64-1.0.0.tgz`; there is no full Darwin npm platform package. These two npm tarballs are not listed in `SHA256SUMS`.
- Publication: after download/merge, the job runs `src/release-verify.sh --os "linux darwin" dist`, wraps the Linux x86_64 pair with both `--darwin-client` archives, and requires both client npm tarballs. If `gh release view` finds the tag's release, it uses `gh release upload ... --clobber`; otherwise it uses `gh release create ... --verify-tag`. Existing releases are therefore updated, not refused.
- Verify (by `#ab-ceo`, results to the release lead): `gh run list --workflow release --limit 1` and `gh run view <id>` show both Linux builds, both Darwin builds and the release job green. `gh release view v1.0.0 --json assets` must show exactly the eight archives, checksum file and two named npm tarballs above (11 assets).
- Archive check: download only the archive inputs into an empty directory: `gh release download v1.0.0 -D <archives-dir> -p '*.tar.gz' -p SHA256SUMS`; then `(cd <archives-dir> && sha256sum -c SHA256SUMS)` and `src/release-verify.sh --os "linux darwin" <archives-dir>`. Download the two `*.tgz` npm assets separately; do not put them in the archive verifier's directory, which rejects unexpected files.
- Linux aarch64 evidence: the `build (ubuntu-24.04-arm)` log must show both named aarch64 archives present and `Verify every archive before upload and release creation` green; on that native runner, `release-verify.sh` runs shipped programs with `--version` and checks build stamps/manifests. The DB agent is deliberately excluded by `src/agents/package.sh`; do not claim its library is in a published archive.
- Limits: the hosted tag run proves tag/VERSION/main ancestry, setup, artifact transfer, token permissions and publication. Foreign Darwin verification checks Mach-O CPU headers, versions, build stamps and manifests; it does not execute native macOS programs. Verify both Darwin full archives explicitly: the directory verifier also allows a client-only Darwin set.
- After the release job is green: replace the workflow's fixed note with a real one (`gh release edit v1.0.0 --notes-file <changelog summary and install lines>`), `#ab-ceo` (decided).
- Back out: a failed Linux or Darwin build prevents the release job; a pre-existing release is left unchanged. Only with the owner's go may `#ab-ceo` remove an unpublished failed tag (`git push origin :refs/tags/v1.0.0`) and fix forward. A bad release object uses the existing owner-controlled procedure (`gh release delete v1.0.0 --yes --cleanup-tag`); never move a published tag.

### 8. The docker push — `#ab-ceo`, standing OK, credentials from `.env` through `--password-stdin`
- A one-arch (x86_64) image, and `:latest` moves (decided). Build from the release's own x86_64 archive, not a re-build: `podman build --build-context release=<dir with the v1.0.0 archives> --build-arg ARCHIVE=agent-bus-1.0.0-linux-x86_64.tar.gz -t agent-bus src/container` (src/container/README.md); the Containerfile checks that selected archive against SHA256SUMS. Tag `docker.io/parf/agent-bus:1.0.0` and `:latest`; `podman login docker.io --username <from .env> --password-stdin` fed from the `.env` credentials only, push both tags, `podman logout docker.io`.
- Check: report tags and digest; verified by a clean pull on another host or an emptied store: `podman rmi` then `podman pull docker.io/parf/agent-bus:1.0.0`, the digest equals the pushed one; `src/container/acceptance.sh` (rootless podman) on the pulled image; take the `smoke-gate` slow lock as for a gate (podman rule) and release with `agent-bus release`.
- Back out: re-push `:latest` to the previous image digest (`podman tag <previous digest> :latest`, push); `:1.0.0` cannot be reused, so a bad one gets `:1.0.1`; logout in all cases.

### 9. The npm publish — **OWNER** (needs the owner's `npm login` as `parf73`, 2FA), Q300: right after STABLE
- Prepare (`#ab-ceo` or a worker, no registry write): `gh release download v1.0.0 -D <dir>`; `src/npm-pack.sh <dir>/agent-bus-1.0.0-linux-x86_64.tar.gz <dir>/agent-bus-client-…` (see src/npm/README.md for the exact argument list, one archive per edition and arch, plus both `--darwin-client` archives: `agent-bus-client-1.0.0-darwin-aarch64.tar.gz` and `agent-bus-client-1.0.0-darwin-x86_64.tar.gz`) `<dir>/SHA256SUMS OUT`; `npm pack` the emitted packages; run `src/acceptance/npm-clean-install.sh OUT` (clean container, local tarballs, request/reply and an MCP tool call).
- Publish (owner, in this order, each `npm publish --access public <tarball>`): the platform packages (`ai-agent-bus-client-linux-x64`, `…-arm64`, `ai-agent-bus-linux-x64`, `…-arm64`, and `ai-agent-bus-client-darwin-arm64` / `ai-agent-bus-client-darwin-x64`), then `ai-agent-bus-client`, then `ai-agent-bus`; the command packages pin the exact-version platform packages, so they go last.
- Check: `npm view ai-agent-bus@1.0.0 dist.tarball dependencies` and the same for the client; `npm install -g ai-agent-bus@1.0.0` in a clean container, then the P.1 acceptance against the real registry, then `agent-bus --version` = 1.0.0.
- Back out: npm unpublish is allowed only within 72 hours and only if nothing depends on it; the owner decides between `npm unpublish ai-agent-bus@1.0.0` and `npm deprecate ai-agent-bus@1.0.0 "<reason>"` plus 1.0.1; no worker runs either.
- Mac check (`#ab-ceo` via `ssh mac`, after the publish check above): one check on the published 1.0.0 on the Mac, `npm install` of the client package and `agent-bus --version` = 1.0.0, light touch per the Mac rules.


### After the release (docs only, via the gate, no bump)
- Flip "publication pending" to published in `docs/07-setup.md` (lines about npm and "Publication pending"), `README.md`, `src/npm/README.md`, `Plans/R1.0-Release/TODO.md` P.1 (✅ with the evidence shas), and CLAUDE.md "What this repo is" and the Versioning table (the 0.9 line is closed; 1.x next); move R1 to `Plans/done/` per the plan lifecycle; open the R1.1 stage as current.

## Open question

1. ~~**Q313 (owner): does 1.0.0 ship macOS?**~~ Settled 2026-11-02: SHIP. The macOS archives and npm client packages are in 1.0.0, so team 1 prepares the `release.yml` change before step 0, step 7 verifies the Darwin assets and step 9 publishes the darwin client platform packages. No question stays open.

Decided and removed from this list: VERSION and STABLE as one gate-landed commit; the changelog name derived from VERSION; the Fable runner and brief; image arch and `latest`; the restart authority; the fold wording (sixteenth changeset); the unsigned annotated tag; the release note; the absolute freeze.
