package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestRunnerDeliveryUsesENVConformance(t *testing.T) {
	b, err := os.ReadFile("../../internal/envfile/testdata/common.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Source string
		Want         []envVar
		Line         int
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			got, err := parseEnv("bus secret", []byte(c.Source))
			if c.Line == 0 {
				if err != nil || !reflect.DeepEqual(got, c.Want) {
					t.Fatalf("got %#v, %v; want %#v", got, err, c.Want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("line %d", c.Line)) {
				t.Fatalf("wanted safe refusal on line %d, got %v", c.Line, err)
			}
			if strings.Contains(err.Error(), "PRIVATE_SENTINEL") {
				t.Fatal("delivery echoed a refused input")
			}
		})
	}
}
