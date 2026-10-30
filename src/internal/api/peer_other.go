//go:build !unix

package api

import (
	"context"
	"net"
	"net/http"
)

// ConnContext is a no-op where a connection cannot be peeked.
func ConnContext(ctx context.Context, _ net.Conn) context.Context { return ctx }

func peerGone(*http.Request) bool { return false }
