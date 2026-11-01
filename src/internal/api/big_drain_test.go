package api

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
)

// counting is a request body that says how much of it was read; it ends after
// total bytes however many were declared.
type counting struct {
	read, total int64
}

func (c *counting) Read(p []byte) (int, error) {
	if c.read >= c.total {
		return 0, io.EOF
	}
	n := int64(len(p))
	if c.total-c.read < n {
		n = c.total - c.read
	}
	c.read += n
	return int(n), nil
}

func putBody(s *Server, token, path string, declared int64, body *counting) *httptest.ResponseRecorder {
	req := httptest.NewRequest("PUT", path, body)
	req.ContentLength = declared
	req.Header.Set(HeaderToken, token)
	req.RemoteAddr = "@"
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

// An early refusal reads what the client declared, and no more, so a client
// still writing sees the answer; a refusal that needs no drain reads nothing.
func TestBigPutDrainsAnEarlyRefusalByItsDeclaredLengthAndNoMore(t *testing.T) {
	s, tok := bigServer(t)
	const declared = 3 << 20
	// A refusal for authority arrives before the body is read: it is drained,
	// up to the declared length even if the client sends more.
	body := &counting{total: 4 * declared}
	w := putBody(s, tok("bob@h"), "/big?record=%23svc@h&name=k", declared, body)
	if w.Code != http.StatusForbidden || w.Header().Get("Connection") != "close" {
		t.Fatalf("an admitted non-Maintainer's put = %d %v, want 403 and a close", w.Code, w.Header())
	}
	if body.read != declared {
		t.Fatalf("the drain read %d bytes, want exactly the declared %d", body.read, declared)
	}
	// A bad parameter is early as well.
	body = &counting{total: 2 * declared}
	if w = putBody(s, tok("alice@h"), "/big?record=%23svc@h&name=k&ttl=0", declared, body); w.Code != http.StatusBadRequest || body.read != declared {
		t.Fatalf("a bad ttl = %d after reading %d, want 400 after %d", w.Code, body.read, declared)
	}
	// No declared length: the drain is bounded by the cap.
	body = &counting{total: 2 * core.BigMaxValue}
	if w = putBody(s, tok("bob@h"), "/big?record=%23svc@h&name=k", -1, body); w.Code != http.StatusForbidden || body.read != core.BigMaxValue {
		t.Fatalf("an unknown length = %d after reading %d, want 403 after %d", w.Code, body.read, int64(core.BigMaxValue))
	}
	// A declared length over the cap is a 413 at once, nothing read.
	body = &counting{total: 2 * core.BigMaxValue}
	if w = putBody(s, tok("alice@h"), "/big?record=%23svc@h&name=k", core.BigMaxValue+1, body); w.Code != http.StatusRequestEntityTooLarge || body.read != 0 {
		t.Fatalf("an over-cap length = %d after reading %d, want 413 after 0", w.Code, body.read)
	}
	// A caller who was never admitted is refused before the handler: nothing read.
	body = &counting{total: declared}
	if w = putBody(s, "not-a-token", "/big?record=%23svc@h&name=k", declared, body); w.Code != http.StatusUnauthorized || body.read != 0 {
		t.Fatalf("an unauthenticated put = %d after reading %d, want 401 after 0", w.Code, body.read)
	}
	// An accepted put reads its body as ever.
	body = &counting{total: 1000}
	if w = putBody(s, tok("alice@h"), "/big?record=%23svc@h&name=ok", 1000, body); w.Code != http.StatusOK || body.read != 1000 {
		t.Fatalf("an accepted put = %d after reading %d", w.Code, body.read)
	}
}

// Over a real connection, a client still writing 3 MiB when the refusal is
// decided receives its status and cause instead of a reset. The client is a
// bare socket: it writes until the server stops it, then reads the answer.
// This is the behaviour check, not the proof of the drain: Go's own server
// already holds a closing connection open for 500 ms on loopback, so this
// passes without the drain too; the drain's bound is proved by the counting
// test above.
func TestBigPutRefusalReachesAClientStillWriting(t *testing.T) {
	s, tok := bigServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	const size = 3 << 20
	for _, c := range []struct {
		who  string
		path string
		code int
		want string
	}{
		{"bob@h", "/big?record=%23svc@h&name=k", http.StatusForbidden, "someone else"},
		{"alice@h", "/big?record=%23nobody@h&name=k", http.StatusNotFound, "no such name"},
		{"alice@h", "/big?record=%23svc@h&name=k&ttl=0", http.StatusBadRequest, "ttl"},
	} {
		conn, err := net.Dial("tcp", srv.Listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		conn.SetDeadline(time.Now().Add(15 * time.Second))
		fmt.Fprintf(conn, "PUT %s HTTP/1.1\r\nHost: x\r\n%s: %s\r\nContent-Length: %d\r\nContent-Type: application/octet-stream\r\n\r\n", c.path, HeaderToken, tok(c.who), size)
		chunk := make([]byte, 32<<10)
		for sent := 0; sent < size; sent += len(chunk) {
			if _, err := conn.Write(chunk); err != nil {
				break // the server has stopped listening
			}
			time.Sleep(time.Millisecond)
		}
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			t.Errorf("%s: the client saw %v instead of the refusal", c.path, err)
			conn.Close()
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != c.code || !strings.Contains(strings.ToLower(string(raw)), c.want) {
			t.Errorf("%s = %d %s, want %d naming %q", c.path, resp.StatusCode, raw, c.code, c.want)
		}
		conn.Close()
	}
}
