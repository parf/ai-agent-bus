package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Query schemas follow JSON schemas: ignore unknown keys, reject repeated
// known keys. Value/type and permission checks remain in the owning handler.
var queryFields = map[string]string{
	"GET /secret": "name", "GET /config": "name", "GET /lookup": "name",
	"GET /manages": "name who", "GET /ls": "kind tag", "GET /may-send": "name who",
	"GET /consume": "topic tag inbox wait host asks original_to share",
	"POST /lock":   "wait", "POST /try-lock": "wait", "GET /holders": "record",
	"GET /kv": "record name kind", "GET /kv/list": "record",
	"PUT /bigfile": "record name ttl memory", "GET /bigfile": "record name", "GET /bigfile/list": "record",
	"GET /activity": "name", "GET /activity/days": "name from to totals",
}

func requestQuery(r *http.Request) error {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return fmt.Errorf("bad query: %w", err)
	}
	method := r.Method
	if method == http.MethodHead {
		method = http.MethodGet
	}
	for _, key := range strings.Fields(queryFields[method+" "+r.URL.Path]) {
		if len(q[key]) > 1 {
			return fmt.Errorf("duplicate query key %q", key)
		}
	}
	return nil
}

func (s *Server) query(w http.ResponseWriter, r *http.Request) bool {
	if err := requestQuery(r); err != nil {
		if r.URL.Path == "/may-send" {
			answer(w, 400, "malformed", err.Error())
		} else {
			s.refuse(w, 400, "malformed", err.Error())
		}
		return false
	}
	return true
}
