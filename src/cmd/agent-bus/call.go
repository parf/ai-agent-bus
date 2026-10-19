package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func callWait(value string) (time.Duration, error) {
	if value == "" {
		return 30 * time.Second, nil
	}
	wait, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("bad --wait: %v", err)
	}
	if wait <= 0 {
		return 0, fmt.Errorf("--wait must be positive, not %s", wait)
	}
	return wait, nil
}

// Only a missing caller record is registered; refreshing it would overwrite
// metadata its owner chose. Do this once, before starting concurrent calls.
func callAddress() error {
	me := whoami()
	if known, err := registered(me); err != nil {
		return err
	} else if !known {
		if err := postQuiet("/register", protocol.Record{Name: me, Kind: protocol.KindAgent}); err != nil {
			return fmt.Errorf("could not register as %s: %w", me, err)
		}
	}
	return nil
}

type callRefusal struct{ reason string }

func (e callRefusal) Error() string { return e.reason }

// The single-call exchange is shared by both CLI forms. The caller owns the
// deadline, including the send; no retry can repeat an accepted operation.
func callTarget(ctx context.Context, e protocol.Envelope, wait time.Duration, receipts io.Writer) ([]byte, error) {
	until, _ := ctx.Deadline()
	tooLate := fmt.Errorf("no answer within %s (the message may have been accepted; do not resend it)", wait)
	if ctx.Err() != nil {
		return nil, tooLate
	}
	e.Wait = time.Until(until).String()
	sent, code, err := callCtx(ctx, "POST", "/send", nil, e)
	if err != nil {
		if ctx.Err() != nil {
			return nil, tooLate
		}
		return nil, err
	}
	if code >= 400 {
		return nil, callRefusal{strings.TrimSpace(string(sent))}
	}
	q := url.Values{"topic": {e.Topic}, "tag": {e.Tag}}
	for {
		left := time.Until(until)
		if left <= 0 {
			return nil, tooLate
		}
		q.Set("wait", left.String())
		reply, body, got, err := next(ctx, q)
		if err != nil {
			if ctx.Err() != nil {
				return nil, tooLate
			}
			return nil, err
		}
		if !got {
			continue
		}
		if reply.Receipt != "" {
			fmt.Fprintf(receipts, "%s from %s\n", reply.Receipt, reply.From)
			if reply.Receipt == protocol.ReceiptDone {
				return nil, fmt.Errorf("the agent finished and sent no answer (not a timeout; do not resend it)")
			}
			continue
		}
		return body, nil
	}
}

func callTagged(pos []string, flags map[string]string, wait time.Duration) error {
	if err := only(flags, nil, "tagged", "wait"); err != nil {
		return fmt.Errorf("--tagged cannot be combined with other call options: %w", err)
	}
	if strings.TrimSpace(flags["tagged"]) == "" {
		return fmt.Errorf("--tagged wants a nonempty tag")
	}
	if len(pos) != 1 {
		return fmt.Errorf("--tagged wants exactly one body and no explicit target; quote a multi-word body")
	}
	if err := callAddress(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	raw, code, err := callCtx(ctx, "GET", "/ls", url.Values{"kind": {protocol.KindAgent}, "tag": {flags["tagged"]}}, nil)
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("%s", strings.TrimSpace(string(raw)))
	}
	var targets []protocol.Record
	if err := json.Unmarshal(raw, &targets); err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no visible agents tagged %q", flags["tagged"])
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Name < targets[j].Name })
	type result struct {
		body string
		err  error
	}
	results := make([]result, len(targets))
	var calls sync.WaitGroup
	for i := range targets {
		calls.Add(1)
		go func(i int) {
			defer calls.Done()
			raw, err := callTarget(ctx, protocol.Envelope{To: targets[i].Name, Topic: "call-" + newTag(), Tag: newTag(), Body: pos[0]}, wait, io.Discard)
			results[i].err = err
			if err == nil {
				var answer protocol.Envelope
				results[i].err = json.Unmarshal(raw, &answer)
				results[i].body = answer.Body
			}
		}(i)
	}
	calls.Wait()
	for i, r := range results {
		if r.err == nil {
			fmt.Fprintf(os.Stdout, "## %s\n%s\n\n", targets[i].Name, r.body)
		}
	}
	failed := 0
	for _, group := range []struct {
		name    string
		refused bool
	}{{"No answer", false}, {"Refused", true}} {
		heading := false
		for i, r := range results {
			if r.err == nil {
				continue
			}
			var refusal callRefusal
			if errors.As(r.err, &refusal) != group.refused {
				continue
			}
			if !heading {
				fmt.Fprintln(os.Stdout, group.name+":")
				heading = true
			}
			fmt.Fprintf(os.Stdout, "- %s: %v\n", targets[i].Name, r.err)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d agent(s) did not answer or were refused", failed)
	}
	return nil
}
