package envfile

import (
	"strings"
	"testing"
)

func TestRequiredMarkerAndDefaults(t *testing.T) {
	src := "# Empty required\nA= # required\nB=private-default # required explanation\nC=\nD=public-default\nE='literal # required'\nF= # requiredness\nG=\n# required\nH=\nI=\"\" # required\nJ='multi\nline' # required\n"
	vars, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	declarations, err := Describe([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	required := []bool{true, true, false, false, false, false, false, false, true, true}
	defaults := []bool{false, true, false, true, true, false, false, false, false, true}
	if len(vars) != len(required) || len(declarations) != len(required) {
		t.Fatal("assignment count changed")
	}
	for i, v := range vars {
		if v.Required != required[i] || declarations[i].Required != required[i] || declarations[i].HasDefault != defaults[i] {
			t.Fatalf("wrong marker/default flags for %s", v.Key)
		}
		if strings.Contains(declarations[i].Help, "private-default") {
			t.Fatal("default value exposed in help")
		}
	}
}
