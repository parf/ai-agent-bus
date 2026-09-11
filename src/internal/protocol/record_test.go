package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// What a query gets instead of a configuration: a digest, and never the
// bytes. The digest is over the bytes as stored; the bus compacts them on the
// way in, which is what makes two spellings of one configuration one digest
// (TestAConfigurationIsStoredCompacted).
func TestPublicReplacesTheConfigurationWithItsDigest(t *testing.T) {
	compact := Record{Name: "svc@h", Config: json.RawMessage(`{"k":"v"}`)}

	// Marshalling is what compacts, so go through it the way a face does.
	digest := func(r Record) string {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var back Record
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatal(err)
		}
		if back.Config != nil {
			t.Fatalf("the configuration crossed the wire: %s", back.Config)
		}
		return back.ConfigSHA
	}

	a := digest(compact.Public())
	want := sha256.Sum256([]byte(`{"k":"v"}`))
	if a != hex.EncodeToString(want[:]) {
		t.Fatalf("digest is not over the stored bytes: %s", a)
	}

	other := Record{Name: "svc@h", Config: json.RawMessage(`{"k":"w"}`)}.Public()
	if other.ConfigSHA == a {
		t.Fatal("two different configurations share a digest")
	}
	if none := (Record{Name: "svc@h"}).Public(); none.ConfigSHA != "" {
		t.Fatalf("an unconfigured service got a digest: %q", none.ConfigSHA)
	}
}
