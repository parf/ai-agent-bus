package agentmeta

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHiddenMethodSummaryAndBodyFormats(t *testing.T) {
	for _, formats := range []string{"", `,"body_formats":["text","json"]`} {
		m, err := Decode([]byte(`{"description":"fixture"` + formats + `,"methods":{"read":{"help":"Read"},"hidden-one":{"help":"private-description","role":["operate","admin"],"example":"private-example"},"hidden-two":{"help":"private-description","role":"admin"}}}`))
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			roles  []string
			count  int
			needed string
		}{
			{nil, 2, `["admin","operate"]`}, {[]string{"operate"}, 1, `["admin"]`},
			{[]string{"admin"}, 0, `[]`}, {[]string{"owner"}, 0, `[]`}, {[]string{"maintainer"}, 0, `[]`},
		} {
			out, err := m.Answer(t.TempDir(), "help-json", tc.roles, false)
			if err != nil {
				t.Fatal(err)
			}
			var data struct {
				BodyFormats []string `json:"body_formats"`
				Hidden      struct {
					Count int             `json:"count"`
					Roles json.RawMessage `json:"roles"`
				} `json:"hidden_methods"`
				Methods map[string]json.RawMessage `json:"methods"`
			}
			if err := json.Unmarshal([]byte(out), &data); err != nil {
				t.Fatal(err)
			}
			if data.Hidden.Count != tc.count || string(data.Hidden.Roles) != tc.needed || len(data.Methods) != 3-tc.count {
				t.Fatalf("roles=%v: %s", tc.roles, out)
			}
			want := `["text"]`
			if formats != "" {
				want = `["text","json"]`
			}
			encoded, _ := json.Marshal(data.BodyFormats)
			if string(encoded) != want {
				t.Fatalf("body_formats=%s want %s", encoded, want)
			}
			if tc.count == 2 && (strings.Contains(out, "private-description") || strings.Contains(out, "private-example") || strings.Contains(out, "hidden-one") || strings.Contains(out, "hidden-two")) {
				t.Fatalf("hidden declaration leaked: %s", out)
			}
		}
	}
	for _, formats := range []string{`[]`, `["yaml"]`, `["text","text"]`, `true`} {
		_, err := Decode([]byte(`{"description":"fixture","body_formats":` + formats + `}`))
		if err == nil || !strings.Contains(err.Error(), "body_formats") {
			t.Fatalf("invalid body_formats accepted: %s: %v", formats, err)
		}
	}
}
