package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
)

// What an agent's author ships in agent.d/<agent>/agent.json, and what the
// host decides in runner/agents.json: the approved formats of
// Plans/R1.0-Release/runner-config.md#file-formats. Every field but `command` and `description`
// is optional, and an unknown one is refused, by name, at load.
type agentFile agentmeta.Manifest

type needs = agentmeta.Needs

type hostEntry struct {
	Tags         []string `json:"tags,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty"`
	Name         string   `json:"name,omitempty"`
	Share        bool     `json:"share,omitempty"`
	Sandbox      string   `json:"sandbox,omitempty"`
	Limits       *limits  `json:"limits,omitempty"`
	Version      string   `json:"version,omitempty"`
	Origin       *origin  `json:"origin,omitempty"`
	FirstStarted string   `json:"first_started,omitempty"`
	LastStarted  string   `json:"last_started,omitempty"`
}

type limits struct {
	Memory    string `json:"memory,omitempty"`
	CPU       string `json:"cpu,omitempty"`
	Processes int    `json:"processes,omitempty"`
	Files     int    `json:"files,omitempty"`
	IOWeight  int    `json:"io_weight,omitempty"`
	Disk      string `json:"disk,omitempty"`
}

type origin struct {
	Remote string `json:"remote,omitempty"`
	Commit string `json:"commit,omitempty"`
}

// span is `processes`: a fixed count, or min..max.
type span struct{ Min, Max int }

// installed is one agent or instance as this load found it: what it is, and
// whether it may be served. A refused one carries why, and is never skipped
// silently.
type installed struct {
	Key          string // <agent> or <agent>/<instance>, the agents.json key
	Dir          string // the directory under agent.d
	Digest       string // installed package files at load
	Agent        agentFile
	Host         hostEntry
	Span         span
	Refused      string
	LastExit     *exitStatus // latest actual exit observed since load
	Unconfigured bool        // exit78, cleared only by a fresh load
	Stopped      bool        // by a stop call, until reload or the runner's restart
}

func (in installed) enabled() bool { return in.Host.Enabled == nil || *in.Host.Enabled }

// wanted is whether serve should run it, before serve's own preflight.
func (in installed) wanted() bool {
	return in.Refused == "" && !in.Unconfigured && !in.Stopped && in.enabled()
}

// strict decodes one JSON value and refuses an unknown field or anything
// after it, naming the file.
func strict(path string, into any) error {
	b, err := agentmeta.ReadFile(path)
	if err != nil {
		return err
	}
	return strictData(filepath.Base(path), b, into)
}

func strictData(path string, b []byte, into any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(into); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("%s: more than one JSON value", filepath.Base(path))
	}
	return nil
}

func parseSpan(raw json.RawMessage) (span, error) {
	if len(raw) == 0 {
		return span{0, 1}, nil
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		if n < 1 {
			return span{}, fmt.Errorf("processes %d: a count is at least 1", n)
		}
		return span{n, n}, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return span{}, fmt.Errorf("processes is a number or \"min..max\"")
	}
	lo, hi, ok := strings.Cut(s, "..")
	a, errA := strconv.Atoi(lo)
	b, errB := strconv.Atoi(hi)
	if !ok || errA != nil || errB != nil || a < 0 || b < 1 || a > b {
		return span{}, fmt.Errorf("processes %q is not min..max with 0 <= min <= max, max >= 1", s)
	}
	return span{a, b}, nil
}

// check is what an agent.json must be beyond its fields' types.
func (a agentFile) check() (span, error) {
	if strings.TrimSpace(a.Command) == "" {
		return span{}, errors.New("agent.json: command is required")
	}
	if err := agentmeta.Manifest(a).Check(); err != nil {
		return span{}, err
	}
	if strings.TrimSpace(a.Description) == "" {
		return span{}, errors.New("agent.json: description is required")
	}
	switch a.Form {
	case "", "args", "json", "std", "jsonl", "msgpack":
	default:
		return span{}, fmt.Errorf("agent.json: form %q is not args, json, std, jsonl or msgpack", a.Form)
	}
	switch a.Serves {
	case "", "runner", "itself":
	default:
		return span{}, fmt.Errorf("agent.json: serves %q is not runner or itself", a.Serves)
	}
	for _, d := range []struct{ field, v string }{{"deadline", a.Deadline}, {"idle", a.Idle}} {
		if d.v == "" {
			continue
		}
		if t, err := time.ParseDuration(d.v); err != nil || t <= 0 {
			return span{}, fmt.Errorf("agent.json: %s %q is not a positive duration such as 30s", d.field, d.v)
		}
	}
	if a.InFlight < 0 {
		return span{}, fmt.Errorf("agent.json: in_flight %d is below 1", a.InFlight)
	}
	sp, err := parseSpan(a.Processes)
	if err != nil {
		return span{}, fmt.Errorf("agent.json: %w", err)
	}
	return sp, nil
}

