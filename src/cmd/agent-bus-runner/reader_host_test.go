package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
)

// A served agent's reads state the runner's host and, when agents.json says
// share, say they are one of a pool: its listing shows the host, and a second
// unfiltered reader that also shares is let in beside it while one that does not
// is refused.
func TestRunnerReadsStateTheirHostAndShare(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "")
	write := func(entry map[string]any) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(r.host, "agents.json"), mustJSON(map[string]any{"fixture": entry}), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// The first reader is stopped and its poll dropped before the pool reader starts.
	write(map[string]any{"sandbox": "off", "enabled": false})
	r.reload()
	for end := time.Now().Add(5 * time.Second); b.Status().Waiting != 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("%d readers still waiting after the agent was disabled", b.Status().Waiting)
		}
	}
	r.readHost = "pool-host"
	write(map[string]any{"sandbox": "off", "share": true})
	r.reload()
	var hosts []string
	for end := time.Now().Add(5 * time.Second); len(hosts) == 0; time.Sleep(10 * time.Millisecond) {
		rec, _ := b.Lookup("owner@h", "#fixture@h")
		for _, h := range rec.Hosts {
			hosts = append(hosts, h.Name)
		}
		if time.Now().After(end) {
			t.Fatal("the runner's read never stated its host")
		}
	}
	if len(hosts) != 1 || hosts[0] != "pool-host" {
		t.Fatalf("hosts %v, want pool-host", hosts)
	}
	wait := func(share bool) error {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		_, err := b.ConsumeAs(ctx, "owner@h", "#fixture@h", "", "", false, share)
		return err
	}
	if err := wait(true); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a second sharing reader beside the pool member: %v, want it admitted and left waiting", err)
	}
	if err := wait(false); !errors.Is(err, core.ErrTwoReads) {
		t.Fatalf("a second reader that does not share: %v, want it refused as a second reader", err)
	}
}
