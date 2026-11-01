package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// All request schemas ignore unknown members. Known members retain their
// types and may occur only once (including case-insensitive spellings that
// encoding/json accepts). Map keys and RawMessage remain caller data.
func decodeRequest(r io.Reader, out any) error {
	dec := json.NewDecoder(r)
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one JSON body")
	}
	if err := checkMembers(json.NewDecoder(bytes.NewReader(raw)), reflect.TypeOf(out)); err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

var rawJSONType = reflect.TypeOf(json.RawMessage{})

func checkMembers(dec *json.Decoder, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if reflect.PointerTo(t).Implements(reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()) || (t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8) || t == rawJSONType || (t.Kind() != reflect.Struct && t.Kind() != reflect.Slice && t.Kind() != reflect.Array && t.Kind() != reflect.Map) {
		return dec.Decode(new(json.RawMessage))
	}
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == nil {
		return nil
	}
	if t.Kind() == reflect.Map {
		if tok != json.Delim('{') {
			return fmt.Errorf("expected JSON object")
		}
		for dec.More() {
			if _, err := dec.Token(); err != nil {
				return err
			}
			if err := checkMembers(dec, t.Elem()); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	if t.Kind() != reflect.Struct {
		if tok != json.Delim('[') {
			return fmt.Errorf("expected JSON array")
		}
		for dec.More() {
			if err := checkMembers(dec, t.Elem()); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	if tok != json.Delim('{') {
		return fmt.Errorf("expected JSON object")
	}
	fields := reflect.VisibleFields(t)
	seen := map[int]bool{}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		key := token.(string)
		match := -1
		// Exact matches win, as in encoding/json; fall back to a folded match.
		for i, f := range fields {
			if f.PkgPath != "" || (f.Anonymous && strings.Split(f.Tag.Get("json"), ",")[0] == "") {
				continue
			}
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			if key == name {
				match = i
				break
			}
			if match < 0 && strings.EqualFold(key, name) {
				match = i
			}
		}
		if match < 0 {
			if err := dec.Decode(new(json.RawMessage)); err != nil {
				return err
			}
			continue
		}
		if seen[match] {
			return fmt.Errorf("duplicate field %q", key)
		}
		seen[match] = true
		if err := checkMembers(dec, fields[match].Type); err != nil {
			return err
		}
	}
	_, err = dec.Token()
	return err
}
