//go:build unix && !linux

package api

func tcpPeerClosed(uintptr) (closed, known bool) { return false, false }
