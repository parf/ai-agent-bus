package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// `host` on consume states where the reader is: a bad one is a 400 that names
// it, a good one shows in the record's lookup as a host with its last read, and
// a read without one adds none.
func TestConsumeHostIsCheckedAndListed(t *testing.T) {
	c := refusalFixture(t)
	if _, err := c.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#pool@h", Owner: "admin@h", Allow: []string{"admin@h"}}); err != nil {
		t.Fatal(err)
	}
	token := c.as("#pool@h")
	get := func(path string) (int, string) {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set(HeaderToken, token)
		w := httptest.NewRecorder()
		c.srv.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	for _, bad := range []string{"has%20space", "under_score", "", strings.Repeat("a", 65)} {
		code, body := get("/consume?wait=0s&host=" + bad)
		if code != http.StatusBadRequest || !strings.Contains(body, "host") {
			t.Fatalf("host=%q: %d %s, want a 400 naming the host", bad, code, body)
		}
	}
	lookup := func() protocol.Record {
		code, body := get("/lookup?name=%23pool%40h")
		var r protocol.Record
		if code != http.StatusOK || json.Unmarshal([]byte(body), &r) != nil {
			t.Fatalf("lookup: %d %s", code, body)
		}
		return r
	}
	if code, _ := get("/consume?wait=0s"); code != http.StatusNoContent {
		t.Fatalf("a read with no host: %d", code)
	}
	if got := lookup().Hosts; len(got) != 0 {
		t.Fatalf("a read with no host listed %+v", got)
	}
	if code, _ := get("/consume?wait=0s&host=srv1"); code != http.StatusNoContent {
		t.Fatalf("a read from srv1: %d", code)
	}
	r := lookup()
	if len(r.Hosts) != 1 || r.Hosts[0].Name != "srv1" || r.Hosts[0].LastRead.IsZero() {
		t.Fatalf("hosts %+v, want srv1 with its last read", r.Hosts)
	}
}
