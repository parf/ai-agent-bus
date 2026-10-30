package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Deployment credentials live outside the file-call namespace (a leading dot
// cannot be an agent name). Only the host provisions these files. No ambient
// Git config, credential helper, SSH agent, askpass or repository hook is used.
func deployGitEnv(home, credentials string) []string {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	config := filepath.Join(credentials, "ssh_config")
	if _, err := os.Stat(config); os.IsNotExist(err) {
		config = "/dev/null"
	}
	ssh := "ssh -F " + quote(config) + " -oBatchMode=yes -oIdentitiesOnly=yes -oStrictHostKeyChecking=yes -oUserKnownHostsFile=" + quote(filepath.Join(credentials, "known_hosts")) + " -i " + quote(filepath.Join(credentials, "id"))
	return []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "XDG_CONFIG_HOME=" + home,
		"LANG=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/false", "SSH_ASKPASS=/bin/false", "GIT_ALLOW_PROTOCOL=https:ssh",
		"GIT_SSH_COMMAND=" + ssh, "GIT_SSH_VARIANT=ssh", "GIT_LFS_SKIP_SMUDGE=1"}
}

// fetchDeploy never resolves a symbolic revision. A fetch that returns some
// other commit (including a tag object) cannot silently change the reviewed
// code. Its caller supplies a private staging directory, not the live tree.
func (r *runner) fetchDeploy(ctx context.Context, in installRequest, dest string) error {
	if !deployCommit.MatchString(in.commit) {
		return fmt.Errorf("git deploy needs a full commit id")
	}
	if err := checkDeployURL(in.remote); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	home, err := os.MkdirTemp(filepath.Dir(dest), ".git-home-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(home)
	env := deployGitEnv(home, filepath.Join(r.host, ".deploy"))
	run := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "credential.helper=", "-c", "init.templateDir=", "-C", dest}, args...)...)
		cmd.Env = env
		// Stderr can contain remote URLs and host credentials. It is not returned,
		// logged, or copied to the bus. Only rev-parse produces captured output.
		if len(args) > 0 && args[0] == "rev-parse" {
			b, err := cmd.Output()
			return strings.TrimSpace(string(b)), err
		}
		return "", cmd.Run()
	}
	init := []string{"init", "--quiet"}
	if len(in.commit) == 64 {
		init = append(init, "--object-format=sha256")
	}
	for _, args := range [][]string{init, {"fetch", "--quiet", "--no-tags", "--depth=1", "--", in.remote, in.commit}, {"checkout", "--quiet", "--detach", "--force", "FETCH_HEAD"}} {
		if _, err := run(args...); err != nil {
			return fmt.Errorf("git deploy failed at %s; the host needs git, a route and any private-repository key in runner/.deploy", args[0])
		}
	}
	head, err := run("rev-parse", "--verify", "HEAD^{commit}")
	if err != nil || head != in.commit {
		return fmt.Errorf("git deploy HEAD does not equal the full requested commit")
	}
	if err := os.RemoveAll(filepath.Join(dest, ".git")); err != nil {
		return err
	}
	return nil
}
