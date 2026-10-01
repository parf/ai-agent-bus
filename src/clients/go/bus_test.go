package bus

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestDialDiscoversAccountSocket(t *testing.T) {
	dir, e := os.MkdirTemp(os.Getenv("TMPDIR"), "ab-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	t.Setenv("XDG_RUNTIME_DIR", dir)
	u, e := user.Current()
	if e != nil {
		t.Fatal(e)
	}
	sock := filepath.Join(dir, "agent-bus", "user-"+u.Username+".sock")
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
	b, err := Dial(Options{})
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
