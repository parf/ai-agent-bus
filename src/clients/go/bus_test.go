package bus

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestDialDiscoversAccountSocket(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	sock := filepath.Join(dir, "agent-bus", "user-test.sock")
	if err := os.MkdirAll(filepath.Dir(sock), 0o755); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"you":"test@h"}`))
	})}
	go srv.Serve(ln)
	defer srv.Close()
	b, err := Dial(Options{Addr: sock})
	if err != nil {
		t.Fatal(err)
	}
	st, err := b.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.You != "test@h" {
		t.Fatalf("identity %q, want test@h", st.You)
	}
}
