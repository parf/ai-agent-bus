// The key-value verbs: each record's store of string, int and JSON values,
// kept in the daemon's database (Plans/R1.0-Release/kv.md#per-record-storage).
// This file only speaks HTTP; the daemon decides.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"unicode/utf8"
)

func kvVerb(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("kv wants get, set, delete, inc or json")
	}
	verb, rest := args[0], args[1:]
	pos, f := split(rest)
	switch verb {
	case "get", "set", "delete":
		if err := only(f, "int", "json", "add", "replace"); err != nil {
			return err
		}
		if verb != "set" && (has(f, "add") || has(f, "replace")) {
			return fmt.Errorf("--add and --replace are modes of kv set")
		}
	case "inc", "json":
		if err := only(f); err != nil {
			return err
		}
	default:
		return fmt.Errorf("kv wants get, set, delete, inc or json, not %q", verb)
	}
	kind := "string"
	switch {
	case has(f, "int") && has(f, "json"):
		return fmt.Errorf("a value is --int or --json, not both")
	case has(f, "int"):
		kind = "int"
	case has(f, "json"):
		kind = "json"
	}
	switch verb {
	case "get":
		if len(pos) != 2 {
			return fmt.Errorf("kv get wants <record> <name>")
		}
		return kvGet(pos[0], pos[1], kind)
	case "set":
		if len(pos) != 3 {
			return fmt.Errorf("kv set wants <record> <name> <value>, or - for stdin")
		}
		how := "set"
		switch {
		case has(f, "add") && has(f, "replace"):
			return fmt.Errorf("a set is --add or --replace, not both")
		case has(f, "add"):
			how = "add"
		case has(f, "replace"):
			how = "replace"
		}
		return kvSet(pos[0], pos[1], pos[2], kind, how)
	case "delete":
		if len(pos) != 2 {
			return fmt.Errorf("kv delete wants <record> <name>")
		}
		out, code, err := call("POST", "/kv/delete", nil, map[string]string{"record": pos[0], "kind": kind, "name": pos[1]})
		if err != nil || code >= 400 {
			return show(out, code, err)
		}
		var res struct {
			Deleted bool `json:"deleted"`
		}
		if err := json.Unmarshal(out, &res); err != nil {
			return err
		}
		if res.Deleted {
			fmt.Printf("%s %s deleted\n", pos[0], pos[1])
		} else {
			fmt.Printf("%s %s held no %s value\n", pos[0], pos[1], kind)
		}
		return nil
	case "inc":
		if len(pos) != 2 && len(pos) != 3 {
			return fmt.Errorf("kv inc wants <record> <name> [n]")
		}
		n := int64(1)
		if len(pos) == 3 {
			var err error
			if n, err = strconv.ParseInt(pos[2], 10, 64); err != nil {
				return fmt.Errorf("kv inc adds an integer, not %q", pos[2])
			}
		}
		out, code, err := call("POST", "/kv/inc", nil, map[string]any{"record": pos[0], "name": pos[1], "n": n})
		if err != nil || code >= 400 {
			return show(out, code, err)
		}
		return printValue(out, "int")
	default:
		if len(pos) != 3 {
			return fmt.Errorf(`kv json wants <record> <name> '[{"op":"push","key":"jobs","value":1}]'`)
		}
		var ops json.RawMessage
		if err := json.Unmarshal([]byte(pos[2]), &ops); err != nil {
			return fmt.Errorf("kv json takes its operations as a JSON array: %v", err)
		}
		out, code, err := call("POST", "/kv/json", nil, map[string]any{"record": pos[0], "name": pos[1], "ops": ops})
		if err != nil || code >= 400 {
			return show(out, code, err)
		}
		var res struct {
			Results json.RawMessage `json:"results"`
		}
		if err := json.Unmarshal(out, &res); err != nil {
			return err
		}
		fmt.Println(string(res.Results))
		return nil
	}
}

func kvGet(record, name, kind string) error {
	out, code, err := call("GET", "/kv", url.Values{"record": {record}, "name": {name}, "kind": {kind}}, nil)
	if err != nil || code >= 400 {
		return show(out, code, err)
	}
	return printValue(out, kind)
}

// printValue prints a value as it is: a string's bytes, an int, a JSON
// value compact, each with a newline.
func printValue(out []byte, kind string) error {
	var res struct {
		Value       json.RawMessage `json:"value"`
		ValueBase64 string          `json:"value_base64"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return err
	}
	if kind != "string" {
		fmt.Println(string(res.Value))
		return nil
	}
	var b []byte
	if res.ValueBase64 != "" {
		var err error
		if b, err = base64.StdEncoding.DecodeString(res.ValueBase64); err != nil {
			return err
		}
	} else {
		var s string
		if err := json.Unmarshal(res.Value, &s); err != nil {
			return err
		}
		b = []byte(s)
	}
	os.Stdout.Write(b)
	fmt.Println()
	return nil
}

func kvSet(record, name, value, kind, how string) error {
	raw := []byte(value)
	if value == "-" {
		var err error
		if raw, err = io.ReadAll(os.Stdin); err != nil {
			return err
		}
	}
	body := map[string]any{"record": record, "kind": kind, "name": name, "how": how}
	switch kind {
	case "string":
		if utf8.Valid(raw) {
			body["value"] = string(raw)
		} else {
			body["value_base64"] = base64.StdEncoding.EncodeToString(raw)
		}
	case "int":
		n, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil {
			return fmt.Errorf("an --int value is an integer, not %q", raw)
		}
		body["value"] = n
	default:
		if !json.Valid(raw) {
			return fmt.Errorf("a --json value is JSON")
		}
		body["value"] = json.RawMessage(raw)
	}
	out, code, err := call("POST", "/kv/set", nil, body)
	if err != nil || code >= 400 {
		return show(out, code, err)
	}
	fmt.Printf("%s %s set\n", record, name)
	return nil
}
