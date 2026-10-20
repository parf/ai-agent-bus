package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type countedBody struct {
	io.Reader
	read int
}

func (b *countedBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err
}
func (b *countedBody) Close() error { return nil }

func TestUnauthenticatedBodyIsNotBuffered(t *testing.T) {
	s, _, _ := journalFixture(t)
	body := &countedBody{Reader: strings.NewReader(strings.Repeat("x", 4<<20))}
	req := httptest.NewRequest("POST", "/send", nil)
	req.Body = body
	req.ContentLength = 4 << 20
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated POST: %d", w.Code)
	}
	if body.read > 4096 {
		t.Fatalf("unauthenticated POST consumed %d body bytes", body.read)
	}
}

func TestAuthenticatedBodyUsesTheHandlerLimit(t *testing.T) {
	s, tok, _ := journalFixture(t)
	body := &countedBody{Reader: strings.NewReader(strings.Repeat(" ", 4<<20))}
	req := httptest.NewRequest("POST", "/send", nil)
	req.Body = body
	req.ContentLength = 4 << 20
	req.Header.Set(HeaderToken, tok("alice@h"))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || body.read > 1<<20+1 || body.read < 1<<20 {
		t.Fatalf("handler limit: status %d, read %d", w.Code, body.read)
	}
}

type timedOutBody struct{}

func (timedOutBody) Read([]byte) (int, error) { return 0, os.ErrDeadlineExceeded }
func (timedOutBody) Close() error             { return nil }
func TestJSONBodyTimeoutIs408(t *testing.T) {
	s, _, _ := journalFixture(t)
	for _, read := range []func(http.ResponseWriter, *http.Request, any) bool{s.read, s.readStrict} {
		req := httptest.NewRequest("POST", "/x", nil)
		req.Body = timedOutBody{}
		w := httptest.NewRecorder()
		var body any
		if read(w, req, &body) || w.Code != http.StatusRequestTimeout {
			t.Fatalf("timeout: %d %s", w.Code, w.Body.String())
		}
	}
}
