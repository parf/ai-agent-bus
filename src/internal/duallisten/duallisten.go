// Package duallisten serves TLS and plain HTTP on one TCP port. Each accepted
// connection is sniffed: a first byte of 0x16, a TLS handshake record, makes
// it a TLS connection, and anything else is handed on as it came, with the
// peeked byte replayed. See docs/11-processes.md#the-tcp-listener.
//
// It bounds only the first byte. A client that starts a TLS handshake and then
// stalls is bounded by the server that reads the connection: net/http applies
// its ReadHeaderTimeout (or ReadTimeout) to the handshake, so a server behind
// this listener must set one.
package duallisten

import (
	"bufio"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"
)

// SniffTimeout is how long a new connection may take to send its first byte.
// A client that says nothing is closed; it never holds up Accept, because
// every connection is sniffed on its own goroutine.
var SniffTimeout = 5 * time.Second

// peekedConn replays the bytes the sniff buffered.
type peekedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *peekedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

type listener struct {
	net.Listener
	cfg   *tls.Config
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

// New wraps inner so that it accepts both TLS, answered with cfg, and plain
// connections.
func New(inner net.Listener, cfg *tls.Config) net.Listener {
	l := &listener{Listener: inner, cfg: cfg, conns: make(chan net.Conn), done: make(chan struct{})}
	go l.acceptLoop()
	return l
}

func (l *listener) acceptLoop() {
	var wait time.Duration
	for {
		c, err := l.Listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			l.shut()
			return
		}
		// Anything else — out of file descriptors above all — passes, as it
		// does in net/http's own loop: giving up would kill the port exactly
		// when the host is busiest.
		if err != nil {
			wait = min(max(2*wait, 5*time.Millisecond), time.Second)
			time.Sleep(wait)
			continue
		}
		wait = 0
		go l.sniff(c)
	}
}

func (l *listener) sniff(c net.Conn) {
	c.SetReadDeadline(time.Now().Add(SniffTimeout))
	br := bufio.NewReader(c)
	b, err := br.Peek(1)
	c.SetReadDeadline(time.Time{})
	if err != nil {
		c.Close()
		return
	}
	var out net.Conn = &peekedConn{Conn: c, r: br}
	if b[0] == 0x16 {
		out = tls.Server(out, l.cfg)
	}
	// The listener may close while this one was sniffing: its connection is
	// then nobody's, and a send on a closed channel would panic.
	select {
	case l.conns <- out:
	case <-l.done:
		c.Close()
	}
}

func (l *listener) shut() { l.once.Do(func() { close(l.done) }) }

func (l *listener) Accept() (net.Conn, error) {
	// What is already sniffed is handed out before the listener says closed.
	select {
	case c := <-l.conns:
		return c, nil
	default:
	}
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *listener) Close() error {
	l.shut()
	return l.Listener.Close()
}
