package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// start reads its agent from stdin only when given `-`: a bare start on a
// terminal answers with its usage and the guide instead of waiting for JSON
// nobody knows to type (K.38). A pipe without `-` is still read, and said.
func TestStartReadsStdinOnlyWhenAskedTo(t *testing.T) {
	const agentJSON = `{"name":"#piped@h","algo":"args","script":"/bin/echo"}`
	pipe := func(body string) *os.File {
		t.Helper()
		path := filepath.Join(t.TempDir(), "agent.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}
	// What the warning went to: stderr, swapped for the test.
	stderr := func(t *testing.T) func() string {
		t.Helper()
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		old := os.Stderr
		os.Stderr = w
		return func() string {
			w.Close()
			os.Stderr = old
			b, _ := io.ReadAll(r)
			return string(b)
		}
	}
	defer func(f *os.File) { os.Stdin = f }(os.Stdin)

	t.Run("a bare start on a device answers with its usage", func(t *testing.T) {
		tty, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatal(err)
		}
		defer tty.Close()
		os.Stdin = tty
		_, err = describe(nil)
		if err == nil {
			t.Fatal("a bare start was accepted")
		}
		for _, want := range []string{"agent-bus start [-N] -", "https://github.com/parf/ai-agent-bus/blob/main/docs/user/runner.md"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("a bare start said %q, without %q", err, want)
			}
		}
	})

	t.Run("start - reads the agent", func(t *testing.T) {
		os.Stdin = pipe(agentJSON)
		said := stderr(t)
		ag, err := describe([]string{"-"})
		warned := said()
		if err != nil || ag.Name != "#piped@h" {
			t.Fatalf("start - read %+v, %v", ag, err)
		}
		if warned != "" {
			t.Errorf("start - warned: %q", warned)
		}
	})

	t.Run("start -3 - keeps its count", func(t *testing.T) {
		os.Stdin = pipe(agentJSON)
		ag, err := describe([]string{"-3", "-"})
		if err != nil || ag.Name != "#piped@h" || ag.Instances != 3 {
			t.Fatalf("start -3 - read %+v, %v", ag, err)
		}
	})

	t.Run("a pipe without - is still read, and said", func(t *testing.T) {
		os.Stdin = pipe(agentJSON)
		said := stderr(t)
		ag, err := describe([]string{"-3"})
		warned := said()
		if err != nil || ag.Name != "#piped@h" || ag.Instances != 3 {
			t.Fatalf("a pipe without - read %+v, %v", ag, err)
		}
		if !strings.Contains(warned, "agent-bus start -") {
			t.Errorf("a pipe without - was read silently: %q", warned)
		}
	})
}
