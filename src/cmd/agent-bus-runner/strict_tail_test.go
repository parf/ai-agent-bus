package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageMetadataRejectsTrailingJSON(t *testing.T) {
	for _, tail := range []string{"]", "}", "{}", "true"} {
		path := filepath.Join(t.TempDir(), "agent.json")
		if err := os.WriteFile(path, []byte(`{"description":"fixture","command":"true"}`+tail), 0600); err != nil {
			t.Fatal(err)
		}
		var agent agentFile
		if err := strict(path, &agent); err == nil || !strings.Contains(err.Error(), "agent.json") {
			t.Fatalf("trailing %q accepted or unnamed: %v", tail, err)
		}
	}
}
