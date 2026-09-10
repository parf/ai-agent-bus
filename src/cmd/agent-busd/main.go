// agent-busd: one process, two listeners, everything in memory.
// PoC scope: docs/12-stages.md#poc.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/core"
)

func main() {
	var (
		addr   = flag.String("addr", env("AGENT_BUS_ADDR", "127.0.0.1:7777"), "TCP listen address — loopback only in PoC")
		sock   = flag.String("socket", env("AGENT_BUS_SOCKET", defaultSocket()), "unix socket path")
		tokenF = flag.String("token-file", env("AGENT_BUS_TOKEN_FILE", defaultTokenFile()), "master token file; created if absent")
	)
	flag.Parse()

	token, err := loadToken(*tokenF)
	if err != nil {
		log.Fatalf("token: %v", err)
	}
	handler := api.New(core.New(), token).Handler()

	// Plaintext bodies and a master token: loopback or an SSH tunnel, never a
	// public interface. See docs/12-stages.md#poc.
	if err := loopbackOnly(*addr); err != nil {
		log.Fatal(err)
	}
	tcp, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}
	os.Remove(*sock)
	if err := os.MkdirAll(filepath.Dir(*sock), 0o700); err != nil {
		log.Fatalf("socket dir: %v", err)
	}
	unix, err := net.Listen("unix", *sock)
	if err != nil {
		log.Fatalf("listen %s: %v", *sock, err)
	}
	os.Chmod(*sock, 0o600)

	srv := &http.Server{Handler: handler}
	go srv.Serve(tcp)
	go srv.Serve(unix)
	log.Printf("agent-busd on http://%s and %s (token %s)", *addr, *sock, *tokenF)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	unix.Close()
	os.Remove(*sock)
	log.Print("stopped")
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

// loadToken reads the master token, creating one on first run. The same file
// is what `static-token` over SSH will hand out. See docs/02-access.md.
func loadToken(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		if t := string(trim(b)); t != "" {
			return t, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	var raw [24]byte
	rand.Read(raw[:])
	token := hex.EncodeToString(raw[:])
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return token, os.WriteFile(path, []byte(token+"\n"), 0o600)
}

func trim(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == ' ' || b[len(b)-1] == '\t') {
		b = b[:len(b)-1]
	}
	return b
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func defaultSocket() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "agent-bus", "bus.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("agent-bus-%d.sock", os.Getuid()))
}

func defaultTokenFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "agent-bus", "token")
}
