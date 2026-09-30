// The lock verbs: a record is the lock's namespace, its Owner, Maintainers and
// own Agent use its locks, every lock has a ttl, and the table lives in the
// daemon's memory
// (docs/01-identity-and-roles.md#shared-locks). This file only speaks HTTP;
// the daemon decides.
package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"time"
)

func lockTake(try bool) func([]string) error {
	return func(args []string) error {
		pos, f := split(args)
		if err := only(f, "ttl", "wait"); err != nil {
			return err
		}
		if len(pos) != 2 {
			return fmt.Errorf("%s wants <record> <name>", map[bool]string{true: "try-lock", false: "lock"}[try])
		}
		ttl := f["ttl"]
		if ttl == "" {
			ttl = "30s"
		}
		wait := f["wait"]
		if try {
			if wait != "" {
				return fmt.Errorf("try-lock answers now; use lock --wait to wait")
			}
			wait = "0s"
		} else if wait == "" {
			wait = "30s"
		}
		path := "/lock"
		if try {
			path = "/try-lock"
		}
		out, code, err := call("POST", path, url.Values{"wait": {wait}}, map[string]string{"record": pos[0], "name": pos[1], "ttl": ttl})
		if err != nil {
			return err
		}
		return show(out, code, nil)
	}
}

func lockRelease(args []string) error {
	pos, f := split(args)
	if err := only(f, "force"); err != nil {
		return err
	}
	if len(pos) != 2 {
		return fmt.Errorf("release wants <record> <name>")
	}
	path := "/release"
	if has(f, "force") {
		path = "/release-force"
	}
	out, code, err := call("POST", path, nil, map[string]string{"record": pos[0], "name": pos[1]})
	if err != nil {
		return err
	}
	return show(out, code, nil)
}

func lockExtend(args []string) error {
	pos, f := split(args)
	if err := only(f, "ttl"); err != nil {
		return err
	}
	if len(pos) != 2 {
		return fmt.Errorf("extend wants <record> <name>")
	}
	ttl := f["ttl"]
	if ttl == "" {
		ttl = "30s"
	}
	out, code, err := call("POST", "/extend", nil, map[string]string{"record": pos[0], "name": pos[1], "ttl": ttl})
	if err != nil {
		return err
	}
	return show(out, code, nil)
}

func lockHolders(args []string) error {
	pos, f := split(args)
	if err := only(f); err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("holders wants <record>")
	}
	out, code, err := call("GET", "/holders", url.Values{"record": {pos[0]}}, nil)
	if err != nil {
		return err
	}
	if code >= 400 {
		return show(out, code, nil)
	}
	var res struct {
		Record string `json:"record"`
		Locks  map[string]struct {
			Holder  string    `json:"holder"`
			Expires time.Time `json:"expires"`
		} `json:"locks"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return err
	}
	if len(res.Locks) == 0 {
		fmt.Printf("%s holds no locks\n", res.Record)
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(res.Locks)) {
		l := res.Locks[name]
		fmt.Printf("%s %s %s %s left\n", res.Record, name, l.Holder, leftText(time.Until(l.Expires)))
	}
	return nil
}

// leftText is time left the way the web face shows it: "1h 5m", "4m", "30s",
// never negative (src/web/format.ts left).
func leftText(d time.Duration) string {
	s := max(0, int((d+time.Second/2)/time.Second))
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := (s + 30) / 60
	switch {
	case m < 60:
		return fmt.Sprintf("%dm", m)
	case m%60 == 0:
		return fmt.Sprintf("%dh", m/60)
	default:
		return fmt.Sprintf("%dh %dm", m/60, m%60)
	}
}
