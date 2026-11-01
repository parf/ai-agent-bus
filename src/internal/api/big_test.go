package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/bigfile"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func bigReq(s *Server, token, method, path string, body []byte, sum string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set(HeaderToken, token)
	req.RemoteAddr = "@"
	if sum != "" {
		req.Header.Set("X-Big-Sha256", sum)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func bigServer(t *testing.T) (*Server, func(string) string) {
	t.Helper()
	b := core.New()
	b.Persistence(memory.NewState())
	store, err := bigfile.Open(filepath.Join(t.TempDir(), "big"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	b.SetBigStore(store)
	s, tok := serverFor(t, b, "admin@h")
	for _, u := range []string{"alice@h", "bob@h", "eve@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: u}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h", Allow: []string{"bob@h"}}); err != nil {
		t.Fatal(err)
	}
	return s, tok
}

func TestBigKeyRoundTripAnswersAndRefusals(t *testing.T) {
	s, tok := bigServer(t)
	data := []byte(strings.Repeat("payload\x00", 1000))
	sum := sha256.Sum256(data)
	hexsum := hex.EncodeToString(sum[:])
	put := "/big?record=%23svc@h&name=file&ttl=3600"

	w := bigReq(s, tok("alice@h"), "PUT", put, data, hexsum)
	var up struct {
		Record, Name, SHA256, Held, Expires string
		Size                                int64
		Replaced                            bool
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &up) != nil || up.Size != int64(len(data)) || up.SHA256 != hexsum || up.Held != "file" || up.Replaced || up.Expires == "" || up.Name != "file" {
		t.Fatalf("put = %d %s", w.Code, w.Body)
	}
	if w = bigReq(s, tok("alice@h"), "PUT", put, data, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"replaced":true`) {
		t.Fatalf("a replace = %d %s", w.Code, w.Body)
	}
	// An admitted caller downloads the bytes and sees the hash in headers.
	w = bigReq(s, tok("bob@h"), "GET", "/big?record=%23svc@h&name=file", nil, "")
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("X-Big-Sha256") != hexsum ||
		w.Header().Get("X-Big-Held") != "file" || w.Header().Get("X-Big-Expires") != up.Expires || w.Header().Get("Content-Length") != "8000" {
		t.Fatalf("get = %d, %d bytes, headers %v", w.Code, w.Body.Len(), w.Header())
	}
	if w = bigReq(s, tok("bob@h"), "GET", "/big/list?record=%23svc@h", nil, ""); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"keys":[{"name":"file","size":8000,"sha256":"`+hexsum+`","held":"file","expires":"`+up.Expires+`"}]`) {
		t.Fatalf("list = %d %s", w.Code, w.Body)
	}
	// memory=true holds the key in memory and says so.
	if w = bigReq(s, tok("alice@h"), "PUT", "/big?record=%23svc@h&name=mem&memory=true", []byte("ephemeral"), ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"held":"memory"`) {
		t.Fatalf("a memory put = %d %s", w.Code, w.Body)
	}
	if w = bigReq(s, tok("bob@h"), "GET", "/big?record=%23svc@h&name=mem", nil, ""); w.Header().Get("X-Big-Held") != "memory" || w.Body.String() != "ephemeral" {
		t.Fatalf("a memory get = %d %q %v", w.Code, w.Body, w.Header())
	}
	// Refusals: kind and status are what clients rely on.
	cases := []struct {
		name, who, method, path string
		body                    []byte
		sum                     string
		code                    int
	}{
		{"an admitted caller's put", "bob@h", "PUT", put, data, "", http.StatusForbidden},
		{"a stranger's get", "eve@h", "GET", "/big?record=%23svc@h&name=file", nil, "", http.StatusForbidden},
		{"a stranger's list", "eve@h", "GET", "/big/list?record=%23svc@h", nil, "", http.StatusForbidden},
		{"an unknown record", "alice@h", "GET", "/big?record=%23nobody@h&name=file", nil, "", http.StatusNotFound},
		{"an absent key", "alice@h", "GET", "/big?record=%23svc@h&name=none", nil, "", http.StatusNotFound},
		{"an empty body", "alice@h", "PUT", "/big?record=%23svc@h&name=e", nil, "", http.StatusBadRequest},
		{"a bad ttl", "alice@h", "PUT", "/big?record=%23svc@h&name=e&ttl=0", data, "", http.StatusBadRequest},
		{"a ttl over 30 days", "alice@h", "PUT", "/big?record=%23svc@h&name=e&ttl=2592001", data, "", http.StatusBadRequest},
		{"a bad memory flag", "alice@h", "PUT", "/big?record=%23svc@h&name=e&memory=yes", data, "", http.StatusBadRequest},
		{"a malformed hash", "alice@h", "PUT", "/big?record=%23svc@h&name=e", data, "XYZ", http.StatusBadRequest},
		{"a wrong hash", "alice@h", "PUT", "/big?record=%23svc@h&name=e", data, strings.Repeat("0", 64), http.StatusBadRequest},
		{"a value over the cap", "alice@h", "PUT", "/big?record=%23svc@h&name=over", make([]byte, core.BigMaxValue+1), "", http.StatusRequestEntityTooLarge},
	}
	for _, c := range cases {
		w := bigReq(s, tok(c.who), c.method, c.path, c.body, c.sum)
		if w.Code != c.code {
			t.Errorf("%s = %d %.120s, want %d", c.name, w.Code, w.Body, c.code)
		}
	}
	if w = bigReq(s, tok("bob@h"), "POST", "/big/delete", []byte(`{"record":"#svc@h","name":"file"}`), ""); w.Code != http.StatusForbidden {
		t.Fatalf("an admitted caller's delete = %d", w.Code)
	}
	if w = bigReq(s, tok("alice@h"), "POST", "/big/delete", []byte(`{"record":"#svc@h","name":"file"}`), ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"deleted":true`) {
		t.Fatalf("delete = %d %s", w.Code, w.Body)
	}
	if w = bigReq(s, tok("alice@h"), "POST", "/big/delete", []byte(`{"record":"#svc@h","name":"file"}`), ""); !strings.Contains(w.Body.String(), `"deleted":false`) {
		t.Fatalf("a second delete = %s", w.Body)
	}
}
