package tokencmd

import (
	"strings"
	"testing"
)

func TestKeyRequiresAValueWithEitherLexicalForm(t *testing.T) {
	for _, args := range [][]string{{"alice@test", "--key=/private/key", "--rotate"}, {"alice@test", "--key", "/private/key", "--rotate"}} {
		{
			n, r, k, err := parse(args)
			if err != nil || n != "alice@test" || !r || k != "/private/key" {
				t.Fatalf("%q: %q %v %q %v", args, n, r, k, err)
			}
		}
	}
	for _, args := range [][]string{{"--key"}, {"--key="}, {"--key", "--rotate"}, {"--key", ""}} {
		if _, _, _, err := parse(args); err == nil || !strings.Contains(err.Error(), "file") {
			t.Fatalf("missing value accepted %q: %v", args, err)
		}
	}
}
func TestTokenRejectsRetiredOptionsBeforeCredentialWork(t *testing.T) {
	for _, flag := range []string{"-h", "-version", "--agent"} {
		if err := Run([]string{flag}); err == nil || !strings.Contains(err.Error(), "no such option") {
			t.Fatalf("%s: %v", flag, err)
		}
	}
}
