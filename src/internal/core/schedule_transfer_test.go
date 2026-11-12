package core

import (
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A record's schedules travel with it through export and import, as system
// state (Q411): same id, holder, target, creator and message; nothing in flight.
func TestRecordImportCarriesSchedules(t *testing.T) {
	b, _, _ := transferBus(t, "owner@source")
	if err := b.EstablishDaemonAgent("source"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "#worker@source", Kind: protocol.KindAgent, Owner: "owner@source", Tags: []string{"ops"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "out@source", Kind: protocol.KindQueue, Owner: "owner@source", Tags: []string{"ops"}}); err != nil {
		t.Fatal(err)
	}
	v, _, err := b.ScheduleAdd("owner@source", "#worker@source", ScheduleRequest{Expr: "0 9 * * *", Target: "out@source", Body: "standup"})
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := b.ExportRecords("owner@source", RecordSelection{Include: []string{"tag:ops"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Schedules) != 1 || data.Schedules[0].Record != "#worker@source" {
		t.Fatalf("exported schedules: %+v", data.Schedules)
	}
	target, _, st := transferBus(t, "owner@source")
	if _, err := target.ImportRecords("owner@source", data, ImportChoices{KeepExisting: []string{"all"}}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.ScheduleGet(v.ID)
	if err != nil || !ok || got.Target != "out@source" || got.Creator != "owner@source" || got.Body != "standup" || got.InFlight != 0 || got.Next == 0 {
		t.Fatalf("imported schedule: %+v %v %v", got, ok, err)
	}
	list, err := target.ScheduleList("owner@source", "#worker@source")
	if err != nil || len(list) != 1 || list[0].ID != v.ID {
		t.Fatalf("the imported holder's schedules: %+v %v", list, err)
	}
}
