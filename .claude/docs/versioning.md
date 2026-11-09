# Versioning detail

📌 **TL;DR:** Version lines, the release step, changelog and push forms, and build evidence. Who owns a version and where its value lives is [CLAUDE.md § versioning](../../CLAUDE.md#versioning); this page holds the rest.

## Lines

| Rule | |
|---|---|
| MVP | `0.5.x`, then `0.6.x` from the record-kind enum, both released; `0.7.x` from the constitution work, stable at `0.7.20`; `0.8.x` from `0.8.0`, closed at `0.8.243`; `0.9.x` from `0.9.0`, an odd development line |
| Stable | An even minor (`0.6`, `0.8`) is a feature line. An odd minor (`0.7`, `0.9`) is major development whose code may be broken; when it passes all tests and reviews — including a final full Fable review — its commit subject is exactly `STABLE - passed all tests and reviews`, and the next even line begins |
| R1 release | The `0.9` line's STABLE is R1's release: the version moves from `0.9.x` to `1.0.0`, `CHANGELOG.0.9.md` moves to `Plans/changelogs/` and a 1.0 changelog starts; ordinary `0.9` PATCH bumps and changelog entries continue until then, and the move to `1.0.0` (the `VERSION` value and the changelog change) happens only at that release |
| Later releases | Before stability, MINOR advances the release line. From major one, breaking changes bump MAJOR, compatible features MINOR, fixes PATCH; reset lower components when advancing a higher one |
| App subprojects | An app built on top of the bus from agents, such as [RAG](../../Plans/R1.0-Release/rag/README.md#versioning), keeps its own SemVer, bumped automatically on each step; the blocks it uses stay on the shared version |
| No behaviour change | Docs, tests and refactoring alone do not require a bump |

## Changelog and push

| Rule | |
|---|---|
| Changelog | Every version bump includes one summary of its deploy window in the current line's changelog — `CHANGELOG.0.9.md` at the repository root from `0.9.0` — newest version on top. One file per release line, and a finished line's file moves to `Plans/changelogs/`; historical release numbers stay in theirs and are never rewritten to match the current version |
| Push | Before a push, replace the unpushed versions' entries with one summary of the unpushed changes: heading `<last version> — <date>`, "Nth published changeset: <first>–<last>, squashed", and a table by area. Commit it alone, show it to the owner, then push |

## Build evidence

Shipped Go binaries must carry [build information](deployment.md#build-information) and expose it through the version query. Use the documented build script; never ship an unstamped development build.
