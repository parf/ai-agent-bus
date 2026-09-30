// The lock verbs: a Group is the lock's namespace and its ACL, every lock has
// a ttl, and the table lives in the daemon's memory
// (docs/01-identity-and-roles.md#shared-locks). This file only speaks HTTP;
// the daemon decides.
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

func lockTake(try bool) func([]string) error {
	return func(args []string) error {
		pos, flags := split(args)
		if err := only(flags, "ttl", "wait"); err != nil {
			return err
		}
		if len(pos) != 2 {
			return fmt.Errorf("wants <group> <name>")
		}
		ttl := flags["ttl"]
		if ttl == "" {
			ttl = "30s"
		}
		if _, err := time.ParseDuration(ttl); err != nil {
			return fmt.Errorf("--ttl is a duration, like 30s")
		}
		q := url.Values{}
		if !try {
			q.Set("wait", flags["wait"])
		}
		body, code, err := call("POST", "/"+map[bool]string{true: "try-lock", false: "lock"}[try], q, map[string]string{"group": pos[0], "name": pos[1], "ttl": ttl})
		if err != nil {
			return err
		}
		return show(body, code, nil)
	}
}

func lockRelease(args []string) error {
	pos, flags := split(args)
	if err := only(flags, "force"); err != nil {
		return err
	}
	if len(pos) != 2 {
		return fmt.Errorf("wants <group> <name>")
	}
	q := url.Values{}
	if has(flags, "force") {
		q.Set("force", "1")
	}
	body, code, err := call("POST", "/release", q, map[string]string{"group": pos[0], "name": pos[1]})
	if err != nil {
		return err
	}
	return show(body, code, nil)
}

func lockHolders(args []string) error {
	pos, flags := split(args)
	if err := only(flags); err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("wants <group>")
	}
	body, code, err := call("GET", "/holders", url.Values{"group": {pos[0]}}, nil)
	if err != nil {
		return err
	}
	if code >= 400 {
		return show(body, code, nil)
	}
	var out struct {
		Group string            `json:"group"`
		Locks map[string]string `json:"locks"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return err
	}
	if len(out.Locks) == 0 {
		fmt.Printf("%s holds no locks\n", out.Group)
		return nil
	}
	for name, holder := range out.Locks {
		fmt.Printf("%s %s %s\n", out.Group, name, holder)
	}
	return nil
}
