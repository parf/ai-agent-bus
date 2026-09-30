// The lock verbs: a Group is the lock's namespace and its ACL, every lock has
// a ttl, and the table lives in the daemon's memory
// (docs/01-identity-and-roles.md#shared-locks). This file only speaks HTTP;
// the daemon decides.
package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
)

func lockTake(try bool) func([]string) error {
	return func(args []string) error {
		pos, f := split(args)
		if err := only(f, "ttl", "wait"); err != nil {
			return err
		}
		if len(pos) != 2 {
			return fmt.Errorf("%s wants <group> <name>", map[bool]string{true: "try-lock", false: "lock"}[try])
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
		out, code, err := call("POST", path, url.Values{"wait": {wait}}, map[string]string{"group": pos[0], "name": pos[1], "ttl": ttl})
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
		return fmt.Errorf("release wants <group> <name>")
	}
	path := "/release"
	if has(f, "force") {
		path = "/release-force"
	}
	out, code, err := call("POST", path, nil, map[string]string{"group": pos[0], "name": pos[1]})
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
		return fmt.Errorf("extend wants <group> <name>")
	}
	ttl := f["ttl"]
	if ttl == "" {
		ttl = "30s"
	}
	out, code, err := call("POST", "/extend", nil, map[string]string{"group": pos[0], "name": pos[1], "ttl": ttl})
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
		return fmt.Errorf("holders wants <group>")
	}
	out, code, err := call("GET", "/holders", url.Values{"group": {pos[0]}}, nil)
	if err != nil {
		return err
	}
	if code >= 400 {
		return show(out, code, nil)
	}
	var res struct {
		Group string            `json:"group"`
		Locks map[string]string `json:"locks"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return err
	}
	if len(res.Locks) == 0 {
		fmt.Printf("%s holds no locks\n", res.Group)
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(res.Locks)) {
		fmt.Printf("%s %s %s\n", res.Group, name, res.Locks[name])
	}
	return nil
}
