//go:build unix

package api

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"syscall"
)

type peerKey struct{}

// ConnContext keeps a request's connection where the handler can ask whether
// its peer is still there. A server installs it as http.Server.ConnContext.
func ConnContext(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, peerKey{}, c)
}

// peerGone is whether the caller has closed its end of the connection, which
// a long poll's handler learns from the request context only once the server
// has read the close: a moment too late for a message that was handed to it in
// between. A peek at the socket sees the close at once and consumes nothing.
// Under TLS the close is the alert the client sends before it: a record of
// type 21. An HTTP/2 stream is cancelled by a frame, not a close, and is
// noticed only when the server reads it.
func peerGone(r *http.Request) bool {
	c, ok := r.Context().Value(peerKey{}).(net.Conn)
	if !ok {
		return false
	}
	secure := false
	for {
		if t, ok := c.(*tls.Conn); ok {
			c, secure = t.NetConn(), true
			continue
		}
		if w, ok := c.(interface{ NetConn() net.Conn }); ok {
			c = w.NetConn()
			continue
		}
		break
	}
	sc, ok := c.(syscall.Conn)
	if !ok {
		return false
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return false
	}
	gone := false
	// Control, not Read: the server's own background read holds the read side.
	raw.Control(func(fd uintptr) {
		var one [1]byte
		n, _, err := syscall.Recvfrom(int(fd), one[:], syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		if err != nil {
			return // nothing to read yet, or an error that says nothing of the peer
		}
		gone = n == 0 || (secure && one[0] == 21) // end of stream, or a TLS alert
	})
	return gone
}
