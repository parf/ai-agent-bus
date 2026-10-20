package agentmeta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageMetadataDoesNotFollowAuthorLinks(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private")
	for _, file := range []string{"agent.json", "env.dist"} {
		t.Run(file, func(t *testing.T) {
			content := `{"description":"private-sentinel"}`
			if file == "env.dist" {
				content = "PRIVATE_SENTINEL=private-value\n"
			}
			if err := os.WriteFile(outside, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, file)
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
			defer os.Remove(path)
			var err error
			if file == "agent.json" {
				_, err = Load(dir)
			} else {
				_, err = (Manifest{Description: "fixture"}).Answer(dir, "help-json", nil, false)
			}
			if err == nil || !strings.Contains(err.Error(), file) || strings.Contains(err.Error(), "private-value") {
				t.Fatalf("linked author file not refused by name: %v", err)
			}
		})
	}
}
