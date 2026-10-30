package api

import (
	"net/http"
	"testing"
)

type closeCounter struct{ closed int }

func (c *closeCounter) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }
func (c *closeCounter) CloseIdleConnections()                           { c.closed++ }

// The client's CloseIdleConnections reaches the poll transport and the other
// one, whichever a call used.
func TestSplitTransportClosesIdleConnectionsOfBothPools(t *testing.T) {
	poll, rest := &closeCounter{}, &closeCounter{}
	(&http.Client{Transport: splitTransport{poll: poll, rest: rest}}).CloseIdleConnections()
	if poll.closed != 1 || rest.closed != 1 {
		t.Fatalf("closed poll %d, rest %d; want 1 and 1", poll.closed, rest.closed)
	}
}
