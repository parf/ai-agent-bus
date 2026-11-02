package api

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/parf/ai-agent-bus/internal/eventlog"
)

// The subscription calls of #bus: `sub add <target> <agent> [level] [/regex/]`,
// `sub list [agent]` and `sub delete <id>`, in text or as a structured call
// with method sub.add, sub.list or sub.delete. They share the event calls'
// bounds (the body is already within 16 KiB and valid text) and validation.

var subDeleteID = regexp.MustCompile(`^[0-9a-f]{12}$`)

func parseSubCall(body string, c eventCall) (eventCall, error) {
	bad := eventlog.Error("invalid_request")
	var target, log, level, filter, id string
	if strings.HasPrefix(strings.TrimSpace(body), "{") {
		fields, err := subFields(body)
		if err != nil {
			return c, bad
		}
		allowed := map[string]bool{"method": true}
		switch c.Method {
		case "sub.add":
			allowed["target"], allowed["log"], allowed["level"], allowed["filter"] = true, true, true, true
		case "sub.list":
			allowed["log"] = true
		case "sub.delete":
			allowed["id"] = true
		}
		for k, raw := range fields {
			if !allowed[k] || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
				return c, bad
			}
			var v string
			if json.Unmarshal(raw, &v) != nil {
				return c, bad
			}
			switch k {
			case "target":
				target = v
			case "log":
				log = v
			case "level":
				level = v
			case "filter":
				filter = v
			case "id":
				id = v
			}
		}
		if _, has := fields["target"]; c.Method == "sub.add" && !has {
			return c, bad
		}
		if _, has := fields["log"]; c.Method == "sub.add" && !has {
			return c, bad
		}
		if _, has := fields["id"]; c.Method == "sub.delete" && !has {
			return c, bad
		}
	} else {
		rest := strings.TrimLeft(body, " ")
		rest = strings.TrimPrefix(rest, "sub")
		rest = strings.TrimLeft(rest, " ")
		rest = strings.TrimPrefix(rest, strings.TrimPrefix(c.Method, "sub."))
		args := strings.Fields(rest)
		switch c.Method {
		case "sub.add":
			if len(args) < 2 {
				return c, bad
			}
			target, log = args[0], args[1]
			// The filter is the rest of the line after the optional level, spaces
			// inside it kept.
			tail := rest
			for range 2 {
				tail = strings.TrimLeft(tail, " ")
				_, after, _ := strings.Cut(tail, " ")
				tail = after
			}
			tail = strings.TrimLeft(tail, " ")
			if word, after, _ := strings.Cut(tail, " "); eventlog.IsLevel(word) {
				level, tail = word, strings.TrimLeft(after, " ")
			}
			filter = tail
		case "sub.list":
			if len(args) > 1 {
				return c, bad
			}
			if len(args) == 1 {
				log = args[0]
			}
		case "sub.delete":
			if len(args) != 1 {
				return c, bad
			}
			id = args[0]
		}
	}
	if c.Method == "sub.add" && (target == "" || log == "") || c.Method == "sub.delete" && !subDeleteID.MatchString(id) {
		return c, eventlog.Error("invalid_request")
	}
	if level != "" && !eventlog.IsLevel(level) {
		return c, eventlog.Error("invalid_level")
	}
	c.Target, c.Log, c.Level, c.Filter, c.SubID = target, log, level, filter, id
	return c, nil
}

// subFields reads a structured call's members: one object, no duplicate member,
// nothing after it.
func subFields(body string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(strings.NewReader(body))
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, io.ErrUnexpectedEOF
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		tok, err = d.Token()
		key, ok := tok.(string)
		if err != nil || !ok {
			return nil, io.ErrUnexpectedEOF
		}
		if _, dup := fields[key]; dup {
			return nil, io.ErrUnexpectedEOF
		}
		var v json.RawMessage
		if d.Decode(&v) != nil {
			return nil, io.ErrUnexpectedEOF
		}
		fields[key] = v
	}
	if tok, err = d.Token(); err != nil || tok != json.Delim('}') {
		return nil, io.ErrUnexpectedEOF
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, io.ErrUnexpectedEOF
	}
	return fields, nil
}
