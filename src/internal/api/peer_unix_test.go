//go:build unix

package api

import (
	"syscall"
	"testing"
)

// A peek says the caller is gone for an end of stream, a reset and a socket no
// longer connected; only nothing-yet and data say it is there.
func TestPeekAnswerReadsAsTheCallerBeingGoneOrNot(t *testing.T) {
	for _, tc := range []struct {
		n    int
		err  error
		gone bool
	}{
		{0, nil, true},
		{1, nil, false},
		{-1, syscall.EAGAIN, false},
		{-1, syscall.ECONNRESET, true},
		{-1, syscall.EPIPE, true},
		{-1, syscall.ENOTCONN, true},
		{-1, syscall.EINTR, false},
	} {
		if got := peerClosed(tc.n, tc.err); got != tc.gone {
			t.Errorf("n=%d err=%v: gone %v, want %v", tc.n, tc.err, got, tc.gone)
		}
	}
}
