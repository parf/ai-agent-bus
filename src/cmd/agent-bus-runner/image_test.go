package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
)

// TestImageCheckedAtPackageLoad: the runner refuses a package by name when its
// image breaks a rule, and answers the image ask of one that keeps them.
func TestImageCheckedAtPackageLoad(t *testing.T) {
	vectors := filepath.Join("..", "..", "testdata", "agent-images")
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(vectors, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	home := t.TempDir()
	agents := filepath.Join(home, "agent.d")
	writeFile(t, filepath.Join(agents, "pictured", "agent.json"), `{"command":"true","description":"Has a picture.","image":"agent.avif"}`)
	writeFile(t, filepath.Join(agents, "pictured", "agent.avif"), read("a.avif"))
	writeFile(t, filepath.Join(agents, "wide", "agent.json"), `{"command":"true","description":"Has a wide picture.","image":"agent.png"}`)
	writeFile(t, filepath.Join(agents, "wide", "agent.png"), read("wide.png"))
	writeFile(t, filepath.Join(agents, "escapes", "agent.json"), `{"command":"true","description":"Points outside.","image":"../pictured/agent.avif"}`)
	list, err := load(agents, filepath.Join(home, "runner"))
	if err != nil {
		t.Fatal(err)
	}
	refused := map[string]string{}
	var pictured installed
	for _, in := range list {
		refused[in.Key] = in.Refused
		if in.Key == "pictured" {
			pictured = in
		}
	}
	if refused["pictured"] != "" {
		t.Fatalf("a valid image refuses the package: %s", refused["pictured"])
	}
	if !strings.Contains(refused["wide"], `image "agent.png"`) || !strings.Contains(refused["wide"], "not square") {
		t.Fatalf("a non-square image is refused by name: %q", refused["wide"])
	}
	if !strings.Contains(refused["escapes"], ". or .. parts") {
		t.Fatalf("an image outside the package is refused: %q", refused["escapes"])
	}
	answer, handled, err := agentmeta.DispatchAsk("image", agentmeta.Manifest(pictured.Agent).AskHandlers((&runner{}).codeDir(pictured), nil, "#pictured"))
	if err != nil || !handled {
		t.Fatalf("image ask: handled=%v err=%v", handled, err)
	}
	if got, _ := base64.StdEncoding.DecodeString(answer); string(got) != read("a.avif") {
		t.Fatalf("the runner answers other bytes for the image ask (%d)", len(got))
	}
}
