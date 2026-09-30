package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// validResource is what a 📚 card must satisfy: the MCP descriptor the latest
// specification requires, and nothing a card cannot have — it has no queue,
// no address of its own and no private values
// (docs/03-records.md#resource-records).
func validResource(r protocol.Record) error {
	if r.Kind != protocol.KindResource {
		if r.Resource != nil {
			return fmt.Errorf("%w: only a resource carries a resource descriptor", ErrKind)
		}
		return nil
	}
	d := r.Resource
	if d == nil || d.URI == "" {
		return fmt.Errorf("%w: a resource needs its uri", ErrKind)
	}
	if r.Addr != "" || r.Proto != "" || r.TTL != "" || r.Bound != 0 || r.Full != "" {
		return fmt.Errorf("%w: a resource has no queue or address here; its source answers reads", ErrKind)
	}
	scheme, _, ok := strings.Cut(d.URI, ":")
	if !ok || !validScheme(scheme) {
		return fmt.Errorf("%w: a resource uri is an absolute URI, like md://notes/a.md", ErrKind)
	}
	if d.Template {
		if !strings.Contains(d.URI, "{") || d.Size != 0 {
			return fmt.Errorf("%w: a resource template is an RFC 6570 URI template and has no size", ErrKind)
		}
	} else if _, err := url.Parse(d.URI); err != nil || strings.Contains(d.URI, "{") {
		return fmt.Errorf("%w: a resource uri is an RFC 3986 URI; a {…} part belongs to a template", ErrKind)
	}
	if d.Size < 0 {
		return fmt.Errorf("%w: a resource size is bytes, not negative", ErrKind)
	}
	if !jsonOf(d.Icons, '[') || !jsonOf(d.Annotations, '{') {
		return fmt.Errorf("%w: resource icons are a compact JSON array and annotations a compact JSON object", ErrKind)
	}
	switch {
	case d.Source == "" && (d.Template || !strings.EqualFold(scheme, "https")):
		return fmt.Errorf("%w: a resource names the agent or mcp service that answers it; only a plain https:// card may name none", ErrKind)
	case d.Source != "":
		if _, err := canon(d.Source); err != nil {
			return err
		}
	}
	return nil
}

// validScheme is RFC 3986's scheme: a letter, then letters, digits, + - .
func validScheme(s string) bool {
	for i, c := range s {
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !letter && (i == 0 || !(c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.')) {
			return false
		}
	}
	return s != ""
}

// jsonOf says an optional value is compact JSON opening with open.
func jsonOf(raw json.RawMessage, open byte) bool {
	if len(raw) == 0 {
		return true
	}
	var compact bytes.Buffer
	return json.Compact(&compact, raw) == nil && bytes.Equal(compact.Bytes(), raw) && raw[0] == open
}

// compactJSON is raw compacted, or raw unchanged when it is not JSON, for
// validResource to refuse.
func compactJSON(raw json.RawMessage) json.RawMessage {
	var b bytes.Buffer
	if json.Compact(&b, raw) != nil {
		return raw
	}
	return b.Bytes()
}
