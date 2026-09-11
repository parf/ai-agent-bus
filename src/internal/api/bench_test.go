package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
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
	store := memory.NewTokens(
		ports.Credential{Name: "src@h", Current: "src-tok"},
		ports.Credential{Name: "sink@h", Current: "sink-tok"},
	)
	tokens, err := auth.Load(store, "src@h")
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
