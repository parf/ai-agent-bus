// The big-key API: values too large for a message, uploaded and downloaded as
// raw bytes and passed between agents by key name. The registry decides who
// may reach a record's keys; this file is the wire (Plans/R1.0-Release/big-keys.md).
package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

var hexSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// bigRow is one key as it is answered.
type bigRow struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Held    string `json:"held"`
	Expires string `json:"expires"`
}

func bigRowOf(i ports.BigInfo) bigRow {
	return bigRow{i.Name, i.Size, i.SHA256, i.Held, i.Expires.UTC().Format(time.RFC3339)}
}

// bigPut takes the raw bytes as the body; the name, lifetime in whole seconds
// and holder travel in the query, an optional SHA-256 in X-Big-Sha256.
func (s *Server) bigPut(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	// A declared length over the cap is refused at once; nothing is read.
	if r.ContentLength > core.BigMaxValue {
		w.Header().Set("Connection", "close")
		s.reply(w, nil, fmt.Errorf("%w: the declared length %d is more than %d bytes", ports.ErrBigTooLarge, r.ContentLength, int64(core.BigMaxValue)))
		return
	}
	q := r.URL.Query()
	var ttl time.Duration
	if v := q.Get("ttl"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 || n > int64(core.BigMaxTTL/time.Second) {
			s.bigRefused(w, r, fmt.Errorf("%w: ttl is whole seconds, 1 to %d", core.ErrBigValue, core.BigMaxTTL/time.Second))
			return
		}
		ttl = time.Duration(n) * time.Second
	}
	memory := false
	switch q.Get("memory") {
	case "", "false":
	case "true":
		memory = true
	default:
		s.bigRefused(w, r, fmt.Errorf("%w: memory is true or false", core.ErrBigValue))
		return
	}
	sum := r.Header.Get("X-Big-Sha256")
	if sum != "" && !hexSHA256.MatchString(sum) {
		s.bigRefused(w, r, fmt.Errorf("%w: X-Big-Sha256 is 64 lowercase hex digits", core.ErrBigValue))
		return
	}
	info, replaced, err := s.bus.BigPut(caller.String(), q.Get("record"), q.Get("name"), r.Body, ttl, memory, sum)
	if err != nil {
		s.bigRefused(w, r, err)
		return
	}
	s.reply(w, struct {
		Record string `json:"record"`
		bigRow
		Replaced bool `json:"replaced"`
	}{q.Get("record"), bigRowOf(info), replaced}, nil)
}

// bigDrain is how long, at most, a refused upload's remaining bytes are read
// and thrown away so the client can see the refusal.
var bigDrain = 2 * time.Second

// bigRefused answers a refusal of an upload. One that arrives before the body
// has been read would reset a client still writing it, so the client never
// saw the status or its cause: the bytes it has declared, at most one key's
// worth, are read and discarded for a fixed time first, then the answer goes
// out and the connection closes. A refusal that already read the body (over a
// cap) needs no drain, and neither does a caller who was never admitted: that
// one is refused before this handler runs, and gets nothing read at all.
func (s *Server) bigRefused(w http.ResponseWriter, r *http.Request, err error) {
	if bigEarly(err) {
		n := r.ContentLength
		if n < 0 || n > core.BigMaxValue {
			n = core.BigMaxValue
		}
		rc := http.NewResponseController(w)
		rc.SetReadDeadline(time.Now().Add(bigDrain)) // a recorder has none; the bound is the length
		io.CopyN(io.Discard, r.Body, n)
		rc.SetReadDeadline(time.Time{})
	}
	w.Header().Set("Connection", "close")
	s.reply(w, nil, err)
}

// bigEarly says whether a refusal of an upload comes before the body is read:
// a bad parameter, no authority, a missing record, an unusable store.
func bigEarly(err error) bool {
	return errors.Is(err, core.ErrBigValue) || errors.Is(err, core.ErrNotOwner) ||
		errors.Is(err, core.ErrUnknown) || errors.Is(err, core.ErrBigNoStore)
}

// bigGet streams the bytes with their size, hash, holder and expiry in
// headers, so the client can check what it received.
func (s *Server) bigGet(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	q := r.URL.Query()
	rc, info, err := s.bus.BigGet(caller.String(), q.Get("record"), q.Get("name"))
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	defer rc.Close()
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Length", strconv.FormatInt(info.Size, 10))
	h.Set("X-Big-Sha256", info.SHA256)
	h.Set("X-Big-Held", info.Held)
	h.Set("X-Big-Expires", info.Expires.UTC().Format(time.RFC3339))
	io.Copy(w, rc)
}

func (s *Server) bigList(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	record := r.URL.Query().Get("record")
	name, keys, err := s.bus.BigList(caller.String(), record)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	rows := make([]bigRow, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, bigRowOf(k))
	}
	s.reply(w, struct {
		Record string   `json:"record"`
		Keys   []bigRow `json:"keys"`
	}{name, rows}, nil)
}

func (s *Server) bigDelete(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Record string `json:"record"`
		Name   string `json:"name"`
	}
	if !s.readStrict(w, r, &in) {
		return
	}
	was, err := s.bus.BigDelete(caller.String(), in.Record, in.Name)
	s.reply(w, struct {
		Record  string `json:"record"`
		Name    string `json:"name"`
		Deleted bool   `json:"deleted"`
	}{in.Record, in.Name, was}, err)
}
