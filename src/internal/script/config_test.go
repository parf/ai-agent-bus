package script

import (
	"reflect"
	"testing"
)

func TestConfigEnvironmentDistinguishesAbsentAndEmptyObject(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want []string
	}{
		{"null\n", []string{"PATH=/bin"}}, {" { } ", []string{"PATH=/bin", "AGENT_BUS_CONFIG={}"}},
		{" {\n \"n\":9007199254740993, \"text\":\"a b\" }", []string{"PATH=/bin", `AGENT_BUS_CONFIG={"n":9007199254740993,"text":"a b"}`}},
	} {
		got, err := ConfigEnv([]string{"AGENT_BUS_CONFIG=wrong", "PATH=/bin"}, []byte(tc.raw))
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("config %s: %v, %v; want %v", tc.raw, got, err, tc.want)
		}
	}
}
