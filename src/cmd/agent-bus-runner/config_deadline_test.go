package main

import (
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type deadlineConfigTransport struct {
	t    *testing.T
	want time.Time
	seen bool
}

func (f *deadlineConfigTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/config" {
		f.seen = true
		got, ok := req.Context().Deadline()
		if !ok || !got.Equal(f.want) {
			f.t.Errorf("config read deadline %v, want message deadline %v", got, f.want)
			return nil, io.ErrClosedPipe
		}
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
}
func TestConfigurationReadReleasesExpiredMessage(t *testing.T) {
	tr := &deadlineConfigTransport{t: t}
	r := &runner{host: t.TempDir(), say: io.Discard}
	s := &served{name: "#fixture@h", work: t.TempDir(), dir: t.TempDir(), cmd: "false", box: sandbox.Off{}, jail: &jail{}, bus: &client{http: &http.Client{Transport: tr}, base: "http://fixture"}, in: installed{Agent: agentFile{Deadline: "1s"}}}
	deadline := time.Now().Add(100 * time.Millisecond)
	tr.want = deadline
	r.runOne(s, protocol.Envelope{ID: "fixture", From: "caller@h", Deadline: deadline})
	if !tr.seen {
		t.Fatal("configuration was not fetched")
	}
	if s.busy.Load() != 0 {
		t.Fatal("expired configuration read retained busy slot")
	}
}
