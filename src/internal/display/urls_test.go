package display

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectHomepages(t *testing.T) {
	root := filepath.Join("..", "..")
	count := 0
	err := filepath.WalkDir(filepath.Join(root, "agents"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if d.Name() != "agent.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var meta struct{ Homepage string }
		if err = json.Unmarshal(data, &meta); err != nil {
			return err
		}
		count++
		if meta.Homepage != Homepage {
			t.Errorf("%s homepage = %q, want %q", path, meta.Homepage, Homepage)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no agent manifests checked")
	}
	for _, path := range []string{"clients/typescript/package.json", "clients/php/composer.json", "clients/python/pyproject.toml", "clients/rust/Cargo.toml", "clients/java/pom.xml"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), Homepage) {
			t.Errorf("%s: missing project homepage", path)
		}
		// A source repository URL may remain, but never as a homepage.
		for _, line := range strings.Split(string(data), "\n") {
			lower := strings.ToLower(line)
			if (strings.Contains(lower, "homepage") || strings.Contains(lower, "<url>")) && strings.Contains(line, Source) {
				t.Errorf("%s: source used as homepage: %s", path, line)
			}
		}
	}
	if SourceURL != Source+"/blob/main" {
		t.Fatal("documentation root is not derived from Source")
	}
	if Source == Homepage {
		t.Fatal("source and homepage must be distinct")
	}
}

func TestNoOldHomepageLiterals(t *testing.T) {
	for _, root := range []string{"../..", "../../../docs"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git") {
				return filepath.SkipDir
			}
			switch filepath.Ext(path) {
			case ".json", ".toml", ".xml", ".go", ".ts", ".tsx", ".md", ".py":
			default:
				return nil
			}
			if d.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for n, line := range strings.Split(string(data), "\n") {
				if strings.Contains(strings.ToLower(line), "homepage") && strings.Contains(line, Source) {
					t.Errorf("%s:%d: old source URL used as homepage", path, n+1)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
