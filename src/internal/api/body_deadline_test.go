package api

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A body that does not arrive in time ends the request; one that did leaves
// the handler free to wait as long as it waits (E3).
func TestABodyHasADeadlineAndOnlyTheBody(t *testing.T) {
	was := bodyWait
	bodyWait = 200 * time.Millisecond
	t.Cleanup(func() { bodyWait = was })
	// Wrapped as Routes wraps it, so the deadline reaches the connection
	// through the request log's writer.
	srv := httptest.NewServer((&Server{}).logged(bodyDeadline(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Longer than the body's deadline: a deadline left set would end
		// the request through the server's background read.
		select {
		case <-time.After(3 * bodyWait):
			io.WriteString(w, "waited "+string(body))
		case <-r.Context().Done():
			http.Error(w, "cut short", http.StatusServiceUnavailable)
		}
	}))))
	defer srv.Close()

	c, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	io.WriteString(c, "POST /x HTTP/1.1\r\nHost: h\r\nContent-Length: 10\r\n\r\n{")
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		t.Fatalf("a dribbled body was never answered: %v", err)
	}
	if resp.StatusCode != http.StatusRequestTimeout {
		t.Fatalf("a dribbled body: %d, want 408", resp.StatusCode)
	}

	resp, err = http.Post(srv.URL+"/x", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(got) != "waited {}" {
		t.Fatalf("a request that waits past the body deadline: %d %s", resp.StatusCode, got)
	}
}
