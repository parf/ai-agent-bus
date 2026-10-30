package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/tlsdir"
)

// J41: a client dialled with api.Dial over https, to a daemon that offers
// HTTP/2, polls over HTTP/1.1 and talks HTTP/2 for everything else. Replacing
// its reader then loses no message: an HTTP/2 poll's cancel is a stream reset
// the server reads late, and over it 199 of 200 runs lost a message under CPU
// load (measured), where HTTP/1.1 lost none.
func TestDialedClientReplacesAReaderOverHTTP2WithoutLoss(t *testing.T) {
	c := refusalFixture(t)
	if _, err := c.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#reader@h", Owner: "admin@h", Allow: []string{"admin@h"}}); err != nil {
		t.Fatal(err)
	}
	token := c.as("#reader@h")
	server := httptest.NewUnstartedServer(c.srv)
	server.Config.ConnContext = ConnContext
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	t.Setenv(PinEnv, tlsdir.FingerprintDER(server.Certificate().Raw))
	client, base := Dial(server.URL)
	defer client.CloseIdleConnections()
	get := func(ctx context.Context, path string) (*http.Response, []byte) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		req.Header.Set(HeaderToken, token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, nil
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return resp, raw
	}
	if resp, _ := get(context.Background(), "/identity"); resp == nil || resp.ProtoMajor != 2 {
		t.Fatalf("a call other than a poll did not negotiate HTTP/2: %+v", resp)
	}
	for run := 0; run < 100; run++ {
		old, cancel := context.WithCancel(context.Background())
		gone := make(chan struct{})
		var proto string
		go func() {
			if resp, _ := get(old, "/consume?wait=30s"); resp != nil {
				proto = resp.Proto
			}
			close(gone)
		}()
		// Bounded: a poll that is never admitted fails here, never hangs the run.
		for end := time.Now().Add(5 * time.Second); c.bus.Status().Waiting != 1; time.Sleep(time.Millisecond) {
			if time.Now().After(end) {
				t.Fatalf("run %d: the old poll was not admitted within 5s", run)
			}
		}
		cancel()
		select {
		case <-gone:
		case <-time.After(5 * time.Second):
			t.Fatalf("run %d: the cancelled poll did not return within 5s", run)
		}
		for _, body := range []string{"one", "two"} {
			if _, err := c.bus.Send(protocol.Envelope{From: "admin@h", To: "#reader@h", Body: body}); err != nil {
				t.Fatal(err)
			}
		}
		got := map[string]int{}
		for range 2 {
			resp, raw := get(context.Background(), "/consume?wait=2s")
			if resp == nil || resp.StatusCode != http.StatusOK {
				continue
			}
			if resp.ProtoMajor != 1 {
				t.Fatalf("run %d: a poll went over %s", run, resp.Proto)
			}
			var e protocol.Envelope
			json.Unmarshal(raw, &e)
			got[e.Body]++
		}
		if got["one"] != 1 || got["two"] != 1 || len(got) != 2 {
			t.Fatalf("run %d: the replacement was delivered %v (the old poll's proto %q), want each of one and two exactly once", run, got, proto)
		}
	}
}
