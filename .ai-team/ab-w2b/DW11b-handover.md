# DW11b handover (#ab-w2b -> #ab-w1c), 2026-10-30 20:30

## Branch
ab-w2b/DW11b, worktree tmp/ab-w2b-DW11b (kept for you; remove after landing), tip 34fb1d24, based on main 49687ec5. Scratch: tmp/dw11b-run*.log, tmp/dw11b-bin (host-built agent-busd and agent-bus for the PHP test), tmp/scripts/w2b-dw11b-php-mut.sh.

## Part 2, PHP example: done
src/clients/php/tests/run.php section "the README example, run as written": extracts the two php blocks and the printed output from src/clients/php/README.md "Try this first", runs them as two identities against the throwaway daemon with T-14's flags, asserts exact output and empty stderr. 3 README mutants (call without topic, changed output, changed serve answer) fail on the output assertion. Commit 6a79f6db. Not yet in the plan's TODO/DONE: DW11's state line still says it was not run; update it when you land.

## Part 1, INSTALL.md container check: install block passes, the rest not proven
Files: src/acceptance/install-md-container.sh (driver; podman systemd container, no network, rootless private store, takes the smoke-gate lock via guard.sh), install-md.Containerfile, install_md_test.py (in-container; reads the Install, Check, Try it and Update blocks out of INSTALL.md, runs each as user dev with sudo, as printed). The doc's git clone URL is redirected to this commit with a system git url.insteadOf; Go (host GOROOT, GOMODCACHE ro, private GOCACHE), Bun and bun's cache come from the host.
- test_1 Install block: PASSES (git clone, bun install, src/build.sh, sudo agent-bus-setup, sudo git-install.sh --no-build).
- test_2 Check block: FAILS, reason not seen (the unittest message only prints at the end and both runs were killed first). Suspects: agent-bus-web inactive in the container, or the output order of the two systemctl lines; run with failfast (-f after the doc path) to see it at once.
- test_3 Try-it (agent-bus run hi + call): HUNG in runs 2 and 3, reason not seen (never printed).
- test_4 Update block: never reached; its fixture pushes a newer commit to /source.git (rw mount).
- Note: the lead asked for "agent-bus version", the verb does not exist; the test uses agent-bus --version and checks build_info.

## Why test 3 hung, and how far bounding got
Run 1/2: calls used --wait 10s inside subprocess.run(timeout=60), 30 tries, so up to 30 min. Run 3 cut that to 6 tries with 40 s timeouts and still hung 19 min: subprocess.run(timeout) kills only the direct child (runuser); a grandchild (agent-bus call or the served script) keeps the output pipe open, so communicate() never returns. Fix is committed (last commit on the branch, NOT yet run): as_user() now uses Popen with start_new_session and killpg on timeout, and the driver wraps the container exec in timeout --kill-after=20 1200. Next step: take the lock, run bash src/acceptance/install-md-container.sh INSTALL.md -f and read the first failure. Real mutant still to do: break one INSTALL.md command in a copy (e.g. src/build.shx) and pass it as the driver's first argument; the install test must fail on its assertion.

## J62
Entry in Plans/issues/2026-10/2026-10-30.md on this branch (under "Gate flakes"): guard-escape.test.sh "no watcher outlived the kill" failed once in the P.1c gate run (tmp/gate-logs/20261030-194439-ab-w2b_P1c.log); owner #ab-w1b, starting about 20:10 on 2026-10-30; likely cause (not reproduced) is the pgrep 3 s after the kill. The same file has the J60b entry I wrote on ab-w2b/J60b: that one is a separate branch; expect a trivial merge conflict on that file when both land.
