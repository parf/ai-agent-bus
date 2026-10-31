package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
)

// Exercise reload itself, including strict decoding, agentFile.check and the
// metadata-only standalone exception. Copy only manifests: hashing node_modules
// or built programs would make this a package-build check instead of a fast gate.
func TestEveryBundledManifestPassesRunnerReload(t *testing.T) {
	root := filepath.Join("..", "..", "agents")
	bundled, host := t.TempDir(), t.TempDir()
	var manifests []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == "node_modules" || entry.Name() == ".git") {
			return fs.SkipDir
		}
		if entry.IsDir() || entry.Name() != "agent.json" {
			return nil
		}
		data, err := agentmeta.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(bundled, rel)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(destination, data, 0o644); err != nil {
			return err
		}
		manifests = append(manifests, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) == 0 {
		t.Fatal("no bundled agent.json found; nothing was validated")
	}
	r := runner{agents: t.TempDir(), bundled: bundled}
	sources := r.sources()
	loaded, err := loadSources(sources, host)
	if err != nil {
		t.Fatal(err)
	}
	checked := map[string]bool{}
	for _, in := range loaded {
		path := filepath.Join(in.Source, in.Dir, "agent.json")
		checked[path] = true
		if in.Refused != "" {
			rel, _ := filepath.Rel(bundled, path)
			t.Errorf("bundled %s refused by runner reload: %s", filepath.ToSlash(rel), in.Refused)
		}
	}
	for _, rel := range manifests {
		path := filepath.Join(bundled, rel)
		if checked[path] {
			continue
		}
		// Reload intentionally skips standalone service metadata, but that
		// exception must validate it too. Reuse the same predicate, not its rules.
		data, err := agentmeta.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var a agentFile
		if err := strictData("agent.json", data, &a); err != nil {
			t.Fatal(err)
		}
		metadata := false
		for _, source := range sources {
			if filepath.Dir(filepath.Dir(path)) == source.path && source.metadataOnly(data, a) {
				metadata = true
			}
		}
		if !metadata {
			t.Errorf("bundled %s was not validated by runner reload", filepath.ToSlash(rel))
		}
	}
	t.Logf("validated all %d bundled agent.json files through runner reload", len(manifests))
}
