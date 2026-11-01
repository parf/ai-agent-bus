package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"github.com/parf/ai-agent-bus/internal/ports"
)

type overlayFile struct {
	name string
	body []byte
}

// shippedOverlays validates and computes the host's overrides of an agent's other shipped
// JSON files: runner/<agent>/<file>.json, then the instance's, each a JSON
// Merge Patch of the file the author ships. It never writes merged files.
// apply publishes the validated plan under runner/.merged/<key>, so
// the agent reads its file where it always did
// (Plans/R1.0-Release/runner-config.md#overriding-a-shipped-file). The
// digest names what was merged, so a changed override restarts a kept agent.
func shippedOverlays(runnerDir, agentDir, dir, key string) ([]ports.Overlay, string, []overlayFile, error) {
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
	merged := filepath.Join(runnerDir, ".merged", filepath.FromSlash(key), ".files")
	if len(names) == 0 {
		return nil, "", nil, nil
	}
	list := make([]string, 0, len(names))
	for n := range names {
		list = append(list, n)
	}
	sort.Strings(list)
	var over []ports.Overlay
	var files []overlayFile
	sum := sha256.New()
	for _, n := range list {
		shipped := filepath.Join(agentDir, dir, n)
		b, err := agentmeta.ReadFile(shipped)
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil, fmt.Errorf("runner/%s/%s overrides a file agent.d/%s does not ship", dir, n, dir)
		}
		if err != nil {
			return nil, "", nil, err
		}
		var doc any
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, "", nil, fmt.Errorf("agent.d/%s/%s is not JSON, so it is not overridden: %w", dir, n, err)
		}
		for _, layer := range layers {
			name := layer + "/" + n
			pb, err := agentmeta.ReadFile(filepath.Join(runnerDir, filepath.FromSlash(name)))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, "", nil, err
			}
			var patch any
			if err := json.Unmarshal(pb, &patch); err != nil {
				return nil, "", nil, fmt.Errorf("runner/%s: %w", name, err)
			}
			doc = mergePatch(doc, patch)
		}
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil, "", nil, err
		}
		out = append(out, '\n')
		src := filepath.Join(merged, n)
		files = append(files, overlayFile{name: n, body: out})
		over = append(over, ports.Overlay{Src: src, Dst: shipped})
		fmt.Fprintf(sum, "%s\x00%s\x00", n, out)
	}
	return over, fmt.Sprintf("%x", sum.Sum(nil)), files, nil
}

// recoverOverlays runs at real apply, never at install validation. Remove only
// our private interrupted output; restore the old tree if publication stopped
// between its two renames. This also covers keys now missing or refused.
func recoverOverlays(runnerDir string) error {
	root := filepath.Join(runnerDir, ".merged")
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".files.merge-") {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if name == ".files.previous" {
			target := filepath.Join(filepath.Dir(path), ".files")
			if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
				if err := os.Rename(path, target); err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else if err := os.RemoveAll(path); err != nil {
				return err
			}
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return nil
	})
}

// publishOverlays is called only by apply under reloadMu. Build every file
// before moving the old directory; a publication error restores the old tree.
// The sibling backup also recovers an interrupted two-rename replacement.
func publishOverlays(runnerDir, key string, files []overlayFile) error {
	target := filepath.Join(runnerDir, ".merged", filepath.FromSlash(key), ".files")
	return publishOverlayTree(target, files, os.Rename)
}

// The rename seam exercises rollback without relying on host permission errors.
func publishOverlayTree(target string, files []overlayFile, rename func(string, string) error) error {
	parent := filepath.Dir(target)
	backup := filepath.Join(parent, ".files.previous")
	if len(files) == 0 {
		return os.RemoveAll(target)
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".files.merge-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(stage, file.name), file.body, 0600); err != nil {
			return err
		}
	}
	hadOld := false
	if _, err := os.Lstat(target); err == nil {
		if err := rename(target, backup); err != nil {
			return err
		}
		hadOld = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := rename(stage, target); err != nil {
		if hadOld {
			if restore := rename(backup, target); restore != nil {
				return fmt.Errorf("publish overlays: %v; previous files retained at %s: %w", err, backup, restore)
			}
		}
		return err
	}
	return os.RemoveAll(backup)
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
