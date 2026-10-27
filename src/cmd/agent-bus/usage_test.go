package main

import (
	"fmt"
	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/version"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func documentedFlags(text string) []string {
	return documentedSectionFlags(text, "OPTIONS")
}

func documentedSectionFlags(text, section string) []string {
	var flags []string
	_, options, found := strings.Cut(text, "\n"+section+"\n")
	if !found {
		return nil
	}
	options, _, _ = strings.Cut(options, "\n\n")
	for _, line := range strings.Split(options, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-") {
			continue
		}
		for _, m := range regexp.MustCompile(`(?:^|, )--([a-z][a-z-]*)(?:[= ]|$)`).FindAllStringSubmatch(line, -1) {
			flags = append(flags, m[1])
		}
	}
	slices.Sort(flags)
	return slices.Compact(flags)
}

// Candidates come from parser source, not a hand-maintained flag list. Probe
// every candidate against the real parser before positional validation or a
// refusing fake transport. This also covers computed management flag names.
func TestParserFlagsMatchCommandHelp(t *testing.T) {
	candidates := map[string]bool{}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	word := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") || path == "usage.go" {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				s, _ := strconv.Unquote(lit.Value)
				if word.MatchString(s) {
					candidates[s] = true
				}
			}
			return true
		})
	}
	fields := make([]string, 0, len(candidates))
	for field := range candidates {
		fields = append(fields, field)
	}
	for _, op := range []string{"add", "add-to-set", "remove"} {
		for _, field := range fields {
			candidates[op+"-"+field] = true
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "fixture refuses", 403) }))
	defer server.Close()
	old := transport
	defer func() { transport = old }()
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	output, err := os.CreateTemp(t.TempDir(), "help")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	probes := map[string]func([]string) error{
		"register": register, "ls": ls, "send": send, "call": callVerb, "consume": consume, "reply": reply, "run": func(a []string) error { _, e := describe(a); return e },
		"ack": func(a []string) error { return receipt("ack", a) }, "done": func(a []string) error { return receipt("done", a) }, "publish": publish, "unsubscribe": unsubscribe,
		"manage": manage, "may-send": maySend, "stop": stopVerb, "logs": logsVerb, "config": agentTemplate, "secret": secret, "group": group, "enrol": enrol,
		"lock": lockTake(false), "try-lock": lockTake(true), "release": lockRelease, "extend": lockExtend, "holders": lockHolders,
		"kv": func(a []string) error { return kvVerb(append([]string{"set"}, a...)) }, "help": func(a []string) error { return agentHelp(a, output) },
		"status": func(a []string) error { _, f, e := split(a); return only(f, e) }, "unregister": func(a []string) error { _, f, e := split(a); return only(f, e) },
	}
	for _, g := range commandGroups {
		for _, c := range g.commands {
			if c.name != "token" && probes[c.name] == nil {
				t.Fatalf("uncovered parser %s", c.name)
			}
		}
	}
	for name, probe := range probes {
		t.Run(name, func(t *testing.T) {
			docs := documentedFlags(verbUsage(name))
			globals := documentedSectionFlags(verbUsage(name), "GLOBAL OPTIONS")
			if !slices.Equal(globals, []string{"addr", "agent", "help", "version"}) || !strings.Contains(verbUsage(name), "-h, --help") {
				t.Fatalf("%s globals/short forms incomplete: %v", name, globals)
			}

			for _, f := range docs {
				candidates[f] = true
			}
			var accepted []string
			for candidate := range candidates {
				e := probe([]string{"--" + candidate + "=fixture"})
				if e == nil || !strings.Contains(e.Error(), "unknown option --"+candidate) {
					accepted = append(accepted, candidate)
				}
			}
			// These legacy spellings remain accepted but are deliberately hidden.
			accepted = slices.DeleteFunc(accepted, func(f string) bool { return name == "register" && f == "addr" })
			// --table is handled before split, just like its hidden -h alias.
			if name == "ls" {
				accepted = append(accepted, "table")
			}
			slices.Sort(accepted)
			accepted = slices.Compact(accepted)
			if !slices.Equal(accepted, docs) {
				t.Fatalf("parser %v != help %v", accepted, docs)
			}
		})
	}
}

