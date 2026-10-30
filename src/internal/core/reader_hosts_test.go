package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func hostsOf(t *testing.T, b *Bus, name string) protocol.Record {
	t.Helper()
	r, ok := b.Lookup("#svc@h", name)
	if !ok {
		t.Fatalf("no record %s", name)
	}
	return r
}

func readFrom(t *testing.T, b *Bus, host string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(WithHost(context.Background(), host), 20*time.Millisecond)
	defer cancel()
	b.Consume(ctx, "#svc@h", "", "", false, true)
}

// A read that states its host puts it in the record's listing with when it
// last read; a name no host has read has no list at all.
func TestAReadStatingItsHostIsListed(t *testing.T) {
	b := newBusWith(t, "#svc@h")
	if r := hostsOf(t, b, "#svc@h"); len(r.Hosts) != 0 || r.HostsCut {
		t.Fatalf("a name nobody read from lists %+v", r.Hosts)
	}
	before := time.Now()
	readFrom(t, b, "srv2")
	readFrom(t, b, "srv1")
	r := hostsOf(t, b, "#svc@h")
	if len(r.Hosts) != 2 || r.Hosts[0].Name != "srv1" || r.Hosts[1].Name != "srv2" {
		t.Fatalf("hosts %+v, want srv1 and srv2 by name", r.Hosts)
	}
	for _, h := range r.Hosts {
		if h.LastRead.Before(before) {
			t.Fatalf("%s last read %v is before the read began at %v", h.Name, h.LastRead, before)
		}
	}
	// A read without a host adds none.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	b.Consume(ctx, "#svc@h", "", "", false, true)
	if got := len(hostsOf(t, b, "#svc@h").Hosts); got != 2 {
		t.Fatalf("a read with no host changed the list to %d", got)
	}
}

// A host's last read is noted again when a read ends, so a member whose places
// are all busy, and so has no read outstanding for a while, is not dropped for
// the length of that read.
func TestAHostIsRefreshedWhenItsReadEnds(t *testing.T) {
	b := newBusWith(t, "#svc@h")
	ctx, cancel := context.WithTimeout(WithHost(context.Background(), "srv1"), 150*time.Millisecond)
	began := time.Now()
	b.Consume(ctx, "#svc@h", "", "", false, true)
	cancel()
	r := hostsOf(t, b, "#svc@h")
	if len(r.Hosts) != 1 || r.Hosts[0].LastRead.Sub(began) < 100*time.Millisecond {
		t.Fatalf("hosts %+v: the last read should be when the read ended, not when it began (%v)", r.Hosts, began)
	}
}

// A host whose last read is older than the window is gone from the listing.
func TestAHostPastTheWindowIsForgotten(t *testing.T) {
	b := newBusWith(t, "#svc@h")
	readFrom(t, b, "srv1")
	readFrom(t, b, "srv2")
	b.mu.Lock()
	b.inboxes["#svc@h"].hosts["srv1"] = time.Now().Add(-hostWindow - time.Second)
	b.unlock()
	r := hostsOf(t, b, "#svc@h")
	if len(r.Hosts) != 1 || r.Hosts[0].Name != "srv2" {
		t.Fatalf("hosts %+v, want only srv2 (srv1 read over %v ago)", r.Hosts, hostWindow)
	}
}

// Past the bound the oldest host is forgotten and the listing says it is cut.
func TestMoreThanTheBoundOfHostsIsCutAndSaysSo(t *testing.T) {
	b := newBusWith(t, "#svc@h")
	for i := 0; i < maxHosts; i++ {
		readFrom(t, b, fmt.Sprintf("h%02d", i))
	}
	if r := hostsOf(t, b, "#svc@h"); len(r.Hosts) != maxHosts || r.HostsCut {
		t.Fatalf("%d hosts, cut %v at the bound itself", len(r.Hosts), r.HostsCut)
	}
	readFrom(t, b, "extra")
	r := hostsOf(t, b, "#svc@h")
	if len(r.Hosts) != maxHosts || !r.HostsCut {
		t.Fatalf("%d hosts, cut %v after one past the bound; want %d and cut", len(r.Hosts), r.HostsCut, maxHosts)
	}
	for _, h := range r.Hosts {
		if h.Name == "h00" {
			t.Fatal("the oldest host was kept and a newer one forgotten")
		}
	}
}

// A hostname is 1 to 64 characters of letters, digits, '.' and '-', and anything
// else is refused by name, never silently dropped.
func TestAHostnameIsCheckedAndRefusedByName(t *testing.T) {
	for _, host := range []string{"srv1", "a", "web-1.example.net", "UPPER", strings.Repeat("a", 64)} {
		if err := CheckHost(host); err != nil {
			t.Errorf("%q refused: %v", host, err)
		}
	}
	for _, host := range []string{"", "has space", "under_score", "slash/x", "ünï", strings.Repeat("a", 65), "a,b"} {
		err := CheckHost(host)
		if !errors.Is(err, ErrBadName) || !strings.Contains(err.Error(), "host") {
			t.Errorf("%q: %v, want a refusal naming the host", host, err)
		}
	}
}

// A host is a label: it is not a registration field, so stating one when
// registering is refused, and nothing about it reaches the stored record.
func TestHostsAreNotRegisterable(t *testing.T) {
	b := New()
	known(t, b, "owner@h")
	_, err := b.Register(protocol.Record{Name: "#x@h", Kind: "agent", Owner: "owner@h", Hosts: []protocol.ReaderHost{{Name: "srv1"}}})
	if err == nil || !strings.Contains(err.Error(), "hosts") {
		t.Fatalf("registering a hosts list: %v, want it refused by name", err)
	}
}

// A host is noted only for a read the ACL allowed: a caller refused the inbox
// leaves nothing in its listing, and the owner's own read lists its host
// without adding a reader or a queued message.
func TestARefusedReadersHostIsNotListed(t *testing.T) {
	b := New()
	known(t, b, "owner@h", "intruder@h")
	if _, err := b.Register(protocol.Record{Name: "#priv@h", Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"owner@h"}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(WithHost(context.Background(), "evil"), 50*time.Millisecond)
	defer cancel()
	if _, err := b.ConsumeAs(ctx, "intruder@h", "#priv@h", "", "", false, false); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("the intruder's read: %v, want it refused", err)
	}
	if r, ok := b.Lookup("owner@h", "#priv@h"); !ok || len(r.Hosts) != 0 {
		t.Fatalf("a refused read listed %+v", r.Hosts)
	}
	own, stop := context.WithTimeout(WithHost(context.Background(), "srv1"), 20*time.Millisecond)
	defer stop()
	b.ConsumeAs(own, "owner@h", "#priv@h", "", "", false, false)
	r, ok := b.Lookup("owner@h", "#priv@h")
	if !ok || len(r.Hosts) != 1 || r.Hosts[0].Name != "srv1" {
		t.Fatalf("the owner's read listed %+v, want srv1", r.Hosts)
	}
	if r.Readers == nil || *r.Readers != 0 || r.Queued != 0 {
		t.Fatalf("a listed host changed readers %v or queued %d", r.Readers, r.Queued)
	}
	// Hosts are visible exactly where readers are: a caller who may not see the
	// record has no lookup at all.
	if _, ok := b.Lookup("intruder@h", "#priv@h"); ok {
		t.Fatal("a caller who may not see the record looked it up with its hosts")
	}
}
