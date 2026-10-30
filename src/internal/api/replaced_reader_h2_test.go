package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/duallisten"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// J41: the replaced-reader delivery count over HTTP/2, where a cancelled poll
// is a stream reset rather than a close. A measurement, not a check: it counts
// the runs that lost or doubled a message and logs the rate, and runs only with
// J41_MEASURE=1 (J41_RUNS sets the repeats, default 500).
func TestMeasureReplacingAReaderOverHTTP2(t *testing.T) {
	if os.Getenv("J41_MEASURE") != "1" {
		t.Skip("J41 measurement: set J41_MEASURE=1")
	}
	runs := 500
	if v := os.Getenv("J41_RUNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			runs = n
		}
	}
	for _, mode := range []string{"tls-h1", "tls-h2", "dual-tls-h2"} {
		t.Run(mode, func(t *testing.T) { measure(t, mode, runs) })
	}
}

func measure(t *testing.T, mode string, runs int) {
	c := refusalFixture(t)
	if _, err := c.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#reader@h", Owner: "admin@h", Allow: []string{"admin@h"}}); err != nil {
		t.Fatal(err)
	}
	token := c.as("#reader@h")
	server := httptest.NewUnstartedServer(c.srv)
	server.Config.ConnContext = ConnContext
	h2 := mode != "tls-h1"
	if mode == "dual-tls-h2" {
		seed := httptest.NewUnstartedServer(nil)
		seed.StartTLS()
		cfg := &tls.Config{Certificates: seed.TLS.Certificates, NextProtos: []string{"h2", "http/1.1"}}
		seed.Close()
		inner, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		server.Listener = duallisten.New(inner, cfg)
		server.Start()
		server.URL = "https" + server.URL[len("http"):]
	} else {
		server.EnableHTTP2 = h2
		server.StartTLS()
	}
	defer server.Close()
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, ForceAttemptHTTP2: h2}}
	defer client.CloseIdleConnections()
	var proto string
	poll := func(ctx context.Context, wait string) (string, int) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/consume?wait="+wait, nil)
		req.Header.Set(HeaderToken, token)
		resp, err := client.Do(req)
		if err != nil {
			return "", 0
		}
		defer resp.Body.Close()
		proto = resp.Proto
		raw, _ := io.ReadAll(resp.Body)
		var e protocol.Envelope
		json.Unmarshal(raw, &e)
		return e.Body, resp.StatusCode
	}
	lost, doubled := 0, 0
	for run := 0; run < runs; run++ {
		old, cancel := context.WithCancel(context.Background())
		gone := make(chan struct{})
		go func() { poll(old, "30s"); close(gone) }()
		for c.bus.Status().Waiting != 1 {
			time.Sleep(time.Millisecond)
		}
		cancel()
		<-gone
		for _, body := range []string{"one", "two"} {
			if _, err := c.bus.Send(protocol.Envelope{From: "admin@h", To: "#reader@h", Body: body}); err != nil {
				t.Fatal(err)
			}
		}
		got := map[string]int{}
		for range 2 {
			if body, code := poll(context.Background(), "300ms"); code == http.StatusOK {
				got[body]++
			}
		}
		if body, code := poll(context.Background(), "0s"); code == http.StatusOK {
			got[body+"!"]++
		}
		switch {
		case got["one"] == 1 && got["two"] == 1 && len(got) == 2:
		case len(got) < 2:
			lost++
		default:
			doubled++
		}
	}
	t.Logf("%s (%s): %d runs, %d lost a message, %d delivered wrongly", mode, proto, runs, lost, doubled)
}
