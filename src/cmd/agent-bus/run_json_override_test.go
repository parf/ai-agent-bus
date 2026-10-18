package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONRunAppliesAlgoAndDescriptionFlags(t *testing.T) {
	for _, tc := range []struct {
		name        string
		flags       []string
		algo, descr string
		refuse      bool
	}{
		{name: "absent flags preserve JSON", algo: "json", descr: "from JSON"},
		{name: "algorithm overrides JSON", flags: []string{"--algo=args"}, algo: "args", descr: "from JSON"},
		{name: "description overrides JSON", flags: []string{"--descr", "from flag"}, algo: "json", descr: "from flag"},
		{name: "empty description clears JSON", flags: []string{"--descr="}, algo: "json"},
		{name: "invalid override is refused", flags: []string{"--algo=invalid"}, refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "agent.json")
			if err := os.WriteFile(path, []byte(`{"name":"#json@h","script":"/bin/echo","algo":"json","descr":"from JSON"}`), 0600); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			saved := os.Stdin
			os.Stdin = f
			defer func() { os.Stdin = saved }()
			ag, err := describe(append([]string{"-"}, tc.flags...))
			if tc.refuse {
				if err == nil || !strings.Contains(err.Error(), "--algo is json or args") {
					t.Fatalf("invalid override refusal: %v", err)
				}
				return
			}
			if err != nil || ag.Algo != tc.algo || ag.Descr != tc.descr {
				t.Fatalf("JSON+flags algo=%q descr=%q err=%v; want %q %q", ag.Algo, ag.Descr, err, tc.algo, tc.descr)
			}
		})
	}
}
