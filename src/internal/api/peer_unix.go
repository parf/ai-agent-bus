//go:build unix

package api

import (
	"context"
	"crypto/tls"
	"errors"
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
// between. Two questions about the socket answer at once and consume nothing:
// a peek that meets the end of stream (plain connections and unix sockets, also
// under the dual listener's replay), and on Linux the kernel's TCP state, which
// has the peer's FIN even behind queued data, as a TLS client's close_notify is.
// An HTTP/2 stream is cancelled by a frame, not a close, and is noticed only
// when the server reads it.
func peerGone(r *http.Request) bool {
	c, ok := r.Context().Value(peerKey{}).(net.Conn)
	if !ok {
		return false
	}
	for {
		if t, ok := c.(*tls.Conn); ok {
			c = t.NetConn()
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
		gone = peerClosed(n, err)
		if closed, known := tcpPeerClosed(fd); known {
			gone = gone || closed
		}
	})
	return gone
}

// peerClosed reads a peek's answer: an end of stream, or a connection that was
// reset or is no longer connected, says the caller is gone; nothing to read
// yet (EAGAIN) and every other answer say it is not.
func peerClosed(n int, err error) bool {
	switch {
	case err == nil:
		return n == 0
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.EPIPE), errors.Is(err, syscall.ENOTCONN):
		return true
	}
	return false
}
