package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

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
	h := New(bus, "tok").Handler()
	body := []byte(`{"to":"sink@h","body":"x"}`)

	do := func(who, method, target string, payload []byte) {
		r := httptest.NewRequest(method, target, bytes.NewReader(payload))
		r.Header.Set(HeaderUser, who)
		r.Header.Set(HeaderToken, "tok")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			b.Fatalf("%s %s: %d %s", method, target, w.Code, w.Body)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		do("src@h", "POST", "/send", body)
		do("sink@h", "GET", "/consume?wait=0s", nil)
	}
}