// load reads what is installed under agent.d and what the host decided in
// runner/agents.json. A bad agents.json refuses the whole load, since every
// agent's host decisions are in it; a bad agent.json refuses only its agent.
// An agents.json entry naming no installed agent is refused too, by name: a
// typo left alone is an agent that never starts and looks fine.
func load(agentDir, runnerDir string) ([]installed, error) {
	host := map[string]hostEntry{}
	if err := strict(filepath.Join(runnerDir, "agents.json"), &host); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for key := range host {
		if err := checkAgentKey(key); err != nil {
			return nil, err
		}
	}
	entries, err := os.ReadDir(agentDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var out []installed
	seen := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := e.Name()
		var a agentFile
		refused := ""
		sp := span{}
		author, _ := agentmeta.ReadFile(filepath.Join(agentDir, dir, "agent.json"))
		if err := strict(filepath.Join(agentDir, dir, "agent.json"), &a); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				refused = "agent.json: missing"
			} else {
				refused = err.Error()
			}
		} else if sp, err = a.check(); err != nil {
			refused = err.Error()
		}
		digest := ""
		if refused == "" {
			digest, err = packageDigest(filepath.Join(agentDir, dir))
			if err != nil {
				refused = err.Error()
			}
		}
		// The agent itself, and each instance the host configured for it.
		keys := []string{}
		for k := range host {
			if k == dir || strings.HasPrefix(k, dir+"/") {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 {
			keys = []string{dir}
		}
		sort.Strings(keys)
		for _, k := range keys {
			seen[k] = true
			in := installed{Key: k, Dir: dir, Digest: digest, Agent: a, Host: host[k], Span: sp, Refused: refused}
			if refused == "" {
				if o, osp, ok, err := overridden(runnerDir, dir, k, author); err != nil {
					in.Refused = err.Error()
				} else if ok {
					in.Agent, in.Span = o, osp
				}
			}
			out = append(out, in)
		}
	}
	for k := range host {
		if !seen[k] {
			out = append(out, installed{Key: k, Refused: "agents.json: no agent " + strings.SplitN(k, "/", 2)[0] + " is installed in agent.d"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return orderDepends(out), nil
}

// overridden is the author's agent.json with the host's overrides applied, in
// layer order: runner/<agent>/agent.json, then the instance's. Each is a JSON
// Merge Patch of what a host may change; `null` removes a key, so it falls
// back to the built default (Plans/R1.0-Release/runner-config.md#overriding-a-shipped-file).
// ok is false when there is no override.
func overridden(runnerDir, dir, key string, author []byte) (agentFile, span, bool, error) {
	names := []string{dir + "/agent.json"}
	if _, instance, isInstance := strings.Cut(key, "/"); isInstance {
		names = append(names, dir+"/"+instance+"/agent.json")
	}
	var doc map[string]json.RawMessage
	for _, name := range names {
		b, err := agentmeta.ReadFile(filepath.Join(runnerDir, filepath.FromSlash(name)))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return agentFile{}, span{}, false, err
		}
		patch, err := parseOverride("runner/"+name, b)
		if err != nil {
			return agentFile{}, span{}, false, err
		}
		if doc == nil {
			if err := json.Unmarshal(author, &doc); err != nil {
				return agentFile{}, span{}, false, err
			}
		}
		for k, v := range patch {
			if string(v) == "null" {
				delete(doc, k)
			} else {
				doc[k] = v
			}
		}
	}
	if doc == nil {
		return agentFile{}, span{}, false, nil
	}
	var a agentFile
	d := json.NewDecoder(bytes.NewReader(mustJSON(doc)))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return agentFile{}, span{}, false, fmt.Errorf("agent.json with its overrides: %w", err)
	}
	sp, err := a.check()
	return a, sp, true, err
}
