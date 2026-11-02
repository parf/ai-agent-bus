package eventlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

type backend struct {
	calls  int
	active map[uint32]bool
	high   map[uint32]uint64
	alerts map[uint32][]ports.EventEntry
	fail   bool
}

func newBackend() *backend {
	return &backend{active: map[uint32]bool{}, high: map[uint32]uint64{}, alerts: map[uint32][]ports.EventEntry{}}
}
func (s *backend) EventHighWater(id uint32) (uint64, error) { return s.high[id], nil }
func (s *backend) ReserveEventIDs(id uint32, n uint64) error {
	if s.fail {
		return errors.New("disk")
	}
	if n < s.high[id] {
		return errors.New("decrease")
	}
	s.high[id] = n
	return nil
}
func (s *backend) LoadEventAlerts(id uint32) ([]ports.EventEntry, bool, error) {
	return append([]ports.EventEntry{}, s.alerts[id]...), s.active[id], nil
}
func (s *backend) ReplaceEventAlerts(id uint32, v []ports.EventEntry, active bool) error {
	if s.fail {
		return errors.New("disk")
	}
	s.calls++
	s.active[id] = active
	s.alerts[id] = append([]ports.EventEntry{}, v...)
	return nil
}
func TestSharedEventRingVectors(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/event-rings.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Cases []struct {
			Name   string
			Now    int64
			Config struct {
				Rings map[string]struct {
					Size int
					TTL  string
				}
				MaxLogs int `json:"max_logs"`
			}
			Steps []struct {
				Op      string
				Args    []any
				Seconds int64
				Expect  map[string]any
			}
		}
	}
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	for _, c := range v.Cases {
		t.Run(c.Name, func(t *testing.T) {
			now := time.Unix(c.Now, 0)
			st := newBackend()
			e := New(st, st, func() time.Time { return now })
			p := Defaults()
			for l, r := range c.Config.Rings {
				old := p[l]
				if r.Size > 0 {
					old.Size = r.Size
				}
				if r.TTL != "" {
					old.TTL, _ = time.ParseDuration(r.TTL)
				}
				p[l] = old
			}
			names := map[string]uint32{}
			active := map[string]bool{}
			for i, step := range c.Steps {
				var got any
				var er error
				arg := func(n int) string {
					if n < len(step.Args) && step.Args[n] != nil {
						return fmt.Sprint(step.Args[n])
					}
					return ""
				}
				name := arg(0)
				if _, ok := names[name]; !ok {
					names[name] = uint32(len(names) + 1)
				}
				id := names[name]
				switch step.Op {
				case "advance":
					now = now.Add(time.Duration(step.Seconds) * time.Second)
					continue
				case "restart":
					e = New(st, st, func() time.Time { return now })
					continue
				case "write":
					if strings.Contains(name, "/") {
						er = Error("invalid_log")
					} else if c.Config.MaxLogs > 0 && !active[name] && len(active) >= c.Config.MaxLogs {
						er = Error("log_limit")
					} else {
						got, er = e.Write(id, arg(1), arg(2), p)
						if er == nil {
							active[name] = true
						}
					}
				case "read":
					got, er = e.Read(id, name, arg(1), p)
				case "pull":
					n := 100
					if len(step.Args) > 2 {
						n = int(step.Args[2].(float64))
					}
					got, er = e.Pull(id, name, arg(1), n, p)
				case "clean":
					var upper uint64
					if len(step.Args) > 2 {
						upper = uint64(step.Args[2].(float64))
					}
					got, er = e.Clean(id, name, arg(1), upper, p)
					if er == nil && arg(1) == "" && upper == 0 {
						delete(active, name)
					}
				case "list":
					logs := []Summary{}
					for name := range active {
						r, err := e.Read(names[name], name, "", p)
						if err == nil {
							logs = append(logs, r.Summary)
						}
					}
					got = map[string]any{"logs": logs}
				default:
					t.Fatalf("unhandled vector op %s", step.Op)
				}
				if er != nil {
					got = map[string]any{"error": Code(er)}
				}
				raw, _ := json.Marshal(got)
				var result any
				json.Unmarshal(raw, &result)
				for path, want := range step.Expect {
					value := result
					for _, key := range strings.Split(path, ".") {
						switch x := value.(type) {
						case map[string]any:
							value = x[key]
						case []any:
							n, _ := strconv.Atoi(key)
							if n >= len(x) {
								value = nil
							} else {
								value = x[n]
							}
						default:
							value = nil
						}
					}
					if !reflect.DeepEqual(value, want) {
						t.Errorf("step %d %s %s: got %#v want %#v (reply %s)", i, step.Op, path, value, want, raw)
					}
				}
			}
		})
	}
}
func TestReservationFailureAndRestartNeverReuseIDs(t *testing.T) {
	st := newBackend()
	now := time.Unix(1793318400, 0)
	e := New(st, nil, func() time.Time { return now })
	p := Defaults()
	v, err := e.Write(1, "notice", "first", p)
	if err != nil || v.ID != 1 {
		t.Fatalf("first %+v %v", v, err)
	}
	if _, err = e.Clean(1, "a", "", 0, p); err != nil {
		t.Fatal(err)
	}
	e = New(st, nil, func() time.Time { return now })
	v, err = e.Write(1, "notice", "second", p)
	if err != nil || v.ID != 65 {
		t.Fatalf("restart reused id %+v %v", v, err)
	}
	st.fail = true
	e = New(st, nil, func() time.Time { return now })
	if _, err = e.Write(1, "notice", "failed", p); Code(err) != "storage_unknown" {
		t.Fatalf("accepted failed reservation: %v", err)
	}
	st.fail = false
	if _, err = e.Write(1, "notice", "after uncertain", p); Code(err) != "storage_unknown" {
		t.Fatalf("uncertain writer reused state: %v", err)
	}
	e = New(st, nil, func() time.Time { return now })
	v, err = e.Write(1, "notice", "third", p)
	if err != nil || v.ID != 129 {
		t.Fatalf("reopen %+v %v", v, err)
	}
	if _, err = e.Read(1, "a", "", p); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Read(2, "replacement", "", p); Code(err) != "not_found" {
		t.Fatalf("new record inherited log: %v", err)
	}
}
func TestRateAndValidationRefusalsDoNotAllocateIDs(t *testing.T) {
	st := newBackend()
	now := time.Unix(1793318400, 0)
	e := New(st, nil, func() time.Time { return now })
	p := Defaults()
	for i := 0; i < 20; i++ {
		if _, err := e.Write(1, "notice", fmt.Sprint(i), p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.Write(1, "notice", "overflow", p); Code(err) != "rate_limit" {
		t.Fatalf("rate refusal %v", err)
	}
	if e.Dropped(1) != 1 {
		t.Fatal("drop not counted")
	}
	now = now.Add(time.Second)
	v, err := e.Write(1, "notice", "after", p)
	if err != nil || v.ID != 21 {
		t.Fatalf("refused write allocated id %+v %v", v, err)
	}
	if _, err = e.Write(1, "invalid", "x", p); Code(err) != "invalid_level" {
		t.Fatalf("invalid level %v", err)
	}
	if _, err = e.Write(1, "notice", "\n\x00", p); Code(err) != "invalid_text" {
		t.Fatalf("empty sanitized text %v", err)
	}
}
func TestDamagedAndUncertainAlertStoreFailClosed(t *testing.T) {
	st := newBackend()
	st.high[1] = 64
	st.active[1] = true
	st.alerts[1] = []ports.EventEntry{{ID: 1, Level: "notice", Text: "wrong ring", At: "2026-10-30T00:00:00.000Z", LastAt: "2026-10-30T00:00:00.000Z", Count: 1}}
	e := New(st, st, nil)
	if _, err := e.Read(1, "a", "", Defaults()); Code(err) != "storage_invalid" {
		t.Fatalf("damage became empty log %v", err)
	}
	st = newBackend()
	e = New(st, st, nil)
	if _, err := e.Write(1, "alert", "ok", Defaults()); err != nil {
		t.Fatal(err)
	}
	st.fail = true
	if _, err := e.Write(1, "alert", "fails", Defaults()); Code(err) != "storage_unknown" {
		t.Fatalf("accepted uncommitted alert %v", err)
	}
	st.fail = false
	if _, err := e.Read(1, "a", "", Defaults()); Code(err) != "storage_unknown" {
		t.Fatalf("poisoned log served %v", err)
	}
}

func TestUnchangedAlertsDoNotWriteAndInactiveSnapshotRefuses(t *testing.T) {
	st := newBackend()
	e := New(st, st, nil)
	p := Defaults()
	if _, err := e.Write(1, "notice", "one", p); err != nil {
		t.Fatal(err)
	}
	before := st.calls
	if _, err := e.Write(1, "warning", "two", p); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Read(1, "a", "", p); err != nil {
		t.Fatal(err)
	}
	if st.calls != before {
		t.Fatalf("unchanged alert snapshot wrote %d extra times", st.calls-before)
	}
	if _, err := e.Write(1, "alert", "alert", p); err != nil {
		t.Fatal(err)
	}
	if st.calls != before+1 {
		t.Fatal("changed alert snapshot was not committed")
	}
	st.active[1] = false
	e = New(st, st, nil)
	if _, err := e.Read(1, "a", "", p); Code(err) != "storage_invalid" {
		t.Fatalf("inactive snapshot with entries accepted: %v", err)
	}
}
