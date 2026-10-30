package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"net/url"
	"strings"
	"time"
)

// Existing agents are refreshed with their own retained credential. An atomic
// set-add preserves operator tags and never uses registration to take a name.
func refreshManagedTags(as *client, in installed, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tags, err := protocol.NormalizeTags(append(append(append([]string(nil), in.Agent.Tags...), in.Host.Tags...), "runner"))
	if err != nil {
		return err
	}
	out, code, err := as.do(ctx, "POST", "/manage", nil, core.Management{Name: name, AddToSet: &core.ListDelta{Tags: tags}})
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("tags: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// clearStaleScript drops the script an earlier `agent-bus run` registered on a
// record handed over to this runner: the runner serves it now, and a listing
// that still says "runs exec …" names a program that no longer serves it. The
// record re-registers itself with what it has, script empty; nothing else changes.
func clearStaleScript(as *client, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, code, err := as.do(ctx, "GET", "/lookup", url.Values{"name": {name}}, nil)
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("lookup: %s", strings.TrimSpace(string(out)))
	}
	var rec protocol.Record
	if err := json.Unmarshal(out, &rec); err != nil {
		return fmt.Errorf("lookup: %w", err)
	}
	if rec.Script == "" {
		return nil
	}
	// Only what a registration replaces is restated; the lists it leaves out
	// (ACL, deliver-to, roles, tags, maintainers) keep what the record has.
	refresh := protocol.Record{Name: name, Kind: rec.Kind, Descr: rec.Descr, TTL: rec.TTL, Bound: rec.Bound, Full: rec.Full}
	out, code, err = as.do(ctx, "POST", "/register", nil, refresh)
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("clearing the old script: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
