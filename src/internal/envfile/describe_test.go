package envfile

import (
	"strings"
	"testing"
)

func TestDescribeCommentsNeverIncludesValues(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n", "\u2028"} {
		src := strings.Join([]string{"# Comment", "A=", "# Not adjacent", "", "B=sentinel", "# Multiline", `C="private`, `more"`, "# Last", "D="}, newline)
		declarations, err := Describe([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if len(declarations) != 4 || declarations[0].Help != "Comment" || !declarations[0].Required || declarations[1].Help != "" || declarations[1].Required || declarations[2].Help != "Multiline" || declarations[3].Help != "Last" {
			t.Fatalf("declarations: %+v", declarations)
		}
	}
}
