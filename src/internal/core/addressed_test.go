package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// One inbox that many names forward to is read one address at a time: a
// read for #b takes #b's message past #a's, a read for the inbox's own name
// takes what was sent to it directly, and a read waiting for one address is
// a reader in the listing (Plans/R1.0-Release/runner-implementation.md#many-names-into-one-inbox).
func TestAReadForOneAddressTakesOnlyItsMessages(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "caller@h", "#runner@h", "#a@h", "#b@h")
	delivers(t, b, fixtureOwner, "#a@h", "#runner@h")
	delivers(t, b, fixtureOwner, "#b@h", "#runner@h")
	for _, to := range []string{"#a@h", "#b@h", "#runner@h"} {
		if _, err := b.Send(protocol.Envelope{From: "caller@h", To: to, Body: "for " + to}); err != nil {
			t.Fatalf("send to %s: %v", to, err)
		}
	}
	read := func(addressed string, wait time.Duration) (protocol.Envelope, error) {
		ctx, cancel := context.WithTimeout(context.Background(), wait)
		defer cancel()
		return b.ConsumeAddressed(ctx, "#runner@h", "#runner@h", "", "", addressed, false, false)
	}
	for _, addressed := range []string{"#b@h", "#runner@h", "#a@h"} {
		e, err := read(addressed, time.Second)
		if err != nil || e.Body != "for "+addressed {
			t.Fatalf("a read for %s took %q, %v", addressed, e.Body, err)
		}
	}

	// Waiting: a read for #a is not handed #b's message, and is a reader.
	got := make(chan protocol.Envelope, 1)
	go func() {
		e, err := read("#a@h", 5*time.Second)
		if err == nil {
			got <- e
		}
	}()
	for i := 0; ; i++ {
		if r, _ := b.Lookup("admin@h", "#runner@h"); r.Reading {
			break
		}
		if i > 200 {
			t.Fatal("a read waiting for one address is not a reader in the listing")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := b.Send(protocol.Envelope{From: "caller@h", To: "#b@h", Body: "late b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "caller@h", To: "#a@h", Body: "late a"}); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-got:
		if e.Body != "late a" {
			t.Fatalf("the waiting read for #a was handed %q", e.Body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the waiting read for #a got nothing")
	}
	if _, err := read("#a@h", 50*time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a second read for #a found something: %v", err)
	}
	if e, err := read("#b@h", time.Second); err != nil || e.Body != "late b" {
		t.Fatalf("#b's message was not left for #b: %q, %v", e.Body, err)
	}
}
