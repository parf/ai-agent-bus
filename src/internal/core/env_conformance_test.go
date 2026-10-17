package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestSecretValidationUsesENVConformance(t *testing.T) {
	b, err := os.ReadFile("../envfile/testdata/common.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Source string
		Line         int
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			err := validEnv(c.Source)
			if c.Line == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("line %d", c.Line)) {
				t.Fatalf("wanted safe refusal on line %d, got %v", c.Line, err)
			}
			if strings.Contains(err.Error(), "PRIVATE_SENTINEL") {
				t.Fatal("save validation echoed a refused input")
			}
		})
	}
}

// Exercise the write boundary, not only the validator: refusals preserve the stored bytes.
func TestSecretWritesAndENVConformanceAgree(t *testing.T) {
	b, err := os.ReadFile("../envfile/testdata/common.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Source string
		Line         int
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	bus := New()
	bus.SetDaemonOwner("owner@h")
	provision(t, bus, []string{"owner@h"}, protocol.Record{Name: "#env@h", Kind: protocol.KindAgent, Owner: "owner@h", Full: protocol.OverflowStrict})
	for _, c := range cases {
		if c.Source == "" {
			continue
		}
		t.Run(c.Name, func(t *testing.T) {
			if _, err := bus.SetSecret("#env@h", "owner@h", "A=kept"); err != nil {
				t.Fatal(err)
			}
			_, err := bus.SetSecret("#env@h", "owner@h", c.Source)
			want := c.Source
			if c.Line > 0 {
				want = "A=kept"
				if !errors.Is(err, ErrEnv) || !strings.Contains(err.Error(), fmt.Sprintf("line %d", c.Line)) {
					t.Fatalf("wanted safe refusal on line %d, got %v", c.Line, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			got, err := bus.Secret("#env@h", "owner@h")
			if err != nil || got != want {
				t.Fatal("the stored bytes disagree with the accepted write")
			}
		})
	}
}
