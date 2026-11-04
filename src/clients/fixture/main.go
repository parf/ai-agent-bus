// A hermetic real-daemon fixture for client protocol conformance checks.
// It listens only on ephemeral loopback ports and exits when stdin closes.
package main

import (
	"context"
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
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/bigmem"
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
	bus.Persistence(memory.NewState())
	must(bus.EstablishDaemon("owner@fixture", nil))
	bus.SetBigStore(bigmem.New())
	tokens, err := auth.Load(memory.NewTokens())
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
	// The daemon's own #bus agent and its reader, as the real daemon starts
	// them, so event calls reach the landed event verbs and never a fake.
	must(bus.EstablishDaemonAgent("fixture.example"))
	stopBus := s.ServeDaemonAgent(context.Background())
	defer stopBus()
	must(busReady(bus))
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

// busReady proves the #bus reader answers before the fixture says it is ready:
// a status call as the daemon Owner, answered into the Owner's own inbox. It
// fails the fixture, with a bounded wait, rather than leave a client to time out.
func busReady(bus *core.Bus) error {
	sent, err := bus.Send(protocol.Envelope{From: "owner@fixture", To: core.DaemonAgent, Body: "status",
		Topic: "ready", Tag: "fixture-ready", Wait: "5s"})
	if err != nil {
		return fmt.Errorf("fixture: #bus refused the readiness call: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	answer, err := bus.Consume(ctx, "owner@fixture", "ready", "fixture-ready", true, false)
	if err != nil || answer.Re != sent.ID {
		return fmt.Errorf("fixture: #bus did not answer its readiness call: %v", err)
	}
	return nil
}