func cliHelpRun(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR=/nonexistent/rt27.sock", "AGENT_BUS_TOKEN=")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if e, ok := err.(*exec.ExitError); ok {
		return string(out), e.ExitCode()
	}
	t.Fatal(err)
	return "", -1
}
func TestManHelpDispatchAndExitCodes(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"help", "send"}, {"send", "--help"}, {"help", "call"}, {"call", "--help"}, {"help", "run"}} {
		out, code := cliHelpRun(t, args...)
		if code != 0 || !strings.Contains(out, "USAGE") {
			t.Fatalf("%v: code%d %s", args, code, out)
		}
		if regexp.MustCompile(`(^|[\s])docs/`).MatchString(out) {
			t.Fatal("bare documentation path")
		}
	}
	for _, args := range [][]string{{"unknown"}, {"send"}, {"status", "--unknown"}, {"status"}} {
		_, code := cliHelpRun(t, args...)
		if code != 1 {
			t.Fatalf("%v exited%d, want1", args, code)
		}
	}
	for _, g := range commandGroups {
		for _, c := range g.commands {
			a, code := cliHelpRun(t, "help", c.name)
			b, other := cliHelpRun(t, c.name, "--help")
			if code != 0 || other != 0 || a != b {
				t.Fatalf("different help paths for%s", c.name)
			}
		}
	}
	for _, hidden := range []string{"agent-template", "channel create", "--service-addr"} {
		if strings.Contains(usage, hidden) {
			t.Fatalf("overview contains detail/alias %s", hidden)
		}
	}
	if !strings.Contains(verbUsage("run"), "Run up to N") {
		t.Fatal("missing concurrency description")
	}
}
func TestLegacyAliasesReachSameDaemonCalls(t *testing.T) {
	var paths []string
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		fmt.Fprint(w, `[]`)
	}))
	defer server.Close()
	old := transport
	defer func() { transport = old }()
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	cases := []struct {
		f    func([]string) error
		a, b []string
	}{
		{register, []string{"svc", "--service-addr=x", "--protocol=demo"}, []string{"svc", "--addr=x", "--protocol=demo"}},
		{ls, []string{"--table", "--all"}, []string{"-h", "--all"}},
		{agentTemplate, []string{"fixture", "{}"}, []string{"fixture", "{}"}},
	}
	for _, c := range cases {
		paths = nil
		bodies = nil
		_ = c.f(c.a)
		_ = c.f(c.b)
		if len(paths) != 2 || paths[0] != paths[1] || bodies[0] != bodies[1] {
			t.Fatalf("alias diverged %v %v", paths, bodies)
		}
	}
	paths = nil
	bodies = nil
	_ = register([]string{"queue", "--kind=queue"})
	_ = channel([]string{"create", "queue"})
	if len(paths) != 2 || paths[0] != paths[1] || bodies[0] != bodies[1] {
		t.Fatalf("channel alias diverged %v", bodies)
	}
	for _, v := range []string{"config", "agent-template"} {
		out, code := cliHelpRun(t, v)
		if code != 1 || !strings.Contains(out, "config wants") {
			t.Fatalf("config dispatch missing %s: %s", v, out)
		}
	}
}

func TestTokenParserOptionsMatchHelp(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "../../internal/tokencmd/token.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var accepted []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, _ := strconv.Unquote(lit.Value)
		if strings.HasPrefix(s, "--") && !strings.ContainsAny(s, " \n") {
			s = strings.TrimPrefix(s, "--")
			s, _, _ = strings.Cut(s, "=")
			if s != "version" {
				accepted = append(accepted, s)
			}
		}
		return true
	})
	slices.Sort(accepted)
	accepted = slices.Compact(accepted)
	if !slices.Equal(documentedSectionFlags(verbUsage("token"), "GLOBAL OPTIONS"), []string{"addr", "agent", "help", "version"}) {
		t.Fatal("token global options incomplete")
	}
	if !slices.Equal(accepted, documentedFlags(verbUsage("token"))) {
		t.Fatalf("token parser %v != help %v", accepted, documentedFlags(verbUsage("token")))
	}
}

func TestManHelpLayout(t *testing.T) {
	var send, ls string
	for _, line := range strings.Split(usage, "\n") {
		if strings.HasPrefix(line, "  send <to>") {
			send = line
		}
		if strings.HasPrefix(line, "  ls [") {
			ls = line
		}
	}
	if strings.Contains(send, " — ") || strings.Index(send, "Send once") != strings.Index(ls, "List visible") || !strings.Contains(send, "<text>") {
		t.Fatalf("unaligned overview: %q %q", send, ls)
	}
	options := verbUsage("send")
	var topic, reply string
	for _, line := range strings.Split(options, "\n") {
		if strings.HasPrefix(line, "  --topic=") {
			topic = line
		}
		if strings.HasPrefix(line, "  --reply-to=") {
			reply = line
		}
	}
	if !strings.Contains(topic, "--topic=<topic>") || !strings.Contains(reply, "--reply-to=<name>") || strings.Index(topic, "Message topic") != strings.Index(reply, "Reply inbox") {
		t.Fatalf("unaligned options: %q %q", topic, reply)
	}
}

func TestHelpSeparatesHomepageAndSource(t *testing.T) {
	page := generalHelp()
	if !strings.HasPrefix(page, "agent-bus "+version.String+" — talk to agent-busd · "+display.Homepage+"\n") {
		t.Fatal("header does not use project homepage")
	}
	for _, page := range []string{page, verbUsage("help"), verbUsage("ls"), verbUsage("run")} {
		_, links, ok := strings.Cut(page, "SEE ALSO")
		if !ok || !strings.Contains(links, display.SourceURL+"/docs/user/cli.md") || !strings.HasSuffix(strings.TrimSpace(links), display.Source) {
			t.Fatal("SEE ALSO must list source documentation and repository")
		}
		if strings.Contains(links, display.Homepage) {
			t.Fatal("homepage used as documentation root")
		}
	}
}
