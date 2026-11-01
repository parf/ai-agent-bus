package version

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestVersionQueryKeepsReleaseAndBuildBeforeCopyright(t *testing.T) {
	oldArgs, oldOut := os.Args, os.Stdout
	defer func() { os.Args, os.Stdout = oldArgs, oldOut }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"agent-bus", "--version"}
	os.Stdout = w
	printed := Print()
	w.Close()
	os.Stdout = oldOut
	data, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	want := String + "\nbuild_info: " + Build + "\n© Sergey Porfiriev\n"
	if !printed || string(data) != want {
		t.Fatalf("version output=%q, want %q", data, want)
	}
	if strings.Count(string(data), "© Sergey Porfiriev") != 1 {
		t.Fatal("copyright must appear once")
	}
}
