package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// ServeMux answers HEAD for every GET pattern, but a consume takes a message
// the caller can never read from a HEAD answer: a probe that sends HEAD with a
// valid token must be refused and take nothing.
func TestAHeadRequestNeverConsumesAMessage(t *testing.T) {
	c := refusalFixture(t)
	if _, err := c.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#reader@h", Owner: "admin@h", Allow: []string{"admin@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.bus.Send(protocol.Envelope{From: "admin@h", To: "#reader@h", Body: "still here"}); err != nil {
		t.Fatal(err)
	}
	reader := c.as("#reader@h")
	call := func(method string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/consume?wait=0s", nil)
		r.Header.Set(HeaderToken, reader)
		w := httptest.NewRecorder()
		c.srv.ServeHTTP(w, r)
		return w
	}
	head := call(http.MethodHead)
	if head.Code != http.StatusMethodNotAllowed || head.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("HEAD /consume = %d Allow %q, want 405 Allow GET", head.Code, head.Header().Get("Allow"))
	}
	if !strings.Contains(head.Body.String(), `"no_route"`) {
		t.Fatalf("HEAD /consume body %q lacks the no_route kind", head.Body.String())
	}
	got := call(http.MethodGet)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "still here") {
		t.Fatalf("after a HEAD the message is %d %s, want it queued", got.Code, got.Body.String())
	}
}
