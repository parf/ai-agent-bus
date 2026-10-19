package bus

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptionsNeverPrintTheToken(t *testing.T) {
	o := Options{Addr: "/run/x.sock", Token: "s3cret-token"}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		if shown := fmt.Sprintf(verb, o); strings.Contains(shown, o.Token) || !strings.Contains(shown, "[redacted]") {
			t.Errorf("%s prints %q", verb, shown)
		}
	}
}

func TestDialDiscoversAccountSocket(t *testing.T) {
	dir := shortSocketDir(t)
	t.Setenv("XDG_RUNTIME_DIR", dir)
	u, e := user.Current()
	if e != nil {
		t.Fatal(e)
	}
	sock := filepath.Join(dir, "agent-bus", "user-"+u.Username+".sock")
	if err := os.MkdirAll(filepath.Dir(sock), 0o755); err != nil {
		t.Fatal(err)
	}
	ln, err := listenTestUnix(t, sock)
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
