package agentmeta

import "testing"

func TestAskDispatchNeverFallsIntoNormalWork(t *testing.T) {
	called := 0
	handlers := map[string]AskHandler{"status": func() (string, error) { called++; return "starting: fixture", nil }}
	for _, ask := range []string{"", "status", "unknown", "mcp"} {
		body, handled, err := DispatchAsk(ask, handlers)
		if err != nil || handled != (ask != "") {
			t.Fatalf("%s: %s %v %v", ask, body, handled, err)
		}
		if ask == "status" && body != "starting: fixture" {
			t.Fatal(body)
		}
		if ask != "" && ask != "status" && body != "unsupported: "+ask {
			t.Fatal(body)
		}
	}
	if called != 1 {
		t.Fatal(called)
	}
	for _, source := range []string{`{"description":"fixture","asks":"none"}`, `{"description":"fixture"}`} {
		if _, err := Decode([]byte(source)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Decode([]byte(`{"description":"fixture","asks":"never"}`)); err == nil {
		t.Fatal("invalid asks policy accepted")
	}
}
