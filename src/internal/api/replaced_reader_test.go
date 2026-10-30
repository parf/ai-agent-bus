package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/duallisten"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A reader replaced while messages arrive loses none of them: the poll it
// cancelled has no reader, so nothing is handed to it, and what it was
// handed goes back. Each of the messages sent right after the cancel is
// delivered to the replacement once, never twice and never not at all.
func TestReplacingAReaderLosesNoMessage(t *testing.T) {
	for _, transport := range []string{"plain", "tls", "dual-plain", "dual-tls"} {
		t.Run(transport, func(t *testing.T) { replacingAReader(t, transport) })
	}
}

// pollServer serves c over the transport: a plain listener, a TLS one, or the
// daemon's dual listener answering either, which wraps each connection.
func pollServer(t *testing.T, handler http.Handler, transport string) (*httptest.Server, *http.Client) {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	server.Config.ConnContext = ConnContext
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	switch transport {
	case "plain":
		server.Start()
	case "tls":
		server.StartTLS()
	default:
		seed := httptest.NewUnstartedServer(nil)
		seed.StartTLS()
		cfg := &tls.Config{Certificates: seed.TLS.Certificates}
		seed.Close()
		inner, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		server.Listener = duallisten.New(inner, cfg)
		server.Start()
		if transport == "dual-tls" {
			server.URL = "https" + server.URL[len("http"):]
		}
	}
	t.Cleanup(func() { client.CloseIdleConnections(); server.Close() })
	return server, client
}

func replacingAReader(t *testing.T, transport string) {
	c := refusalFixture(t)
	if _, err := c.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#reader@h", Owner: "admin@h", Allow: []string{"admin@h"}}); err != nil {
		t.Fatal(err)
	}
	token := c.as("#reader@h")
	server, client := pollServer(t, c.srv, transport)
	poll := func(ctx context.Context, wait string) (string, int) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/consume?wait="+wait, nil)
		req.Header.Set(HeaderToken, token)
		resp, err := client.Do(req)
		if err != nil {
			return "", 0
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var e protocol.Envelope
		json.Unmarshal(raw, &e)
		return e.Body, resp.StatusCode
	}
	for run := 0; run < 150; run++ {
		old, cancel := context.WithCancel(context.Background())
		gone := make(chan struct{})
		go func() { poll(old, "30s"); close(gone) }()
		for c.bus.Status().Waiting != 1 {
			time.Sleep(time.Millisecond)
		}
		// The cancel returns once the client has closed its end, as a runner's
		// stop does before it serves again; messages then arrive at once.
		cancel()
		<-gone
		for _, body := range []string{"one", "two"} {
			if _, err := c.bus.Send(protocol.Envelope{From: "admin@h", To: "#reader@h", Body: body}); err != nil {
				t.Fatal(err)
			}
		}
		got := map[string]int{}
		for range 2 {
			if body, code := poll(context.Background(), "2s"); code == http.StatusOK {
				got[body]++
			}
		}
		if got["one"] != 1 || got["two"] != 1 || len(got) != 2 {
			t.Fatalf("run %d: the replacement was delivered %v, want each of one and two exactly once", run, got)
		}
		if body, code := poll(context.Background(), "0s"); code == http.StatusOK {
			t.Fatalf("run %d: %q was delivered a second time", run, body)
		}
	}
}
