package api

import (
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRetiredRecordMembersRefused(t *testing.T) {
	for _, raw := range []string{`{"personal":false}`, `{"personal":true}`, `{"reading":false}`, `{"reading":true}`} {
		t.Run(raw, func(t *testing.T) {
			if err := decodeRequest(strings.NewReader(raw), new(protocol.Record)); err == nil {
				t.Fatalf("retired record member accepted: %s", raw)
			}
			if err := decodeRequest(strings.NewReader(raw), new(core.Management)); err == nil {
				t.Fatalf("retired management member accepted: %s", raw)
			}
		})
	}
}

func TestUnknownQueryKeysRefused(t *testing.T) {
	for _, path := range []string{"/identity", "/status", "/ls", "/consume", "/register", "/records/import"} {
		for _, method := range []string{"GET", "POST"} {
			if err := requestQuery(httptest.NewRequest(method, path+"?retired=1", nil)); err == nil {
				t.Errorf("%s %s accepted unknown query key", method, path)
			}
		}
	}
}
