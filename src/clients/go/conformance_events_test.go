package bus

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The shared event-log vectors (testdata/client-conformance/events.json)
// through this client's own Call and Send, against the disposable real daemon
// with its own #bus reader. The daemon Owner, the writing agent, a third
// party and a temporary source Maintainer act with their real credentials.

var eventIdentities = map[string]string{"owner": "owner@fixture", "agent": "#caller@fixture", "third": "#outsider@fixture", "worker": "#worker@fixture"}

// eventPath reads a dotted path from decoded JSON: map keys and list indexes.
func eventPath(value any, path string) (any, bool) {
	for _, key := range strings.Split(path, ".") {
		switch x := value.(type) {
		case map[string]any:
			v, ok := x[key]
			if !ok {
				return nil, false
			}
			value = v
		case []any:
			n, err := strconv.Atoi(key)
			if err != nil || n < 0 || n >= len(x) {
				return nil, false
			}
			value = x[n]
		default:
			return nil, false
		}
	}
	return value, true
}

// eventCall is one waiting call to #bus; the reply body is returned as the daemon sent it.
func eventCall(t *testing.T, where string, b *Bus, body string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := b.Call(ctx, "#bus", body, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("%s: call %q: %v", where, body, err)
	}
	if !out.Answered || out.Message == nil {
		t.Fatalf("%s: call %q was not answered: %+v", where, body, out)
	}
	return out.Message.Body
}

func TestSharedEventLogVectors(t *testing.T) { eventVectors(t, "events") }

func TestSharedEventSubscriptionVectors(t *testing.T) { eventVectors(t, "event-subscriptions") }

func eventVectors(t *testing.T, file string) {
	for _, c := range stateVectors(t, file) {
		t.Run(c.ID, func(t *testing.T) {
			addr, tokens := realFixture(t)
			buses := map[string]*Bus{}
			for who, name := range eventIdentities {
				buses[who] = fixtureBus(t, addr, tokens[name], name)
			}
			answers := map[int]string{}
			subscription := ""
			for i, step := range c.Steps {
				where := c.ID + " step " + strconv.Itoa(i) + " " + str(step, "op")
				b := buses[str(step, "as")]
				if b == nil {
					t.Fatalf("%s: unknown identity %q", where, step["as"])
				}
				switch str(step, "op") {
				case "call":
					answer := eventCall(t, where, b, strings.ReplaceAll(str(step, "body"), "$subscription", subscription))
					answers[i] = answer
					var decoded any
					if json.Unmarshal([]byte(answer), &decoded) != nil {
						t.Fatalf("%s: reply is not JSON: %q", where, answer)
					}
					if strings.HasPrefix(str(step, "body"), "sub add ") {
						subscription, _ = decoded.(map[string]any)["id"].(string)
						if subscription == "" {
							t.Fatalf("%s: sub add returned no id: %s", where, answer)
						}
					}
					exp, _ := expectation(step)
					for path, want := range exp {
						if value, ok := want.(string); ok {
							want = strings.ReplaceAll(value, "$subscription", subscription)
						}
						got, ok := eventPath(decoded, path)
						if !ok || !reflect.DeepEqual(got, want) {
							t.Fatalf("%s: %s = %#v (present %v), want %#v; reply %s", where, path, got, ok, want, answer)
						}
					}
					if prior, ok := step["same_answer_as"].(float64); ok && answer != answers[int(prior)] {
						t.Fatalf("%s: reply %q differs from step %d's %q", where, answer, int(prior), answers[int(prior)])
					}
				case "send":
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					_, err := b.Send(ctx, "#bus", str(step, "body"))
					cancel()
					if err != nil {
						t.Fatalf("%s: send: %v", where, err)
					}
				case "manage":
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					changes, _ := step["changes"].(map[string]any)
					_, err := b.Record(str(step, "name")).Manage(ctx, changes)
					cancel()
					if err != nil {
						t.Fatalf("%s: manage: %v", where, err)
					}
				case "inbox":
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					m, err := b.Consume(ctx, WithInbox(str(step, "inbox")), WithWait(300*time.Millisecond))
					cancel()
					if err != nil {
						t.Fatalf("%s: consume: %v", where, err)
					}
					exp, _ := expectation(step)
					if empty, _ := exp["empty"].(bool); empty {
						if m != nil {
							t.Fatalf("%s: the inbox holds a message from %s: %q", where, m.From, m.Body)
						}
						continue
					}
					if m == nil {
						t.Fatalf("%s: expected a subscription copy, inbox was empty", where)
					}
					raw, _ := json.Marshal(m)
					var decoded map[string]any
					if err := json.Unmarshal(raw, &decoded); err != nil {
						t.Fatal(err)
					}
					var body any
					if err := json.Unmarshal([]byte(m.Body), &body); err != nil {
						t.Fatalf("%s: copy body is not JSON: %v", where, err)
					}
					decoded["body"] = body
					decoded["wait"] = m.Wait
					for path, want := range exp {
						got, ok := eventPath(decoded, path)
						if !ok || !reflect.DeepEqual(got, want) {
							t.Fatalf("%s: copy %s = %#v (present %v), want %#v", where, path, got, ok, want)
						}
					}
					// Copies are notices, not waiting calls. Never send ack/done
					// for them; the receipt rule is exercised by the fields above.
				default:
					t.Fatalf("%s: unknown op", where)
				}
			}
		})
	}
}
