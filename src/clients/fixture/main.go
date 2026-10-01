// A hermetic real-daemon fixture for client protocol conformance checks.
// It listens only on ephemeral loopback ports and exits when stdin closes.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	bus := core.New()
	bus.Restore(ports.Snapshot{Clean: true,
		Users: []protocol.User{{Name: "owner@fixture", Status: "active"}},
		Records: []protocol.Record{{Name: "owner@fixture", Owner: "owner@fixture", Kind: protocol.KindUser,
			Personal: true, Full: protocol.OverflowStrict, At: time.Now()}},
	})
	bus.Persistence(memory.NewState())
	tokens, err := auth.Load(memory.NewTokens(), "owner@fixture")
	must(err)
	s := api.New(bus, tokens, "owner@fixture")
	defer s.Close()
	credentials := map[string]string{}
	for _, name := range []string{"owner@fixture", "#caller@fixture", "#worker@fixture", "#helper@fixture", "#outsider@fixture"} {
		if name != "owner@fixture" {
			_, err = bus.Register(protocol.Record{Name: name, Owner: "owner@fixture", Kind: protocol.KindAgent,
				Allow: []string{"*"}, Full: protocol.OverflowStrict})
			must(err)
		}
		credentials[name], err = tokens.Issue(name)
		must(err)
	}
	_, err = bus.Register(protocol.Record{Name: "jobs@fixture", Owner: "owner@fixture", Kind: protocol.KindQueue,
		Allow: []string{"*"}, Full: protocol.OverflowStrict})
	must(err)
	for _, name := range []string{"#worker@fixture", "#helper@fixture"} {
		_, err = bus.Configure(name, "owner@fixture", json.RawMessage(`{"workers":1,"label":"fixture"}`))
		must(err)
		_, err = bus.SetSecret(name, "owner@fixture", "TOKEN=dummy-fixture-only\n")
		must(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	must(err)
	server := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(l) }()
	var local *http.Server
	if len(os.Args) > 1 {
		unix, e := net.Listen("unix", os.Args[1])
		must(e)
		defer os.Remove(os.Args[1])
		local = &http.Server{Handler: s.HandlerFor(protocol.Name{Local: "owner", Realm: "fixture"}), ReadHeaderTimeout: 5 * time.Second}
		go func() { _ = local.Serve(unix) }()
	}
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{"addr": "http://" + l.Addr().String(), "tokens": credentials}))
	_, _ = io.Copy(io.Discard, os.Stdin)
	_ = server.Close()
	if local != nil {
		_ = local.Close()
	}
}
