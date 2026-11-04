// Package jsonshape validates canonical JSON object members before decoding.
package jsonshape

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Request schemas accept only their exact member names and types, once each.
// Map keys and RawMessage remain opaque caller data.
// Decode accepts exact member spellings once each; map keys and raw JSON stay opaque.
func Decode(r io.Reader, out any) error {
	dec := json.NewDecoder(r)
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		if err != nil {
			return err
		}
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
		if strings.EqualFold(key, "personal") || strings.EqualFold(key, "reading") {
			return fmt.Errorf("unsupported field %q", key)
		}
		match := -1
		// Only the declared spelling is accepted.
		for i, f := range fields {
			if f.PkgPath != "" || (f.Anonymous && strings.Split(f.Tag.Get("json"), ",")[0] == "") {
				continue
			}
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = strings.ToLower(f.Name)
			}
			if key == name {
				match = i
				break
			}
		}
		if match < 0 {
			return fmt.Errorf("unknown field %q", key)
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
