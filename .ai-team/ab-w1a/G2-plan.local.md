# G2: root setup runs system tools by absolute path (approach only)

📌 **TL;DR:** checked at HEAD 52e4576d. Root setup still finds `systemctl`, `useradd`, `usermod`, `userdel`, `chown` and `ldd` through PATH at 34 call sites (24 via `run`, 2 via `commandOutput`, 8 direct). The user-runner part already pins them with `tool()`/`toolDirs`. Plan: pin every root exec through the same `tool()`, centralized in `run`/`runIn`/`commandOutput` plus 8 direct sites. Add a planted-PATH test and an AST guard. No PATH sanitizing.

## HEAD check (52e4576d)

- **Status:** the issue entry (2026-11-01.md, G2) is ⚪ "not fixing", because sudo's `secure_path` governs root. The 11-05 recheck lists it again. Reopening it is #ab-dir's call: the rationale holds only while setup runs under sudo. `sudo -E`, `su`, a root shell or a `secure_path` with a user directory all bypass it.
- **Already pinned:**
  - `user_runner.go:63` (`getent`), `:513` (`systemctl --version` through `toolPath`), `:677` (helper argv) and `:741` (`setpriv`);
  - `main.go:538` and `:367` run setup's own absolute `exe`;
  - `main.go:427` runs `admin`, an absolute path.

## Approach: absolute paths through `tool()`, not a sanitized PATH

- **Why `tool()`:** a sanitized PATH still leaves a lookup, and any child that reads PATH inherits a choice we did not audit. `tool()` already exists, is tested and is the rule in `user_runner.go`, so the same root process gets one lookup rule.
- **No PATH rewrite:** the children's environment is left as it is today, so a tool that runs others (for example `useradd` hooks) behaves as before.

## Call sites

| Where | Today | Change |
|---|---|---|
| `main.go:661-666` `run`, `runIn` | `exec.Command(name)` | a name without `/` goes through `toolPath`; a missing tool fails as "X is not in /usr/bin, /bin, /usr/sbin, /sbin" |
| `upgrade.go:394` `commandOutput` | same | same |
| through `run`: `main.go:322,334,351,382,385,478`; `upgrade.go:51,67,197,229,262,265`; `web.go:184,188,192,227,236`; `record_backup.go:15`; `runner_install.go:204,297,300,307,317,324` | `systemctl`, `useradd`, `usermod`, `userdel`, `chown` by name | none: fixed by `run` |
| `main.go:311,488` | `exec.Command("systemctl", …)` | `tool("systemctl")` |
| `runner_install.go:410,411` | same | `tool("systemctl")` |
| `web.go:50` | `exec.Command("ldd", …)` | `tool("ldd")` |
| `web.go:191,235,332` | `systemctl` | `tool("systemctl")` |
| `main.go:654` `sudoLine` | `LookPath` | unchanged: it only prints a line for a person to run, never execs |

- `tool`/`toolPath`/`toolDirs` stay in `user_runner.go`, so that file is not edited.

## Files G2 touches, against team 2's list

| Team 2 file | G2 |
|---|---|
| runner `overlay.go` (F5) | no |
| setup `runner_install.go` (F7) | yes: 2 lines, 410-411 |
| setup `user_runner.go` (F7) | no |
| runner `install.go`, `control.go` (F1) | no |

- **Other files:** `main.go`, `web.go`, `upgrade.go`, and the tests named below. `runner_install.go` is the only overlap, so G2 goes after F7 lands.

## Distros

- `toolDirs` is `/usr/bin`, `/bin`, `/usr/sbin`, `/sbin` in that order, which covers merged-`/usr` (Fedora, Debian 12+, Arch) and split layouts (older Debian, Alpine `/sbin/useradd` via shadow).
- **A missing tool:** `run` fails by name before any exec, with the directories it searched, rather than at the absolute fallback. Only `tool()`'s callers that must not fail early (the status probes at `runner_install.go:410-411`, `main.go:311`) keep `tool()`'s absolute-fallback behavior. There, a missing systemctl reads as "not active", as today.
- **macOS:** `darwin.go` execs none of these tools, so it is unchanged.

## Tests

- **Existing tests:** 7 test files fake tools by prepending PATH (`main_test.go:168`, `runner_authority_test.go:33`, `upgrade_test.go:225`, `web_restart_test.go:42`, `start_limit_test.go:22`, `tls_restart_test.go:41`, `install_sh_test.go:309`). Each moves to a `fakeTools(t, dir)` helper that sets `toolDirs`. Under the fix, a PATH fake would stop being run, and the test fails loudly. That shows the change is real.
  - `install_sh_test.go` drives `install.sh`, a shell script outside G2. It stays on PATH if it never reaches setup's Go exec; I verify this before editing.
- **New test:** `TestRootToolsNeverComeFromPATH`, modelled on `user_runner_test.go:445`.
  - It puts recording look-alikes (`systemctl`, `useradd`, `usermod`, `userdel`, `chown`, `ldd`) first on PATH, each writing to a marker. Trusted recording stubs go in `toolDirs`.
  - It drives `run`, `runIn`, `commandOutput`, the daemon status probe (main.go:311), the start-limit read (main.go:488), runner status (runner_install.go:410-411), `webRuntimeOK` (web.go:50), and the web restart and stop.
  - **Assertions:** the marker file is absent, and every trusted stub was called once (counted, so a skipped site cannot pass).
- **AST guard:** `TestNoRootExecByBareName` parses the package's non-test files. It fails on any `exec.Command` or `exec.CommandContext` whose first argument is a string literal without `/`. This is a check for future sites, and it names the file and line.

## Mutants (each must fail on an assertion)

1. `run` back to `exec.Command(name, …)`: the look-alike marker appears in the new test.
2. `runner_install.go:410` back to `"systemctl"`: the marker appears (runner status case).
3. A new `exec.Command("ldd", …)` added: the AST guard names it.

## Readers

- W1B (Claude): the whole change, and whether the status probes keep their fallback semantics.
- W1C (Codex): the security read. Is any root exec left on PATH? Can a planted file in `toolDirs` win (no: those directories are root-owned)?

## ETA

- About 40 min after F7 lands: 10 for code, 20 for the test-helper move across 7 files, 10 for the new tests and mutants. Then ready.
