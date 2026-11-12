package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"
)

// The shared schedule vectors (testdata/client-conformance/schedule.json)
// through Record.Schedules, against the disposable real daemon: the owner
// keeps schedules on jobs@fixture, and #worker@fixture reads the firing.

func scheduleSpec(step map[string]any) ScheduleSpec {
	return ScheduleSpec{Expr: str(step, "expr"), In: str(step, "in"), At: str(step, "at"), Target: str(step, "target"), Body: str(step, "body")}
}

// scheduleMatches compares a schedule with a step's expect: scalar fields by
// their wire name, and outcomes by their outcome words, newest first.
func scheduleMatches(t *testing.T, where string, got Schedule, exp map[string]any) {
	t.Helper()
	var wire map[string]any
	_ = json.Unmarshal(mustJSON(got), &wire)
	for k, want := range exp {
		if k == "outcomes" {
			words := []any{}
			for _, o := range got.Outcomes {
				words = append(words, o.Outcome)
			}
			if fmt.Sprint(words) != fmt.Sprint(want) {
				t.Fatalf("%s: outcomes %v, want %v", where, words, want)
			}
			continue
		}
		if fmt.Sprint(wire[k]) != fmt.Sprint(want) {
			t.Fatalf("%s: %s is %v, want %v", where, k, wire[k], want)
		}
	}
}

func TestSharedScheduleVectors(t *testing.T) {
	for _, c := range stateVectors(t, "schedule") {
		t.Run(c.ID, func(t *testing.T) {
			addr, tokens := realFixture(t)
			owner := fixtureBus(t, addr, tokens["owner@fixture"], "owner@fixture")
			worker := fixtureBus(t, addr, tokens["#worker@fixture"], "#worker@fixture")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			sched := owner.Record("jobs@fixture").Schedules
			id := ""
			for i, step := range c.Steps {
				where := c.ID + " step " + strconv.Itoa(i) + " " + str(step, "op")
				exp, _ := expectation(step)
				var got Schedule
				var err error
				switch str(step, "op") {
				case "add":
					got, _, err = sched.Add(ctx, scheduleSpec(step))
					if err == nil {
						id = got.ID
					}
				case "edit":
					got, err = sched.Edit(ctx, id, scheduleSpec(step))
				case "get":
					got, err = sched.Get(ctx, id)
				case "enable":
					got, err = sched.Enable(ctx, id)
				case "disable":
					got, err = sched.Disable(ctx, id)
				case "list":
					var list []Schedule
					if step["all"] == true {
						list, err = owner.Schedules(ctx)
					} else {
						list, err = sched.List(ctx)
					}
					if err == nil {
						ids := []any{}
						for _, s := range list {
							ids = append(ids, s.ID)
						}
						want := []any{}
						for _, w := range exp["ids"].([]any) {
							if w == "$schedule" {
								w = id
							}
							want = append(want, w)
						}
						if fmt.Sprint(ids) != fmt.Sprint(want) {
							t.Fatalf("%s: ids %v, want %v", where, ids, want)
						}
					}
					refusal(t, where, step, err)
					continue
				case "run":
					var o ScheduleOutcome
					if o, err = sched.Run(ctx, id); err == nil && o.Outcome != exp["outcome"] {
						t.Fatalf("%s: outcome %+v", where, o)
					}
					refusal(t, where, step, err)
					continue
				case "inbox":
					m, e := worker.Consume(ctx, WithWait(5*time.Second))
					if e != nil || m == nil {
						t.Fatalf("%s: no firing: %v", where, e)
					}
					seen := map[string]string{"from": m.From, "topic": m.Topic, "tag": m.Tag, "body": m.Body}
					for k, want := range exp {
						if seen[k] != want {
							t.Fatalf("%s: %s is %q, want %v", where, k, seen[k], want)
						}
					}
					continue
				case "delete":
					var deleted bool
					if deleted, err = sched.Delete(ctx, id); err == nil && deleted != exp["deleted"] {
						t.Fatalf("%s: deleted %v", where, deleted)
					}
					refusal(t, where, step, err)
					continue
				default:
					t.Fatalf("%s: unknown vector op", where)
				}
				if !refusal(t, where, step, err) {
					scheduleMatches(t, where, got, exp)
				}
			}
		})
	}
}
