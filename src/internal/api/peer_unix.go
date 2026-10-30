//go:build unix

package api

import (
	"context"
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
func peerGone(r *http.Request) bool {
	c, ok := r.Context().Value(peerKey{}).(net.Conn)
	if !ok {
		return false
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
		gone = n == 0 && err == nil // an end of stream, not "nothing to read yet"
	})
	return gone
}
