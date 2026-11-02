package memory

import (
	"context"
	"github.com/parf/ai-agent-bus/internal/ports"
	"testing"
)

func TestEventAlertBatchValidatesAllRingsBeforePublishing(t *testing.T) {
	s := NewState()
	if err := s.ReplaceEventAlertBatch(context.Background(), []ports.EventAlertSnapshot{{RecordID: 1, Active: true}, {RecordID: 0, Active: true}}); err == nil {
		t.Fatal("invalid second ring accepted")
	}
	_, active, err := s.LoadEventAlerts(1)
	if err != nil || active {
		t.Fatalf("invalid batch partly published: %v %v", active, err)
	}
}
