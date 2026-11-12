// The schedule verbs: each record's crons and ats, kept and fired by the
// daemon (Plans/R1.0-Release/cron-in-core.md#calls). This file only speaks
// HTTP; the daemon decides.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const scheduleVerbs = "add, in, at, list, get, edit, enable, disable, run or delete"

// scheduleBody reads a message: the remaining words, or stdin for "-".
func scheduleBody(words []string) (string, error) {
	if len(words) == 1 && words[0] == "-" {
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	}
	return strings.Join(words, " "), nil
}

// scheduleOptions applies --until, --max-runs, --max-late and --disabled.
func scheduleOptions(f map[string]string, req map[string]any) error {
	if v, ok := f["until"]; ok {
		req["until"] = v
	}
	if v, ok := f["max-runs"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return fmt.Errorf("--max-runs wants a positive count")
		}
		req["max_runs"] = n
	}
	if v, ok := f["max-late"]; ok {
		req["max_late"] = v
	}
	if has(f, "disabled") {
		req["disabled"] = true
	}
	return nil
}

func scheduleVerb(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("schedule wants %s", scheduleVerbs)
	}
	verb, rest := args[0], args[1:]
	pos, f, err := split(rest)
	switch verb {
	case "add", "in", "at":
		if err := only(f, err, "until", "max-runs", "max-late", "disabled"); err != nil {
			return err
		}
		if len(pos) < 4 {
			what := map[string]string{"add": "'<cron expression>'", "in": "<delay>", "at": "<HH:MM|RFC 3339>"}[verb]
			return fmt.Errorf("schedule %s wants <record> %s <target> <message|->", verb, what)
		}
		body, err := scheduleBody(pos[3:])
		if err != nil {
			return err
		}
		form := map[string]string{"add": "expr", "in": "in", "at": "at"}[verb]
		req := map[string]any{"record": pos[0], form: pos[1], "target": pos[2], "body": body}
		if err := scheduleOptions(f, req); err != nil {
			return err
		}
		out, code, err := call("POST", "/schedule/add", nil, req)
		return show(out, code, err)
	case "edit":
		if err := only(f, err, "expr", "in", "at", "target", "body", "until", "max-runs", "max-late", "disabled"); err != nil {
			return err
		}
		if len(pos) != 1 {
			return fmt.Errorf("schedule edit wants <id> and --expr, --in or --at, --target or --body to change")
		}
		return scheduleEdit(pos[0], f)
	case "list":
		if err := only(f, err); err != nil {
			return err
		}
		if len(pos) > 1 {
			return fmt.Errorf("schedule list wants at most <record>")
		}
		q := url.Values{}
		if len(pos) == 1 {
			q.Set("record", pos[0])
		}
		out, code, err := call("GET", "/schedule/list", q, nil)
		return show(out, code, err)
	case "get":
		if err := only(f, err); err != nil {
			return err
		}
		if len(pos) != 1 {
			return fmt.Errorf("schedule get wants <id>")
		}
		out, code, err := call("GET", "/schedule", url.Values{"id": {pos[0]}}, nil)
		return show(out, code, err)
	case "enable", "disable", "run", "delete":
		if err := only(f, err); err != nil {
			return err
		}
		if len(pos) != 1 {
			return fmt.Errorf("schedule %s wants <id>", verb)
		}
		out, code, err := call("POST", "/schedule/"+verb, nil, map[string]string{"id": pos[0]})
		return show(out, code, err)
	}
	return fmt.Errorf("schedule wants %s, not %q", scheduleVerbs, verb)
}

// scheduleEdit keeps what the options do not change.
func scheduleEdit(id string, f map[string]string) error {
	out, code, err := call("GET", "/schedule", url.Values{"id": {id}}, nil)
	if err != nil || code >= 400 {
		return show(out, code, err)
	}
	var cur struct {
		Kind, Expr, At, Target, Body, Until, MaxLate string
		MaxRuns                                      int `json:"max_runs"`
		Enabled                                      bool
	}
	if err := json.Unmarshal(out, &cur); err != nil {
		return err
	}
	req := map[string]any{"id": id, "target": cur.Target, "body": cur.Body}
	forms := 0
	for _, k := range []string{"expr", "in", "at"} {
		if v, ok := f[k]; ok {
			req[k] = v
			forms++
		}
	}
	switch {
	case forms > 1:
		return fmt.Errorf("an edit takes one of --expr, --in or --at")
	case forms == 0 && cur.Kind == "recurring":
		req["expr"] = cur.Expr
	case forms == 0:
		req["at"] = cur.At
	}
	if v, ok := f["target"]; ok {
		req["target"] = v
	}
	if v, ok := f["body"]; ok {
		req["body"] = v
	}
	if cur.Until != "" {
		req["until"] = cur.Until
	}
	if cur.MaxRuns > 0 {
		req["max_runs"] = cur.MaxRuns
	}
	if cur.MaxLate != "" {
		req["max_late"] = cur.MaxLate
	}
	if !cur.Enabled {
		req["disabled"] = true
	}
	if err := scheduleOptions(f, req); err != nil {
		return err
	}
	out, code, err = call("POST", "/schedule/edit", nil, req)
	return show(out, code, err)
}
