package envfile

import (
	"strings"
	"testing"
)

func TestDescribeCommentsNeverIncludesValues(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n", "\u2028"} {
		src := strings.Join([]string{"# Comment", "A= # required", "# Not adjacent", "", "B=sentinel", "# Multiline", `C="private`, `more"`, "# Last", "D="}, newline)
		declarations, err := Describe([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if len(declarations) != 4 || declarations[0].Help != "Comment" || !declarations[0].Required || declarations[1].Help != "" || declarations[1].Required || declarations[2].Help != "Multiline" || declarations[3].Help != "Last" {
			t.Fatalf("declarations: %+v", declarations)
		}
	}
}

func TestDescribeMatchesParseAtUnicodeSeparators(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("Describe panicked instead of matching Parse: %v", err)
		}
	}()
	for _, sep := range []string{"\u2028", "\u2029"} {
		for _, src := range []string{"A=a" + sep + "B=x\n", "# Required" + sep + "A=" + sep + "# Next" + sep + "B='value'\n", "A='private" + sep + "# not documentation'\nB=\n"} {
			vars, err := Parse([]byte(src))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Describe([]byte(src))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(vars) {
				t.Fatal("Describe disagrees with Parse variable count")
			}
			for i, v := range vars {
				if got[i].Name != v.Key || got[i].Required != v.Required || strings.Contains(got[i].Help, "not documentation") {
					t.Fatal("Describe changed Parse semantics or exposed a value")
				}
			}
		}
	}
}
