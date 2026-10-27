package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestAdminHelpProcess(t *testing.T) {
	if os.Getenv("AB_ADMIN_HELP_TEST") != "1" {
		return
	}
	for i, a := range os.Args {
		if a == "--" {
			os.Args = append([]string{"agent-bus-admin"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
}
func TestAdminHelpAndRealExitCodes(t *testing.T) {
	for _, c := range []struct {
		args []string
		code int
	}{{[]string{"--help"}, 0}, {nil, 1}} {
		cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestAdminHelpProcess$", "--"}, c.args...)...)
		cmd.Env = append(os.Environ(), "AB_ADMIN_HELP_TEST=1", "GORACE=atexit_sleep_ms=0")
		out, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			code = err.(*exec.ExitError).ExitCode()
		}
		if code != c.code {
			t.Fatalf("%v: %d %s", c.args, code, out)
		}
		if regexp.MustCompile(`(^|\s)docs/`).Match(out) {
			t.Fatal("bare docs path")
		}
	}
	for _, section := range []string{"USAGE", "MAIN COMMANDS", "OPTIONS", "ENVIRONMENT", "EXIT STATUS", "SEE ALSO"} {
		if !strings.Contains(usage, section) {
			t.Fatal(section)
		}
	}
}

func TestAdminCommandHelp(t *testing.T) {
	for _, name := range []string{"user", "user add", "user import-local", "user add-local", "user list", "user remove", "account", "account list", "account set", "account remove", "token", "tls"} {
		a := commandHelp(append([]string{"help"}, strings.Fields(name)...))
		b := commandHelp(append(strings.Fields(name), "--help"))
		if a == "" || a != b {
			t.Fatalf("missing command help %s", name)
		}
		if regexp.MustCompile(`(^|\s)docs/`).MatchString(a) {
			t.Fatal("bare docs path")
		}
	}
}

func TestAdminParserOptionsMatchHelp(t *testing.T) {
	accepted := map[string]bool{}
	for _, path := range []string{"main.go", "../../internal/tokencmd/token.go"} {
		data, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range regexp.MustCompile(`"--([a-z-]+)"`).FindAllStringSubmatch(string(data), -1) {
			if m[1] != "help" && m[1] != "version" {
				accepted[m[1]] = true
			}
		}
	}
	documented := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^  --([a-z-]+)[= ]`).FindAllStringSubmatch(usage, -1) {
		documented[m[1]] = true
	}
	if len(accepted) != len(documented) {
		t.Fatalf("parser%v help%v", accepted, documented)
	}
	for name := range accepted {
		if !documented[name] {
			t.Fatal("missing option " + name)
		}
	}
}
