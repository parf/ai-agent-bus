package main

import (
	"os"
	"strings"
	"testing"
)

// The runner's registered description lists every call the runner answers, so
// `agent-bus ls` and help discovery name the deploy calls too. The runner's own
// refusal is the other statement of that list: each call in ours must be in it.
func TestRunnerDescriptionNamesEveryCallTheRunnerAnswers(t *testing.T) {
	source, err := os.ReadFile("../agent-bus-runner/main.go")
	if err != nil {
		t.Fatal(err)
	}
	var refusal string
	for _, line := range strings.Split(string(source), "\n") {
		if strings.Contains(line, " answers status, reload,") {
			refusal = line
		}
	}
	if refusal == "" {
		t.Fatal("the runner's refusal that lists its calls moved; update this check")
	}
	calls := strings.Split(runnerCalls, ", ")
	for _, call := range calls {
		if !strings.Contains(refusal, call) {
			t.Errorf("the description names %q, which the runner's own refusal does not", call)
		}
	}
	for _, want := range []string{"install", "remove"} {
		if !strings.Contains(runnerCalls, want) {
			t.Errorf("the description lacks %q: %s", want, runnerCalls)
		}
	}
	// The refusal's verbs, read back out of it, are all in ours.
	list := refusal[strings.Index(refusal, " answers ")+len(" answers "):]
	list = list[:strings.Index(list, ", not")]
	list = strings.NewReplacer(", install and remove", ", install, remove").Replace(list)
	if list != runnerCalls {
		t.Errorf("the runner answers %q but the description says %q", list, runnerCalls)
	}
}
