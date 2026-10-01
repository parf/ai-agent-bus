package core

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A card carries what the MCP specification requires and nothing a card
// cannot have (docs/03-records-resource.md#what-a-resource-is).
func TestAResourceCardIsChecked(t *testing.T) {
	b := New()
	known(t, b, "o@h")
	card := func(d protocol.Resource) protocol.Record {
		return protocol.Record{Name: "notes@h", Kind: protocol.KindResource, Owner: "o@h", Resource: &d}
	}
	for _, tc := range []struct {
		why string
		r   protocol.Record
	}{
		{"no descriptor", protocol.Record{Name: "notes@h", Kind: protocol.KindResource, Owner: "o@h"}},
		{"no uri", card(protocol.Resource{Source: "#reader@h"})},
		{"a relative uri", card(protocol.Resource{URI: "notes/a.md", Source: "#reader@h"})},
		{"a template whose scheme is not one", card(protocol.Resource{URI: "1md://notes/{p}", Template: true, Source: "#reader@h"})},
		{"a template part in a plain uri", card(protocol.Resource{URI: "md://notes/{path}", Source: "#reader@h"})},
		{"a template without a template part", card(protocol.Resource{URI: "md://notes/a.md", Template: true, Source: "#reader@h"})},
		{"a template with a size", card(protocol.Resource{URI: "md://notes/{path}", Template: true, Size: 10, Source: "#reader@h"})},
		{"a negative size", card(protocol.Resource{URI: "md://notes/a.md", Size: -1, Source: "#reader@h"})},
		{"icons that are not an array", card(protocol.Resource{URI: "md://notes/a.md", Icons: json.RawMessage(`{}`), Source: "#reader@h"})},
		{"annotations that are not an object", card(protocol.Resource{URI: "md://notes/a.md", Annotations: json.RawMessage(`[]`), Source: "#reader@h"})},
		{"no source for a non-https card", card(protocol.Resource{URI: "md://notes/a.md"})},
		{"no source for an https template", card(protocol.Resource{URI: "https://x.example/{p}", Template: true})},
		{"a queue setting", func() protocol.Record {
			r := card(protocol.Resource{URI: "md://notes/a.md", Source: "#reader@h"})
			r.TTL = "1h"
			return r
		}()},
		{"an address", func() protocol.Record {
			r := card(protocol.Resource{URI: "md://notes/a.md", Source: "#reader@h"})
			r.Addr, r.Proto = "x", "mcp"
			return r
		}()},
		{"a descriptor on another kind", protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "o@h", Resource: &protocol.Resource{URI: "md://x"}}},
	} {
		if _, err := b.Register(tc.r); !errors.Is(err, ErrKind) {
			t.Errorf("%s: err = %v, want ErrKind", tc.why, err)
		}
	}
	for _, d := range []protocol.Resource{
		{URI: "https://example.com/readme.md"},
		{URI: "md://notes/a.md", Source: "#reader@h", MimeType: "text/markdown", Size: 42, Title: "A"},
		{URI: "md://notes/{+path}", Template: true, Source: "mcp-notes@h"},
	} {
		if _, err := b.Register(card(d)); err != nil {
			t.Errorf("%+v refused: %v", d, err)
		}
	}
}

// A card has no queue: nothing is delivered to it.
func TestAResourceHasNoQueue(t *testing.T) {
	if onBus(protocol.Record{Kind: protocol.KindResource}) {
		t.Fatal("a resource has a queue here")
	}
}

// Optional JSON is stored compact, so a spaced one from a CLI is accepted.
func TestAResourceStoresCompactJSON(t *testing.T) {
	b := New()
	known(t, b, "o@h")
	r, err := b.Register(protocol.Record{Name: "n@h", Kind: protocol.KindResource, Owner: "o@h",
		Resource: &protocol.Resource{URI: "md://n", Source: "#r@h", Annotations: json.RawMessage(`{ "priority": 0.5 }`)}})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(r.Resource.Annotations); got != `{"priority":0.5}` {
		t.Fatalf("stored %s", got)
	}
}
