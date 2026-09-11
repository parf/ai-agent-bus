// agent-busd: one process, two listeners, everything in memory.
// PoC scope: docs/12-stages.md#poc.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func main() {
	var (
		addr   = flag.String("addr", env("AGENT_BUS_ADDR", "127.0.0.1:7777"), "TCP listen address — loopback only in PoC")
		sock   = flag.String("socket", env("AGENT_BUS_SOCKET", api.DefaultSocket()), "unix socket path")
		tokenF = flag.String("token-file", env("AGENT_BUS_TOKEN_FILE", defaultTokenFile()), "token store; created if absent")
		owner  = flag.String("owner", env("AGENT_BUS_OWNER", defaultOwner()), "the principal this daemon belongs to")
	)
	flag.Parse()

	tokens, err := auth.Load(*tokenF, *owner)
	if err != nil {
		log.Fatalf("token: %v", err)
	}
	me, err := protocol.ParseName(*owner)
	if err != nil {
		log.Fatalf("owner: %v", err)
	}
	handler := api.New(core.New(), tokens, me.String()).Handler()

	// Plaintext bodies and a master token: loopback or an SSH tunnel, never a
	// public interface. See docs/12-stages.md#poc.
	if err := loopbackOnly(*addr); err != nil {
		log.Fatal(err)
	}
	tcp, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}
	if err := os.MkdirAll(filepath.Dir(*sock), 0o700); err != nil {
		log.Fatalf("socket dir: %v", err)
	}
	if err := clearStaleSocket(*sock); err != nil {
		log.Fatal(err)
	}
	unix, err := net.Listen("unix", *sock)
	if err != nil {
		log.Fatalf("listen %s: %v", *sock, err)
	}
	// The socket is the credential's hiding place, so the mode is not advice.
	if err := os.Chmod(*sock, 0o600); err != nil {
		log.Fatalf("chmod %s: %v", *sock, err)
	}

	srv := &http.Server{
		Handler: handler,
		// Long-poll consume holds a request open, so there is no write
		// deadline; the header and idle deadlines cost nothing.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	serve := func(l net.Listener) {
		if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve %s: %v", l.Addr(), err)
		}
	}
	go serve(tcp)
	go serve(unix)
	log.Printf("agent-busd on http://%s and %s for %s (tokens %s)", *addr, *sock, me, *tokenF)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	os.Remove(*sock)
	log.Print("stopped")
}

// clearStaleSocket removes a leftover socket, and refuses to touch anything
// else: a regular file at that path is a mistake, and a live daemon there is
// somebody else's.
func clearStaleSocket(path string) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s exists and is not a socket; refusing to remove it", path)
	}
	if c, err := net.DialTimeout("unix", path, 200*time.Millisecond); err == nil {
		c.Close()
		return fmt.Errorf("%s is already served by a running daemon", path)
	}
	return os.Remove(path)
}

func loopbackOnly(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("bad -addr %q: %w", addr, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("-addr %q is not loopback: PoC has no encryption, so it does not bind a public interface", addr)
	}
	return nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// defaultOwner is the account running the daemon, vouched for by this host —
// the `user@host` form in docs/01-identity.md#names.
func defaultOwner() string {
	who := "agent-bus"
	if u, err := user.Current(); err == nil && u.Username != "" {
		who = u.Username
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "localhost"
	}
	if i := strings.IndexByte(host, '.'); i > 0 {
		host = host[:i]
	}
	return who + "@" + host
}

func defaultTokenFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "agent-bus", "token")
}
