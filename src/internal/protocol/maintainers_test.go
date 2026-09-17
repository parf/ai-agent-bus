package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMaintainersReadLegacyStringAndWriteArray(t *testing.T) {
	var record Record
	if err := json.Unmarshal([]byte(`{"name":"svc@h","owner":"alice@h","maintainers":"@ops"}`), &record); err != nil {
		t.Fatal(err)
	}
	if len(record.Maintainers) != 1 || record.Maintainers[0] != "@ops" {
		t.Fatalf("legacy maintainer was not migrated in memory: %#v", record.Maintainers)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"maintainers":["@ops"]`) || strings.Contains(string(encoded), `"maintainers":"@ops"`) {
		t.Fatalf("daemon-originated record did not use array spelling: %s", encoded)
	}

	if err := json.Unmarshal([]byte(`{"name":"svc@h","owner":"alice@h","maintainers":[]}`), &record); err != nil {
		t.Fatal(err)
	}
	if len(record.Maintainers) != 0 {
		t.Fatalf("empty array did not clear maintainers: %#v", record.Maintainers)
	}
}

func TestMaintainersRejectNonStringAndNonArray(t *testing.T) {
	var record Record
	if err := json.Unmarshal([]byte(`{"name":"svc@h","owner":"alice@h","maintainers":7}`), &record); err == nil {
		t.Fatal("numeric maintainers value was accepted")
	}
}
