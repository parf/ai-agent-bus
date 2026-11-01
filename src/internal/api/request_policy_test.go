package api

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestRequestJSONIgnoresUnknownButRefusesDuplicateKnownFields(t *testing.T) {
	type child struct {
		Name string `json:"name"`
	}
	type input struct {
		Name     string                  `json:"name"`
		Children []child                 `json:"children"`
		Payload  json.RawMessage         `json:"payload"`
		Bytes    []byte                  `json:"bytes"`
		At       time.Time               `json:"at"`
		Custom   protocol.MaintainerList `json:"custom"`
	}
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`{"name":"first","future":{"name":1,"name":2},"future":false}`, true},
		{`{"name":"first","name":"second"}`, false},
		{`{"name":"first","Name":"second"}`, false},
		{`{"name":1}`, false},
		{`{"children":[{"name":"a","future":1}]}`, true},
		{`{"children":[{"name":"a","name":"b"}]}`, false},
		{`{"payload":{"name":1,"name":2},"bytes":"YWJj","at":"2026-01-01T00:00:00Z","custom":"@ops"}`, true},
		{`{"name":"a"} {"name":"b"}`, false},
	} {
		var out input
		err := decodeRequest(strings.NewReader(tc.body), &out)
		if (err == nil) != tc.ok {
			t.Errorf("%s: %v, want accepted=%v", tc.body, err, tc.ok)
		}
		if strings.Contains(tc.body, `"payload"`) && tc.ok && string(out.Payload) != `{"name":1,"name":2}` {
			t.Fatalf("payload rewritten: %s", out.Payload)
		}
	}
}

func TestEveryRequestBodyReaderUsesTheSameFieldPolicy(t *testing.T) {
	s, _ := serverFor(t, core.New(), "owner@h")
	readers := []struct {
		name string
		read func(*httptest.ResponseRecorder, *strings.Reader, any) bool
	}{
		{"ordinary", func(w *httptest.ResponseRecorder, b *strings.Reader, v any) bool {
			return s.read(w, httptest.NewRequest("POST", "/register", b), v)
		}},
		{"narrow", func(w *httptest.ResponseRecorder, b *strings.Reader, v any) bool {
			return s.readStrict(w, httptest.NewRequest("POST", "/account", b), v)
		}},
		{"large", func(w *httptest.ResponseRecorder, b *strings.Reader, v any) bool {
			return s.readStrictUpTo(w, httptest.NewRequest("POST", "/kv/set", b), v, 4096)
		}},
	}
	for _, reader := range readers {
		t.Run(reader.name, func(t *testing.T) {
			for _, body := range []string{`{"name":"a","future":true}`, `{"name":"a","name":"b"}`, `{"name":3}`} {
				w := httptest.NewRecorder()
				var out struct{ Name string }
				ok := reader.read(w, strings.NewReader(body), &out)
				want := strings.Contains(body, "future")
				if ok != want {
					t.Fatalf("%s: accepted=%v, response=%d %s", body, ok, w.Code, w.Body.String())
				}
				if !want && (w.Code != 400 || !strings.Contains(w.Body.String(), `"kind":"malformed"`)) {
					t.Fatalf("%s", w.Body.String())
				}
			}
		})
	}
}

func TestEveryQuerySchemaIgnoresUnknownAndRefusesDuplicateKnownKeys(t *testing.T) {
	for route, fields := range queryFields {
		method, path, _ := strings.Cut(route, " ")
		for _, key := range strings.Fields(fields) {
			r := httptest.NewRequest(method, path+"?"+url.QueryEscape(key)+"=one&"+url.QueryEscape(key)+"=two", nil)
			if err := requestQuery(r); err == nil {
				t.Errorf("%s accepted duplicate %s", path, key)
			}
		}
		r := httptest.NewRequest(method, path+"?future=one&future=two", nil)
		if err := requestQuery(r); err != nil {
			t.Errorf("%s rejected unknown keys: %v", path, err)
		}
	}
}

