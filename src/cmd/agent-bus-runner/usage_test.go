package main

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestRunnerHelpProcess(t *testing.T) {
	if os.Getenv("AB_RUNNER_HELP_TEST") != "1" {
		return
	}
	for i, a := range os.Args {
		if a == "--" {
			os.Args = append([]string{"agent-bus-runner"}, os.Args[i+1:]...)
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			main()
			os.Exit(0)
		}
	}
}
func TestRunnerHelpAndRealExitCodes(t *testing.T) {
	for _, c := range []struct {
		args []string
		code int
	}{{[]string{"--help"}, 0}, {nil, 1}, {[]string{"--unknown"}, 2}, {[]string{"--bundled="}, 1}} {
		cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestRunnerHelpProcess$", "--"}, c.args...)...)
		cmd.Env = append(os.Environ(), "AB_RUNNER_HELP_TEST=1", "GORACE=atexit_sleep_ms=0")
		out, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			code = err.(*exec.ExitError).ExitCode()
		}
		if code != c.code {
			t.Fatalf("%v: %d %s", c.args, code, out)
		}
		if len(c.args) == 1 && c.args[0] == "--bundled=" && !strings.Contains(string(out), "--bundled wants a directory") {
			t.Fatalf("empty bundled directory was not refused: %s", out)
		}
		if regexp.MustCompile(`(^|\s)docs/`).Match(out) {
			t.Fatal("bare docs path")
		}
	}
	f, e := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	ast.Inspect(f, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		s, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || s.Sel.Name != "String" {
			return true
		}
		x, ok := s.X.(*ast.Ident)
		if !ok || x.Name != "flag" {
			return true
		}
		lit, ok := c.Args[0].(*ast.BasicLit)
		if !ok {
			t.Fatal("flag name not literal")
		}
		name := strings.Trim(lit.Value, `"`)
		options := strings.Split(strings.Split(usage, "OPTIONS\n")[1], "\nBUS COMMANDS")[0]
		if !strings.Contains(options, "--"+name+"=") {
			t.Fatalf("undocumented %s", name)
		}
		count++
		return true
	})
	if got := len(regexp.MustCompile(`(?m)^  --\w+=`).FindAllString(usage, -1)); got != count {
		t.Fatalf("%d help options vs %d parser", got, count)
	}
}

func TestRunnerHelpKeepsStartupAndControlFacts(t *testing.T) {
	for _, text := range []string{"DESCRIPTION\n", "EXAMPLES\n", "GLOBAL OPTIONS\n", "--version", "-h, --help", "agent.d/*/agent.json", "runner/agents.json", "never to the person", "until the next reload", "enable and disable persist", "host files", "instances"} {
		if !strings.Contains(strings.Join(strings.Fields(usage), " "), strings.Join(strings.Fields(text), " ")) {
			t.Fatalf("lost runner fact %q", text)
		}
	}
}
