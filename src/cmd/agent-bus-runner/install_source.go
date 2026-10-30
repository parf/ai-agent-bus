package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// The deploy call carries a source selector, never source bytes or a key.
// Fetching and publication use this checked selection; no revision resolver
// is allowed to reinterpret a branch, tag or short object name as a pin.
type installRequest struct {
	agent, remote, commit string
	replace               bool
}

var deployCommit = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)

func parseInstall(body string) (installRequest, error) {
	var in installRequest
	if strings.ContainsAny(body, "\r\n") {
		return in, fmt.Errorf("install takes one source line, never file content or credentials")
	}
	args := strings.Fields(body)
	if len(args) == 0 || args[0] != "install" {
		return in, fmt.Errorf("install <agent> [git <url> <full-commit>]")
	}
	args = args[1:]
	if len(args) > 0 && args[0] == "--replace" {
		in.replace, args = true, args[1:]
	}
	if len(args) != 1 && (len(args) != 4 || args[1] != "git") {
		return in, fmt.Errorf("install [--replace] <agent> [git <url> <full-commit>]")
	}
	in.agent = args[0]
	if !segment.MatchString(in.agent) {
		return in, fmt.Errorf("install agent must be one package name, not a path or instance")
	}
	if len(args) == 1 {
		return in, nil
	}
	if !deployCommit.MatchString(args[3]) {
		return in, fmt.Errorf("git deploy needs a full 40- or 64-hex commit id; branches, tags, abbreviated ids and HEAD are refused")
	}
	if err := checkDeployURL(args[2]); err != nil {
		return in, err
	}
	in.remote, in.commit = args[2], strings.ToLower(args[3])
	return in, nil
}

func checkDeployURL(remote string) error {
	// Do not echo a refused URL: userinfo or a query could hold a credential.
	refuse := fmt.Errorf("git deploy URL must be https:// or ssh:// with a host and repository path, without userinfo, query, fragment or controls; private access uses a host-provisioned key")
	if strings.IndexFunc(remote, func(c rune) bool { return unicode.IsControl(c) || unicode.IsSpace(c) }) >= 0 {
		return refuse
	}
	u, err := url.Parse(remote)
	if err != nil || (u.Scheme != "https" && u.Scheme != "ssh") || u.Opaque != "" || u.Hostname() == "" || strings.HasPrefix(u.Hostname(), "-") || u.Path == "" || u.Path == "/" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return refuse
	}
	if strings.IndexFunc(u.Path, unicode.IsControl) >= 0 {
		return refuse
	}
	return nil
}

// Resolve only the release/checkout's bundled roots, never an arbitrary host
// path or a package with the same name in the writable --agents directory.
func (r *runner) bundledInstallSource(name string) (string, error) {
	if !segment.MatchString(name) || r.bundled == "" {
		return "", fmt.Errorf("install needs a known bundled agent name")
	}
	for _, root := range []string{r.bundled, filepath.Join(r.bundled, "standalone")} {
		path := filepath.Join(root, name)
		fi, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !fi.IsDir() {
			return "", fmt.Errorf("bundled source must be a regular package directory")
		}
		var manifest agentFile
		if err := strict(filepath.Join(path, "agent.json"), &manifest); err != nil {
			return "", fmt.Errorf("bundled source requires a regular agent.json at the package root")
		}
		if _, err := manifest.check(); err != nil {
			return "", fmt.Errorf("bundled source agent.json: %w", err)
		}
		return path, nil
	}
	return "", fmt.Errorf("no bundled agent by that name")
}
