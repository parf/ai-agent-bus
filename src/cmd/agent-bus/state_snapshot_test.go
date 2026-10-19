package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A reader may already have opened the old file when another invocation
// publishes a replacement. Finish that read after publication: it must still
// be the complete old JSON, while a fresh open gets the complete new JSON.
func TestCLIStateReadersKeepOneCompleteSnapshot(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	savedWho := whoami
	whoami = func() string { return "fixture@host" }
	defer func() { whoami = savedWho }()
	for _, kind := range []string{"reply context", "running note"} {
		t.Run(kind, func(t *testing.T) {
			var path string
			var write func(string)
			if kind == "reply context" {
				path = filepath.Join(stateDir(), "message.json")
				write = func(value string) { remember(protocol.Envelope{ID: "message", From: value, Topic: "topic"}) }
			} else {
				path = notePath("#fixture@host")
				write = func(value string) {
					if err := (running{Name: "#fixture@host", PID: os.Getpid(), Script: value}).note(); err != nil {
						t.Fatal(err)
					}
				}
			}
			write("old command")
			old, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			prefix := make([]byte, 12)
			if _, err := io.ReadFull(reader, prefix); err != nil {
				t.Fatal(err)
			}
			write("new command with a different length")
			rest, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(append(prefix, rest...), old) {
				t.Fatal("reader saw mixed snapshots across a state replacement")
			}
			fresh, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(fresh, old) || !bytes.Contains(fresh, []byte("new command")) {
				t.Fatal("fresh reader did not see the complete new snapshot")
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0600 {
				t.Fatalf("state permissions %o, want 600", info.Mode().Perm())
			}
		})
	}
}
