package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/bigmem"
)

// bigFixture is kvFixture with a holder bound: alice owns #svc@h, carol is a
// Maintainer through @crew, bob is only admitted by its allow list.
func bigFixture(t *testing.T) *Bus {
	t.Helper()
	b, _ := kvFixture(t)
	b.SetBigStore(bigmem.New())
	return b
}

func put(b *Bus, who, name, data string) error {
	_, _, err := b.BigPut(who, "#svc@h", name, strings.NewReader(data), 0, false, "")
	return err
}

func read(t *testing.T, b *Bus, who, name string) string {
	t.Helper()
	rc, _, err := b.BigGet(who, "#svc@h", name)
	if err != nil {
		t.Fatalf("%s could not get %s: %v", who, name, err)
	}
	defer rc.Close()
	raw, _ := io.ReadAll(rc)
	return string(raw)
}

func TestBigKeysMaintainersWriteAndTheAllowListReads(t *testing.T) {
	b := bigFixture(t)
	for _, who := range []string{"alice@h", "carol@h", "#svc@h"} {
		if err := put(b, who, "k-"+who, "bytes of "+who); err != nil {
			t.Fatalf("%s could not put: %v", who, err)
		}
		if got := read(t, b, who, "k-"+who); got != "bytes of "+who {
			t.Fatalf("%s read %q", who, got)
		}
	}
	// bob is admitted by the allow list: he downloads and lists, nothing else.
	if got := read(t, b, "bob@h", "k-alice@h"); got != "bytes of alice@h" {
		t.Fatalf("an admitted caller read %q", got)
	}
	if _, keys, err := b.BigList("bob@h", "#svc@h"); err != nil || len(keys) != 3 {
		t.Fatalf("an admitted caller's list: %v %v", keys, err)
	}
	if err := put(b, "bob@h", "mine", "x"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("an admitted non-Maintainer's put = %v, want refusal", err)
	}
	if err := put(b, "bob@h", "k-alice@h", "x"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("an admitted non-Maintainer replaced a key: %v", err)
	}
	if _, err := b.BigDelete("bob@h", "#svc@h", "k-alice@h"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("an admitted non-Maintainer's delete = %v, want refusal", err)
	}
	if got := read(t, b, "alice@h", "k-alice@h"); got != "bytes of alice@h" {
		t.Fatalf("a refused put or delete changed the key: %q", got)
	}
	// eve is a stranger: all four are refused.
	if _, _, err := b.BigGet("eve@h", "#svc@h", "k-alice@h"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a stranger's get = %v", err)
	}
	if _, _, err := b.BigList("eve@h", "#svc@h"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a stranger's list = %v", err)
	}
	if err := put(b, "eve@h", "n", "x"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a stranger's put = %v", err)
	}
	if _, err := b.BigDelete("eve@h", "#svc@h", "k-alice@h"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a stranger's delete = %v", err)
	}
	// A Maintainer deletes, and the answer says whether the key was there.
	if was, err := b.BigDelete("carol@h", "#svc@h", "k-alice@h"); err != nil || !was {
		t.Fatalf("delete = %v %v", was, err)
	}
	if was, err := b.BigDelete("carol@h", "#svc@h", "k-alice@h"); err != nil || was {
		t.Fatalf("a second delete = %v %v, want false", was, err)
	}
	if _, _, err := b.BigGet("alice@h", "#svc@h", "k-alice@h"); !errors.Is(err, ErrBigAbsent) {
		t.Fatalf("a deleted key = %v", err)
	}
}

func TestBigKeysAreScopedToTheirRecord(t *testing.T) {
	b := bigFixture(t)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#other@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if err := put(b, "alice@h", "same", "svc"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.BigGet("alice@h", "#other@h", "same"); !errors.Is(err, ErrBigAbsent) {
		t.Fatalf("a key of #svc@h is visible on #other@h: %v", err)
	}
	if _, _, err := b.BigGet("alice@h", "#nobody@h", "same"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("an unknown record = %v", err)
	}
}

