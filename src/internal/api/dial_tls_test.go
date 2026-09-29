package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/tlsdir"
)

// A pinned client reaches exactly the certificate it names, a self-signed one
// included; any other is refused, and so is a self-signed one with no pin.
// Nothing falls back to plain HTTP.
func TestAnHTTPSDaemonIsReachedByItsPinAndNothingElse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "pinned") }))
	defer srv.Close()
	addr := srv.URL
	right := tlsdir.FingerprintDER(srv.Certificate().Raw)

	call := func(pin string) (string, error) {
		t.Setenv(PinEnv, pin)
		client, base := Dial(addr)
		r, err := client.Get(base + "/")
		if err != nil {
			return "", err
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return string(b), nil
	}
	if got, err := call(right); err != nil || got != "pinned" {
		t.Fatalf("the right pin: %q, %v", got, err)
	}
	if got, err := call(strings.ToUpper(strings.TrimPrefix(right, "sha256:"))); err != nil || got != "pinned" {
		t.Fatalf("the right pin, written otherwise: %q, %v", got, err)
	}
	if _, err := call("sha256:" + strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "not the pinned") {
		t.Fatalf("a wrong pin was not refused for the pin: %v", err)
	}
	if _, err := call(""); err == nil {
		t.Fatal("a self-signed daemon was trusted with no pin")
	}
	if !IsTCP("https://h:1") || !IsTCP("http://h:1") || IsTCP("/run/agent-bus/bus.sock") {
		t.Fatal("IsTCP does not tell a port from a socket")
	}
}
