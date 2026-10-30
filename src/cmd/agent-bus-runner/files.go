package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The host's files over the bus (M.5): `read <file>` answers one of them,
// `write <file>` with the whole new content after its first line replaces one
// and applies it as reload does, and `create` and `delete <agent>/<instance>`
// add and remove an instance. A file is named from the runner's home, as
// configuration.md lists them; nothing else there may be named, so a call can
// never reach the kept tokens or an agent's work. The author's files are read
// only: what the host decides is in runner/.

// fileAnswer is what `read` answers: the file, and whether it is there, so an
// absent file is not mistaken for an empty one.
type fileAnswer struct {
	File    string    `json:"file"`
	Exists  bool      `json:"exists"`
	Content *string   `json:"content,omitempty"`
	SHA256  string    `json:"sha256,omitempty"`
	Size    *int      `json:"size,omitempty"`
	Names   *[]string `json:"names,omitempty"`
}

// A directory or instance name: what the runner itself makes keys and paths of.
var segment = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,63}$`)

// Inventory keys are one package segment and at most one instance segment.
func checkAgentKey(key string) error {
	parts := strings.Split(key, "/")
	if len(parts) > 2 {
		return fmt.Errorf("agents.json: invalid agent key %q", key)
	}
	for _, part := range parts {
		if !segment.MatchString(part) {
			return fmt.Errorf("agents.json: invalid agent key %q", key)
		}
	}
	return nil
}

// The kinds of file a call may name.
const (
	fileAuthor   = "author"   // agent.d/<agent>/agent.json or env.dist, read only
	fileEnv      = "env"      // runner/<agent>[/<instance>]/env
	fileOverride = "override" // runner/<agent>[/<instance>]/agent.json
	fileAgents   = "agents"   // runner/agents.json
	fileSettings = "settings" // runner/<agent>[/<instance>]/<file>.json, a merge patch of a file the agent ships
)

// hostFile is the path and kind of a file a call names, or why it may not
// name it.
func (r *runner) hostFile(name string) (string, string, error) {
	parts := strings.Split(name, "/")
	refuse := fmt.Errorf("%s is not a file the runner reads or writes: agent.d/<agent>/agent.json or env.dist, runner/agents.json, runner/<agent>[/<instance>]/env, agent.json or <file>.json", name)
	if len(parts) < 2 || len(parts) > 4 {
		return "", "", refuse
	}
	for _, p := range parts[1 : len(parts)-1] {
		if !segment.MatchString(p) || p == "env" || p == "agent.json" {
			return "", "", refuse
		}
	}
	base, last := parts[0], parts[len(parts)-1]
	switch {
	case base == "agent.d" && len(parts) == 3 && (last == "agent.json" || last == "env.dist"):
		if err := r.isInstalled(parts[1]); err != nil {
			return "", "", err
		}
		root, err := r.packageRoot(parts[1])
		if err != nil {
			return "", "", err
		}
		return filepath.Join(root, parts[1], last), fileAuthor, r.noLinks(root, parts[1:]...)
	case base == "runner" && len(parts) == 2 && last == "agents.json":
		return filepath.Join(r.host, last), fileAgents, r.noLinks(r.host, last)
	case base == "runner" && len(parts) >= 3 && (last == "env" || last == "agent.json"):
		if err := r.hostDir(parts[1]); err != nil {
			return "", "", err
		}
		kind := fileEnv
		if last == "agent.json" {
			kind = fileOverride
		}
		return filepath.Join(append([]string{r.host}, parts[1:]...)...), kind, r.noLinks(r.host, parts[1:]...)
	case base == "runner" && len(parts) >= 3 && strings.HasSuffix(last, ".json") && segment.MatchString(last):
		if err := r.hostDir(parts[1]); err != nil {
			return "", "", err
		}
		return filepath.Join(append([]string{r.host}, parts[1:]...)...), fileSettings, r.noLinks(r.host, parts[1:]...)
	}
	return "", "", refuse
}

// hostDir is isInstalled for a path under runner/, where tokens and work are
// the runner's own directories: an agent of either name has no host files.
func (r *runner) hostDir(dir string) error {
	if dir == "tokens" || dir == "work" {
		return fmt.Errorf("%s is the runner's own directory under runner/, not an agent's", dir)
	}
	return r.isInstalled(dir)
}

// isInstalled says whether dir is an agent's directory under agent.d.
func (r *runner) isInstalled(dir string) error {
	// Checked before it is joined to a path: "." and ".." would name agent.d
	// itself and its parent, both directories.
	if !segment.MatchString(dir) {
		return fmt.Errorf("%q is not an agent's directory name", dir)
	}
	_, err := r.packageRoot(dir)
	return err
}

// noLinks refuses a path through a symbolic link under base: a link placed
// there could point a write anywhere the runner's account may write.
func (r *runner) noLinks(base string, parts ...string) error {
	p := base
	for _, part := range parts {
		p = filepath.Join(p, part)
		fi, err := os.Lstat(p)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symbolic link; the runner does not follow one", p)
		}
	}
	return nil
}

// readFile answers `read <file>`.
func (r *runner) readFile(name string) (fileAnswer, error) {
	path, kind, err := r.hostFile(name)
	if err != nil {
		return fileAnswer{}, err
	}
	b, err := agentmeta.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if kind == fileEnv {
			size, names := 0, []string{}
			return fileAnswer{File: name, Size: &size, Names: &names}, nil
		}
		empty := ""
		return fileAnswer{File: name, Content: &empty}, nil
	}
	if err != nil {
		return fileAnswer{}, err
	}
	if kind == fileEnv {
		vars, err := parseEnv(name, b)
		if err != nil {
			return fileAnswer{}, err
		}
		names, seen := []string{}, map[string]bool{}
		for _, v := range vars {
			if !seen[v.Key] {
				names = append(names, v.Key)
				seen[v.Key] = true
			}
		}
		size := len(b)
		return fileAnswer{File: name, Exists: true, SHA256: fmt.Sprintf("%x", sha256.Sum256(b)), Size: &size, Names: &names}, nil
	}
	content := string(b)
	return fileAnswer{File: name, Exists: true, Content: &content}, nil
}

// writeHostFile answers `write <file>`: the content is checked as the runner
// will read it, so a call cannot leave a file that refuses its agent, or the
// whole load, without saying so first; then it replaces the file and applies.
func (r *runner) writeHostFile(name, content string) error {
	path, kind, err := r.hostFile(name)
	if err != nil {
		return err
	}
	switch kind {
	case fileAuthor:
		return fmt.Errorf("%s is the author's, read only: the host decides in runner/", name)
	case fileEnv:
		if _, err := parseEnv(name, []byte(content)); err != nil {
			return err
		}
	case fileAgents:
		if err := checkAgents([]byte(content)); err != nil {
			return err
		}
	case fileOverride:
		if _, err := parseOverride(name, []byte(content)); err != nil {
			return err
		}
	case fileSettings:
		if err := r.checkSettingsOverride(name, []byte(content)); err != nil {
			return err
		}
	}
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	if err := replaceFile(path, []byte(content)); err != nil {
		return err
	}
	r.apply()
	return nil
}

// checkAgents is what load and serve would refuse of an agents.json, said
// before it is written.
func checkAgents(b []byte) error {
	if t := bytes.TrimSpace(b); len(t) == 0 || t[0] != '{' {
		return errors.New("agents.json: an object of entries, {} for none")
	}
	host := map[string]hostEntry{}
	if err := strictData("agents.json", b, &host); err != nil {
		return err
	}
	for key, h := range host {
		if err := checkAgentKey(key); err != nil {
			return err
		}
		switch h.Sandbox {
		case "", "on", "off":
		default:
			return fmt.Errorf("agents.json: %s: sandbox %q is on or off", key, h.Sandbox)
		}
		if _, why := limitsFor(installed{Key: key, Host: h}); why != "" {
			return fmt.Errorf("%s: %s", key, why)
		}
	}
	return nil
}

// The author's options a host may override, in runner/<agent>[/<instance>]/agent.json
// (Plans/R1.0-Release/runner-config.md#overriding-a-shipped-file). The rest
// is what the code is.
var overridable = map[string]bool{"processes": true, "idle": true, "deadline": true, "depends": true}

// parseOverride reads an override of agent.json: a JSON Merge Patch naming
// only what a host may change.
func parseOverride(name string, b []byte) (map[string]json.RawMessage, error) {
	patch := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &patch); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	for k, v := range patch {
		if !overridable[k] {
			return nil, fmt.Errorf("%s: %s may not be overridden; a host overrides processes, idle, deadline and depends", name, k)
		}
		if string(v) == "null" {
			continue
		}
		switch k {
		case "processes":
			if _, err := parseSpan(v); err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
		case "idle", "deadline":
			var s string
			if json.Unmarshal(v, &s) != nil {
				return nil, fmt.Errorf("%s: %s is a duration such as 30s", name, k)
			}
			if t, err := time.ParseDuration(s); err != nil || t <= 0 {
				return nil, fmt.Errorf("%s: %s %q is not a positive duration such as 30s", name, k, s)
			}
		case "depends":
			var l []string
			if json.Unmarshal(v, &l) != nil {
				return nil, fmt.Errorf("%s: depends is a list of names", name)
			}
		}
	}
	return patch, nil
}

// createInstance answers `create <agent>/<instance>`: its agents.json entry
// and its directory with an empty env, then applied.
func (r *runner) createInstance(key string) error {
	dir, instance, err := r.instanceKey(key)
	if err != nil {
		return err
	}
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	entries, err := readAgents(r.host)
	if err != nil {
		return err
	}
	if _, ok := entries[key]; ok {
		return fmt.Errorf("%s is already an instance here", key)
	}
	entries[key] = map[string]json.RawMessage{"enabled": json.RawMessage("true")}
	home := filepath.Join(r.host, dir, instance)
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	if f, err := os.OpenFile(filepath.Join(home, "env"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err == nil {
		f.Close()
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	if err := writeJSON(filepath.Join(r.host, "agents.json"), entries); err != nil {
		return err
	}
	r.apply()
	return nil
}

// deleteInstance answers `delete <agent>/<instance>`: its entry and its host
// files go, then applied. Its work directory, its kept token and its bus
// record stay: they are data and a name, not configuration.
func (r *runner) deleteInstance(key string) error {
	dir, instance, err := r.instanceKey(key)
	if err != nil {
		return err
	}
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	entries, err := readAgents(r.host)
	if err != nil {
		return err
	}
	if _, ok := entries[key]; !ok {
		return fmt.Errorf("%s is not an instance here", key)
	}
	// Its files first and its entry last: a failure between leaves an entry
	// a retry still finds, never files no entry names.
	home := filepath.Join(r.host, dir, instance)
	files := []string{"env"}
	if list, _ := filepath.Glob(filepath.Join(home, "*.json")); len(list) > 0 {
		for _, f := range list {
			files = append(files, filepath.Base(f))
		}
	}
	for _, f := range files {
		if err := os.Remove(filepath.Join(home, f)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	os.Remove(home) // only if nothing else is left in it
	delete(entries, key)
	if err := writeJSON(filepath.Join(r.host, "agents.json"), entries); err != nil {
		return err
	}
	delete(r.stopped, key)
	r.apply()
	return nil
}

// instanceKey splits <agent>/<instance>, an installed agent and a name the
// runner can make a directory of.
func (r *runner) instanceKey(key string) (string, string, error) {
	dir, instance, ok := strings.Cut(key, "/")
	if !ok || !segment.MatchString(instance) || instance == "env" || instance == "agent.json" {
		return "", "", fmt.Errorf("%s is not <agent>/<instance>, an instance a letter, digit or _ then up to 63 of those, . or -", key)
	}
	if err := r.hostDir(dir); err != nil {
		return "", "", err
	}
	if err := r.noLinks(r.host, dir, instance); err != nil {
		return "", "", err
	}
	return dir, instance, nil
}

// readAgents is runner/agents.json as entries of raw fields, so a write keeps
// every field it does not change as it was.
func readAgents(runnerDir string) (map[string]map[string]json.RawMessage, error) {
	entries := map[string]map[string]json.RawMessage{}
	b, err := agentmeta.ReadFile(filepath.Join(runnerDir, "agents.json"))
	if errors.Is(err, os.ErrNotExist) {
		return entries, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, fmt.Errorf("agents.json: %w", err)
	}
	if entries == nil { // the file says null
		entries = map[string]map[string]json.RawMessage{}
	}
	for k, v := range entries {
		if v == nil {
			entries[k] = map[string]json.RawMessage{}
		}
	}
	return entries, nil
}

// checkSettingsOverride is what reload would refuse of an override of a
// shipped settings file, said before it is written: a JSON object, naming a
// file the agent ships.
func (r *runner) checkSettingsOverride(name string, content []byte) error {
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(content, &patch); err != nil {
		return fmt.Errorf("%s: a JSON object naming only what changes: %w", name, err)
	}
	if patch == nil { // JSON null unmarshals to a nil map without an error
		return fmt.Errorf("%s: a JSON object naming only what changes, not null", name)
	}
	parts := strings.Split(name, "/")
	last := parts[len(parts)-1]
	root, err := r.packageRoot(parts[1])
	if err != nil {
		return err
	}
	b, err := agentmeta.ReadFile(filepath.Join(root, parts[1], last))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s overrides a file agent.d/%s does not ship", name, parts[1])
	}
	if err != nil {
		return err
	}
	var shipped any
	if err := json.Unmarshal(b, &shipped); err != nil {
		return fmt.Errorf("agent.d/%s/%s is not JSON, so it is not overridden", parts[1], last)
	}
	return nil
}