func TestBigKeyPutReportsReplaceSizeAndHash(t *testing.T) {
	b := bigFixture(t)
	info, replaced, err := b.BigPut("alice@h", "#svc@h", "k", strings.NewReader("hello"), 0, false, "")
	want := sha256.Sum256([]byte("hello"))
	if err != nil || replaced || info.Size != 5 || info.SHA256 != hex.EncodeToString(want[:]) {
		t.Fatalf("first put = %+v %v %v", info, replaced, err)
	}
	info, replaced, err = b.BigPut("carol@h", "#svc@h", "k", strings.NewReader("hello world"), 0, false, "")
	if err != nil || !replaced || info.Size != 11 {
		t.Fatalf("a Maintainer's replace = %+v %v %v, want replaced and not refused", info, replaced, err)
	}
	if got := read(t, b, "alice@h", "k"); got != "hello world" {
		t.Fatalf("replace left %q", got)
	}
	if _, _, err := b.BigPut("alice@h", "#svc@h", "k", strings.NewReader("other"), 0, false, strings.Repeat("0", 64)); !errors.Is(err, ErrBigValue) {
		t.Fatalf("a wrong SHA-256 = %v", err)
	}
	if got := read(t, b, "alice@h", "k"); got != "hello world" {
		t.Fatalf("a refused put changed the key: %q", got)
	}
}

func TestBigKeyLimits(t *testing.T) {
	b := bigFixture(t)
	if err := put(b, "alice@h", "empty", ""); !errors.Is(err, ErrBigValue) {
		t.Fatalf("an empty put = %v", err)
	}
	if err := put(b, "alice@h", "", "x"); !errors.Is(err, ErrBigValue) {
		t.Fatalf("an empty name = %v", err)
	}
	if err := put(b, "alice@h", strings.Repeat("n", BigMaxName+1), "x"); !errors.Is(err, ErrBigValue) {
		t.Fatalf("a long name = %v", err)
	}
	over := bytes.NewReader(make([]byte, BigMaxValue+1))
	if _, _, err := b.BigPut("alice@h", "#svc@h", "big", over, 0, false, ""); !errors.Is(err, ports.ErrBigTooLarge) {
		t.Fatalf("one byte over the cap = %v", err)
	}
	exact := bytes.NewReader(make([]byte, BigMaxValue))
	if _, _, err := b.BigPut("alice@h", "#svc@h", "exact", exact, 0, false, ""); err != nil {
		t.Fatalf("a value at the cap = %v", err)
	}
	for _, ttl := range []time.Duration{time.Millisecond, BigMaxTTL + time.Second} {
		if _, _, err := b.BigPut("alice@h", "#svc@h", "t", strings.NewReader("x"), ttl, false, ""); !errors.Is(err, ErrBigValue) {
			t.Fatalf("ttl %v = %v", ttl, err)
		}
	}
}

func TestBigKeyCountLimit(t *testing.T) {
	b := bigFixture(t)
	for i := 0; i < BigMaxNames; i++ {
		if err := put(b, "alice@h", fmt.Sprintf("key-%d", i), "x"); err != nil {
			t.Fatalf("key %d: %v", i, err)
		}
	}
	if err := put(b, "alice@h", "one-too-many", "x"); !errors.Is(err, ports.ErrBigFull) {
		t.Fatalf("key %d = %v", BigMaxNames+1, err)
	}
	if err := put(b, "alice@h", "key-0", "replaced"); err != nil {
		t.Fatalf("a replace at the count limit = %v", err)
	}
}

func TestBigKeysExpire(t *testing.T) {
	b := bigFixture(t)
	now := time.Now()
	b.clock = func() time.Time { return now }
	if _, _, err := b.BigPut("alice@h", "#svc@h", "short", strings.NewReader("x"), time.Minute, false, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.BigPut("alice@h", "#svc@h", "default", strings.NewReader("x"), 0, false, ""); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute + time.Second)
	if _, _, err := b.BigGet("alice@h", "#svc@h", "short"); !errors.Is(err, ErrBigAbsent) {
		t.Fatalf("an expired key = %v", err)
	}
	if _, keys, _ := b.BigList("alice@h", "#svc@h"); len(keys) != 1 || keys[0].Name != "default" {
		t.Fatalf("list after expiry = %+v", keys)
	}
	now = now.Add(BigDefaultTTL)
	if _, keys, _ := b.BigList("alice@h", "#svc@h"); len(keys) != 0 {
		t.Fatalf("the default lifetime did not expire: %+v", keys)
	}
}

func TestBigKeysNeedAHolder(t *testing.T) {
	b, _ := kvFixture(t)
	if err := put(b, "alice@h", "k", "x"); !errors.Is(err, ErrBigNoStore) {
		t.Fatalf("no holder = %v", err)
	}
}
