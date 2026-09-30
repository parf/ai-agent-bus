// The key-value API: each record's store of string, int and JSON values. The
// registry decides who may reach a store and the store makes each edit
// atomic; this file is the wire between them
// (Plans/R1.0-Release/kv.md#per-record-storage).
package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// kvAnswer is one value: a string is a JSON string, or value_base64 when its
// bytes are not UTF-8; an int is a number; a JSON value is the object itself.
type kvAnswer struct {
	Record      string          `json:"record"`
	Kind        ports.KVKind    `json:"kind"`
	Name        string          `json:"name"`
	Value       json.RawMessage `json:"value,omitempty"`
	ValueBase64 string          `json:"value_base64,omitempty"`
	Deleted     *bool           `json:"deleted,omitempty"`
}

// kvBody is the largest write body: a value at its limit, every byte escaped
// as \u00XX, and room for the rest of the request.
const kvBody = 6*core.KVMaxValue + 64<<10

// readStrictUpTo is readStrict for a body larger than 1 MiB: a key-value
// write carries a value up to its limit, escaped.
func (s *Server) readStrictUpTo(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		s.refuse(w, http.StatusBadRequest, "malformed", "bad json: "+err.Error())
		return false
	}
	return true
}

func kvKind(s string) (ports.KVKind, error) {
	switch k := ports.KVKind(s); k {
	case "":
		return ports.KVString, nil
	case ports.KVString, ports.KVInt, ports.KVJSON:
		return k, nil
	}
	return "", fmt.Errorf("%w: kind is string, int or json", core.ErrKVValue)
}

func (s *Server) kvGet(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	q := r.URL.Query()
	kind, err := kvKind(q.Get("kind"))
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	record, name := q.Get("record"), q.Get("name")
	v, err := s.bus.KVGet(caller.String(), record, kind, name)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	a := kvAnswer{Record: record, Kind: kind, Name: name}
	switch {
	case kind == ports.KVInt:
		a.Value = json.RawMessage(strconv.FormatInt(v.Int, 10))
	case kind == ports.KVJSON:
		a.Value = v.Bytes
	case utf8.Valid(v.Bytes):
		a.Value, _ = json.Marshal(string(v.Bytes))
	default:
		a.ValueBase64 = base64.StdEncoding.EncodeToString(v.Bytes)
	}
	s.reply(w, a, nil)
}

// kvValue reads a set's value as its kind takes it.
func kvValue(kind ports.KVKind, raw json.RawMessage, b64 string) (ports.KVValue, error) {
	if b64 != "" {
		if kind != ports.KVString || len(raw) != 0 {
			return ports.KVValue{}, fmt.Errorf("%w: value_base64 is a string's bytes, instead of value", core.ErrKVValue)
		}
		b, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return ports.KVValue{}, fmt.Errorf("%w: value_base64 is not base64", core.ErrKVValue)
		}
		return ports.KVValue{Bytes: b}, nil
	}
	if len(raw) == 0 {
		return ports.KVValue{}, fmt.Errorf("%w: a set carries a value", core.ErrKVValue)
	}
	switch kind {
	case ports.KVInt:
		n, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil {
			return ports.KVValue{}, fmt.Errorf("%w: an int value is a 64-bit integer", core.ErrKVValue)
		}
		return ports.KVValue{Int: n}, nil
	case ports.KVJSON:
		return ports.KVValue{Bytes: raw}, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return ports.KVValue{}, fmt.Errorf("%w: a string value is a JSON string, or value_base64", core.ErrKVValue)
	}
	return ports.KVValue{Bytes: []byte(text)}, nil
}

func (s *Server) kvSet(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Record      string          `json:"record"`
		Kind        string          `json:"kind"`
		Name        string          `json:"name"`
		Value       json.RawMessage `json:"value"`
		ValueBase64 string          `json:"value_base64"`
		How         string          `json:"how"`
	}
	if !s.readStrictUpTo(w, r, &in, kvBody) {
		return
	}
	kind, err := kvKind(in.Kind)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	v, err := kvValue(kind, in.Value, in.ValueBase64)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	err = s.bus.KVSet(caller.String(), in.Record, kind, in.Name, v, in.How)
	s.reply(w, kvAnswer{Record: in.Record, Kind: kind, Name: in.Name}, err)
}

