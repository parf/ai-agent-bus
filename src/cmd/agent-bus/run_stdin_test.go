package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run reads its agent from stdin only when given `-`: a bare run answers with
// its usage and the guide instead of waiting for JSON nobody knows to type
// (K.38), and a pipe without `-` is not read either.
func TestRunReadsStdinOnlyWhenAskedTo(t *testing.T) {
	const agentJSON = `{"name":"#piped@h","algo":"args","script":"/bin/echo"}`
	// A file stands in for a pipe: what run asks is only whether stdin
	// is a device.
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

	t.Run("a bare run on a device answers with its usage", func(t *testing.T) {
		tty, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatal(err)
		}
		defer tty.Close()
		os.Stdin = tty
		_, err = describe(nil)
		if err == nil {
			t.Fatal("a bare run was accepted")
		}
		for _, want := range []string{"agent-bus run [-N] -", "https://github.com/parf/ai-agent-bus/blob/main/docs/user/dedicated-runner.md"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("a bare run said %q, without %q", err, want)
			}
		}
	})

	t.Run("a bare run on a closed stdin answers with its usage", func(t *testing.T) {
		closed := pipe(agentJSON)
		closed.Close()
		os.Stdin = closed
		if _, err := describe(nil); err == nil || !strings.Contains(err.Error(), "docs/user/dedicated-runner.md") {
			t.Fatalf("a bare run on a closed stdin said %v", err)
		}
	})

	t.Run("run - reads the agent", func(t *testing.T) {
		os.Stdin = pipe(agentJSON)
		said := stderr(t)
		ag, err := describe([]string{"-"})
		warned := said()
		if err != nil || ag.Name != "#piped@h" {
			t.Fatalf("run - read %+v, %v", ag, err)
		}
		if warned != "" {
			t.Errorf("run - warned: %q", warned)
		}
	})

	t.Run("run -3 - keeps its count", func(t *testing.T) {
		os.Stdin = pipe(agentJSON)
		ag, err := describe([]string{"-3", "-"})
		if err != nil || ag.Name != "#piped@h" || ag.Instances != 3 {
			t.Fatalf("run -3 - read %+v, %v", ag, err)
		}
	})

	t.Run("a pipe without - is not read", func(t *testing.T) {
		os.Stdin = pipe(agentJSON)
		ag, err := describe([]string{"-3"})
		if err == nil || ag.Name != "" || !strings.Contains(err.Error(), "agent-bus run [-N] -") {
			t.Fatalf("a pipe without - read %+v, %v", ag, err)
		}
	})
}

func TestRunDefaultsToArgsAndKeepsExplicitJSON(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"hello", "/bin/echo"}, "args"},
		{[]string{"hello", "--algo=json", "/bin/echo"}, "json"},
	} {
		ag, err := describe(tc.args)
		if err != nil || ag.Algo != tc.want {
			t.Fatalf("describe(%q): algo=%q error=%v, want %q", tc.args, ag.Algo, err, tc.want)
		}
	}
	saved := os.Stdin
	defer func() { os.Stdin = saved }()
	for _, body := range []struct{ input, want string }{
		{`{"name":"hello","script":"/bin/echo"}`, "args"},
		{`{"name":"hello","script":"/bin/echo","algo":"json"}`, "json"},
	} {
		path := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(path, []byte(body.input), 0600); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		os.Stdin = f
		ag, err := describe([]string{"-"})
		f.Close()
		if err != nil || ag.Algo != body.want {
			t.Fatalf("JSON default: algo=%q error=%v want=%q", ag.Algo, err, body.want)
		}
	}
}
