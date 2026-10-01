package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	bus "github.com/parf/ai-agent-bus/src/clients/go"
)

func testWorker(t *testing.T) *worker {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			fmt.Fprint(w, `{"you":"#go"}`)
		case "/kv/inc":
			fmt.Fprint(w, `{"value":1}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	b, err := bus.Dial(bus.Options{Addr: server.URL, ExpectedName: "#go"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	path := t.TempDir()
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	t.Setenv("AGENT_BUS_NAME", "#go")
	return &worker{b, root, path}
}

func TestInventoryExactBytesAndLines(t *testing.T) {
	w := testWorker(t)
	path := filepath.Join(w.rootPath, "notes")
	data := []byte("TODO\n\nlast TODO")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := w.inspect(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	if got.Bytes != len(data) || got.Lines != 3 || got.BlankLines != 1 || got.LongestLineBytes != 9 || got.TODOs != 2 || got.SHA256 != hex.EncodeToString(hash[:]) {
		t.Fatalf("wrong inventory: %+v", got)
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	got, err = w.inspect(context.Background(), path, false)
	if err != nil || got.Lines != 0 || got.BlankLines != 0 {
		t.Fatalf("empty file: %+v %v", got, err)
	}
}

func TestPathsStayUnderRoot(t *testing.T) {
	w := testWorker(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(w.rootPath, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative", outside, link, w.rootPath} {
		if _, err := w.inspect(context.Background(), path, false); err == nil {
			t.Errorf("accepted disallowed path %q", path)
		}
	}
}