func (s *Server) kvDelete(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Record string `json:"record"`
		Kind   string `json:"kind"`
		Name   string `json:"name"`
	}
	if !s.readStrict(w, r, &in) {
		return
	}
	kind, err := kvKind(in.Kind)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	was, err := s.bus.KVDelete(caller.String(), in.Record, kind, in.Name)
	s.reply(w, kvAnswer{Record: in.Record, Kind: kind, Name: in.Name, Deleted: &was}, err)
}

func (s *Server) kvInc(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Record string `json:"record"`
		Name   string `json:"name"`
		N      *int64 `json:"n"`
	}
	if !s.readStrict(w, r, &in) {
		return
	}
	n := int64(1)
	if in.N != nil {
		n = *in.N
	}
	now, err := s.bus.KVIntInc(caller.String(), in.Record, in.Name, n)
	s.reply(w, kvAnswer{Record: in.Record, Kind: ports.KVInt, Name: in.Name, Value: json.RawMessage(strconv.FormatInt(now, 10))}, err)
}

func (s *Server) kvJSON(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Record string      `json:"record"`
		Name   string      `json:"name"`
		Ops    []core.KVOp `json:"ops"`
	}
	if !s.readStrictUpTo(w, r, &in, kvBody) {
		return
	}
	results, err := s.bus.KVJSON(caller.String(), in.Record, in.Name, in.Ops)
	s.reply(w, struct {
		Record  string          `json:"record"`
		Name    string          `json:"name"`
		Results []core.KVResult `json:"results"`
	}{in.Record, in.Name, results}, err)
}

// kvListEntry is one name in a listing: its size and a preview, a string's
// as text or base64 as a value is, an int's the value itself.
type kvListEntry struct {
	Name          string          `json:"name"`
	Size          int             `json:"size"`
	Preview       json.RawMessage `json:"preview,omitempty"`
	PreviewBase64 string          `json:"preview_base64,omitempty"`
}

// kvList answers one record's store, or with no record every store the
// caller may use, as /holders does for locks.
func (s *Server) kvList(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	record := r.URL.Query().Get("record")
	if record == "" {
		rows, err := s.bus.KVStores(caller.String())
		s.reply(w, rows, err)
		return
	}
	l, err := s.bus.KVList(caller.String(), record)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	values := map[ports.KVKind][]kvListEntry{}
	for kind, entries := range l.Values {
		out := make([]kvListEntry, 0, len(entries))
		for _, e := range entries {
			le := kvListEntry{Name: e.Name, Size: e.Size}
			// A preview cut mid-character is still text: the partial
			// character goes, not the whole preview into base64.
			if cut := e.Preview.Bytes; e.Size > len(cut) {
				for i := 0; i < 3 && len(cut) > 0 && !utf8.Valid(cut); i++ {
					cut = cut[:len(cut)-1]
				}
				if utf8.Valid(cut) {
					e.Preview.Bytes = cut
				}
			}
			switch {
			case kind == ports.KVInt:
				le.Preview = json.RawMessage(strconv.FormatInt(e.Preview.Int, 10))
			case utf8.Valid(e.Preview.Bytes):
				le.Preview, _ = json.Marshal(string(e.Preview.Bytes))
			default:
				le.PreviewBase64 = base64.StdEncoding.EncodeToString(e.Preview.Bytes)
			}
			out = append(out, le)
		}
		values[kind] = out
	}
	s.reply(w, struct {
		Record string                         `json:"record"`
		Kind   string                         `json:"kind"`
		Values map[ports.KVKind][]kvListEntry `json:"values"`
	}{l.Record, l.Kind, values}, nil)
}
