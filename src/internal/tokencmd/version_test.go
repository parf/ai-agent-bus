package tokencmd

import (
	"github.com/parf/ai-agent-bus/internal/version"
	"io"
	"os"
	"testing"
)

func TestTokenVersionCreditsCopyrightWithoutIssuingCredential(t *testing.T) {
	old := os.Stdout
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer func() { os.Stdout = old }()
	os.Stdout = w
	err := Run([]string{"--version"})
	w.Close()
	os.Stdout = old
	out, e := io.ReadAll(r)
	r.Close()
	if e != nil {
		t.Fatal(e)
	}
	want := version.String + "\nbuild_info: " + version.Build + "\n© Sergey Porfiriev\n"
	if err != nil || string(out) != want {
		t.Fatalf("version=%q err=%v", out, err)
	}
}
