package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"github.com/parf/ai-agent-bus/internal/ports"
)

// shippedOverlays builds the host's overrides of an agent's other shipped
// JSON files: runner/<agent>/<file>.json, then the instance's, each a JSON
// Merge Patch of the file the author ships. The merged files are written
// under runner/.merged/<key> and mounted read-only over the shipped path, so
// the agent reads its file where it always did
// (Plans/R1.0-Release/runner-config.md#overriding-a-shipped-file). The
// digest names what was merged, so a changed override restarts a kept agent.
func shippedOverlays(runnerDir, agentDir, dir, key string) ([]ports.Overlay, string, error) {
	layers := []string{dir}
	if _, instance, ok := strings.Cut(key, "/"); ok {
		layers = append(layers, dir+"/"+instance)
	}
	names := map[string]bool{}
	for _, layer := range layers {
		entries, err := os.ReadDir(filepath.Join(runnerDir, filepath.FromSlash(layer)))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if n := e.Name(); !e.IsDir() && n != "agent.json" && strings.HasSuffix(n, ".json") {
				names[n] = true
			}
		}
	}
	merged := filepath.Join(runnerDir, ".merged", filepath.FromSlash(key))
	if err := os.RemoveAll(merged); err != nil {
		return nil, "", err
	}
	if len(names) == 0 {
		return nil, "", nil
	}
	list := make([]string, 0, len(names))
	for n := range names {
		list = append(list, n)
	}
	sort.Strings(list)
	var over []ports.Overlay
	sum := sha256.New()
	for _, n := range list {
		shipped := filepath.Join(agentDir, dir, n)
		b, err := agentmeta.ReadFile(shipped)
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", fmt.Errorf("runner/%s/%s overrides a file agent.d/%s does not ship", dir, n, dir)
		}
		if err != nil {
			return nil, "", err
		}
		var doc any
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, "", fmt.Errorf("agent.d/%s/%s is not JSON, so it is not overridden: %w", dir, n, err)
		}
		for _, layer := range layers {
			name := layer + "/" + n
			pb, err := agentmeta.ReadFile(filepath.Join(runnerDir, filepath.FromSlash(name)))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, "", err
			}
			var patch any
			if err := json.Unmarshal(pb, &patch); err != nil {
				return nil, "", fmt.Errorf("runner/%s: %w", name, err)
			}
			doc = mergePatch(doc, patch)
		}
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil, "", err
		}
		out = append(out, '\n')
		if err := os.MkdirAll(merged, 0700); err != nil {
			return nil, "", err
		}
		src := filepath.Join(merged, n)
		if err := os.WriteFile(src, out, 0600); err != nil {
			return nil, "", err
		}
		over = append(over, ports.Overlay{Src: src, Dst: shipped})
		fmt.Fprintf(sum, "%s\x00%s\x00", n, out)
	}
	return over, fmt.Sprintf("%x", sum.Sum(nil)), nil
}

// mergePatch is RFC 7386: an object patch merges into an object, a null
// removes the key, anything else replaces whole.
func mergePatch(target, patch any) any {
	p, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	t, ok := target.(map[string]any)
	if !ok {
		t = map[string]any{}
	}
	for k, v := range p {
		if v == nil {
			delete(t, k)
		} else {
			t[k] = mergePatch(t[k], v)
		}
	}
	return t
}
