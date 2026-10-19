package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Socket fixtures cannot inherit TMPDIR: deep checkouts exceed sun_path.
func shortSocketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ab-go-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return dir
}

func listenTestUnix(t *testing.T, path string) (net.Listener, error) {
	t.Helper()
	// 103 bytes also fits BSD/macOS, whose sun_path is shorter than Linux’s.
	const maxUnixSocketPath = 103
	if len(path) > maxUnixSocketPath {
		t.Fatalf("Unix socket fixture path exceeds portable sun_path limit: %d bytes, maximum %d: %s", len(path), maxUnixSocketPath, path)
	}
	return net.Listen("unix", path)
}

func TestUnixSocketFixtureIgnoresLongTMPDIR(t *testing.T) {
	deep := filepath.Join(t.TempDir(), strings.Repeat("deep", 30))
	if err := os.MkdirAll(deep, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", deep)
	listener, err := listenTestUnix(t, filepath.Join(shortSocketDir(t), "fixture.sock"))
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()
}
