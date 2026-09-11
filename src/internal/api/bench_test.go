package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The whole handler path, which is where the daemon's time actually goes:
// auth parses a name, the handler decodes, core parses both ends.
func BenchmarkSendAndConsume(b *testing.B) {
	bus := core.New()
	if _, err := bus.Register(protocol.Record{Name: "sink@h", Kind: "generic", Owner: "sink@h"}); err != nil {
		b.Fatal(err)
	}
	// Two principals, because a token now backs one name: the benchmark
	// pays the lookup the real path pays.
	path := filepath.Join(b.TempDir(), "tokens")
	if err := os.WriteFile(path, []byte("src@h src-tok\nsink@h sink-tok\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	tokens, err := auth.Load(path, "src@h")
	if err != nil {
		b.Fatal(err)
	}
	h := New(bus, tokens, "src@h").Handler()
	body := []byte(`{"to":"sink@h","body":"x"}`)

	do := func(who, token, method, target string, payload []byte) {
		r := httptest.NewRequest(method, target, bytes.NewReader(payload))
		r.Header.Set(HeaderUser, who)
		r.Header.Set(HeaderToken, token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			b.Fatalf("%s %s: %d %s", method, target, w.Code, w.Body)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		do("src@h", "src-tok", "POST", "/send", body)
		do("sink@h", "sink-tok", "GET", "/consume?wait=0s", nil)
	}
}
