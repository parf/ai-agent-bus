//go:build linux

package api

import (
	"net"
	"syscall"
	"testing"
	"time"
)

// The kernel's state for a TCP socket says its peer closed, with data still
// queued ahead of the close (as a TLS client's close_notify is), and says not
// while the peer is connected.
func TestTCPStateSeesThePeerCloseBehindQueuedData(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	server, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	closed := func() bool {
		raw, err := server.(syscall.Conn).SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		var got, known bool
		raw.Control(func(fd uintptr) { got, known = tcpPeerClosed(fd) })
		if !known {
			t.Fatal("the kernel would not say the socket's state")
		}
		return got
	}
	if closed() {
		t.Fatal("a connected peer reads as closed")
	}
	client.Write([]byte("queued before the close"))
	client.Close()
	for end := time.Now().Add(time.Second); !closed(); time.Sleep(time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("the peer's close was not seen behind queued data")
		}
	}
}
