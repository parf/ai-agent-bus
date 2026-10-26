package main

import (
	"context"
	"fmt"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
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
