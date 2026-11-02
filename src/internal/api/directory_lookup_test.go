package api

import (
	"fmt"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type countedEnrolDirectory struct{ calls int }

func (d *countedEnrolDirectory) Lookup(string) (ports.DirectoryEntry, error) {
	d.calls++
	return ports.DirectoryEntry{Keys: []string{"key"}}, nil
}
func TestEnrolDirectoryAllowanceReturns429WithoutFetching(t *testing.T) {
	b := core.New()
	d := &countedEnrolDirectory{}
	b.Directories(map[string]ports.Directory{"vouched": d}, nil)
	s, _ := serverFor(t, b, "owner@h")
	refused := false
	for i := 0; i < 100; i++ {
		r := httptest.NewRequest("POST", "/enrol", strings.NewReader(fmt.Sprintf(`{"name":"person%d@vouched"}`, i)))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code == 429 {
			refused = true
			if d.calls != i {
				t.Fatalf("refused lookup fetched: %d calls for %d accepted", d.calls, i)
			}
			seconds, err := strconv.Atoi(w.Header().Get("Retry-After"))
			if err != nil || seconds < 1 || seconds > 3600 {
				t.Fatalf("Retry-After=%q", w.Header().Get("Retry-After"))
			}
			if !strings.Contains(w.Body.String(), `"kind":"busy"`) {
				t.Fatalf("wrong kind: %s", w.Body.String())
			}
			break
		}
		if w.Code != 200 {
			t.Fatalf("lookup %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	if !refused {
		t.Fatal("uncached directory lookups never reached HTTP 429")
	}
}

func (d *countedEnrolDirectory) Profile(string) (ports.DirectoryProfile, error) {
	return ports.DirectoryProfile{}, nil
}