func TestHTTPQueryPolicyAndRemovedRecipientField(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "owner@h")
	for _, tc := range []struct {
		method, path, body string
		code               int
		kind               string
	}{
		{"GET", "/identity?future=1&future=2", "", 200, ""},
		{"GET", "/ls?future=1&future=2", "", 200, ""},
		{"GET", "/ls?kind=agent&kind=queue", "", 400, "malformed"},
		{"GET", "/lookup?name=a@h&name=b@h", "", 400, "malformed"},
		{"GET", "/may-send?name=missing@h&from=forged@h", "", 404, "unknown"},
		{"GET", "/may-send?name=missing@h&name=other@h", "", 400, "malformed"},
		{"POST", "/register", `{"name":"#wire@h","kind":"agent","future":true}`, 200, ""},
		{"POST", "/register", `{"name":"#one@h","name":"#two@h","kind":"agent"}`, 400, "malformed"},
	} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.Header.Set(HeaderToken, token("owner@h"))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		var response map[string]json.RawMessage
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		var kind string
		_ = json.Unmarshal(response["kind"], &kind)
		if w.Code != tc.code || (tc.code >= 400 && kind != tc.kind) {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	// The old member cannot remove a recipient through the canonical route.
	for _, rec := range []protocol.Record{{Name: "#recipient@h", Kind: "agent", Owner: "owner@h"}, {Name: "channel@h", Kind: "pubsub", Owner: "owner@h", Subs: []string{"#recipient@h"}}} {
		if _, err := b.Register(rec); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest("POST", "/deliver-to/remove", strings.NewReader(`{"channel":"channel@h","subscriber":"#recipient@h"}`))
	r.Header.Set(HeaderToken, token("owner@h"))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	rec, _ := b.Lookup("owner@h", "channel@h")
	if w.Code == 200 || len(rec.Subs) != 1 {
		t.Fatalf("old field removed recipient: %d %s %+v", w.Code, w.Body.String(), rec.Subs)
	}
}

func TestRecordImportUsesTheSharedRequestFieldPolicy(t *testing.T) {
	s, _ := transferServer(t)
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{"data":{"format":1,"future":true},"choices":{"dry_run":true},"future":1}`, 200},
		{`{"data":{"format":1,"format":2},"choices":{"dry_run":true}}`, 400},
		{`{"data":{"format":1},"choices":{"dry_run":true,"dry_run":false}}`, 400},
	} {
		r := httptest.NewRequest("POST", "/records/import", strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		s.HandlerForOwner().ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.body, w.Code, w.Body.String())
		}
		if tc.code == 400 && !strings.Contains(w.Body.String(), `"kind":"malformed"`) {
			t.Fatal(w.Body.String())
		}
	}
}

// Pin the HTTP inventory independently of queryFields: dropping a route from
// that map must not silently remove its duplicate-key coverage.
func TestHTTPRejectsDuplicateKnownQueriesAcrossTheRouteInventory(t *testing.T) {
	s, token := serverFor(t, core.New(), "owner@h")
	for _, probe := range []string{
		"GET /secret name", "GET /config name", "GET /lookup name",
		"GET /manages name who", "GET /ls kind tag", "GET /may-send name who",
		"GET /consume topic tag inbox wait host asks original_to share",
		"POST /lock wait", "POST /try-lock wait", "GET /holders record",
		"GET /kv record name kind", "GET /kv/list record",
		"PUT /bigfile record name ttl memory", "GET /bigfile record name", "GET /bigfile/list record",
		"GET /activity name", "GET /activity/days name from to totals",
	} {
		args := strings.Fields(probe)
		for _, key := range args[2:] {
			q := url.Values{key: {"0s", "0s"}}
			if args[1] == "/consume" && key != "wait" {
				q.Set("wait", "0s")
			}
			r := httptest.NewRequest(args[0], args[1]+"?"+q.Encode(), nil)
			r.Header.Set(HeaderToken, token("owner@h"))
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			var body map[string]string
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			if w.Code != 400 || body["kind"] != "malformed" || !strings.Contains(body["error"], "duplicate query key") {
				t.Errorf("%s duplicate %s: %d %s", probe, key, w.Code, w.Body.String())
			}
		}
	}
	// Query syntax cannot override the authentication gate on private routes.
	r := httptest.NewRequest("GET", "/lookup?name=a@h&name=b@h", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("query validation bypassed authentication: %d %s", w.Code, w.Body.String())
	}
}

func TestRequestJSONPreservesTheBodyDeadlineAfterOneDocument(t *testing.T) {
	s, _ := serverFor(t, core.New(), "owner@h")
	r := httptest.NewRequest("POST", "/register", io.MultiReader(strings.NewReader(`{"name":"a"}`), timedOutBody{}))
	w := httptest.NewRecorder()
	var out struct{ Name string }
	if s.read(w, r, &out) || w.Code != 408 {
		t.Fatalf("body deadline after JSON document: %d %s", w.Code, w.Body.String())
	}
}
