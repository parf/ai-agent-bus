//go:build linux

package api

import (
	"syscall"
	"unsafe"
)

const (
	solTCP  = 6  // SOL_TCP
	tcpInfo = 11 // TCP_INFO
)

// tcpPeerClosed says whether the kernel has the peer's FIN on this TCP socket:
// its state is CLOSE_WAIT, LAST_ACK, CLOSING or CLOSE. Unlike a peek it does
// not depend on what is queued ahead of the FIN, so it holds under TLS, where
// the client's close_notify is data in front of it. known is false when the
// socket is not TCP or the kernel will not say.
func tcpPeerClosed(fd uintptr) (closed, known bool) {
	var info [192]byte // struct tcp_info; its first byte is tcpi_state
	size := uint32(len(info))
	_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, solTCP, tcpInfo, uintptr(unsafe.Pointer(&info[0])), uintptr(unsafe.Pointer(&size)), 0)
	if errno != 0 || size == 0 {
		return false, false
	}
	switch info[0] {
	case 7, 8, 9, 11: // CLOSE, CLOSE_WAIT, LAST_ACK, CLOSING
		return true, true
	}
	return false, true
}
