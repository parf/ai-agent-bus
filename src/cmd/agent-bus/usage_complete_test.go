package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestAllCommandPagesHaveArgumentsAndExamples(t *testing.T) {
	var names []string
	for _, group := range commandGroups {
		for _, c := range group.commands {
			names = append(names, c.name)
		}
	}
	names = append(names, "help")
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			page := verbUsage(name)
			for _, section := range []string{"USAGE\n", "DESCRIPTION\n", "OPTIONS\n", "EXAMPLES\n", "SEE ALSO\n"} {
				if !strings.Contains(page, section) {
					t.Fatalf("%s missing %s", name, section)
				}
			}
			if !strings.Contains(strings.Split(page, "EXAMPLES\n")[1], "agent-bus ") {
				t.Fatal("no command example")
			}
			if name != "help" && (commandDetails[name].description == "" || commandDetails[name].examples == "") {
				t.Fatal("missing command detail")
			}
		})
	}
}

func TestEnumOptionsShowEveryAllowedValue(t *testing.T) {
	kinds := []string{protocol.KindUser, protocol.KindAgent, protocol.KindQueue, protocol.KindPubSub, protocol.KindService, protocol.KindResource}
	for _, c := range []struct {
		page, option string
		values       []string
	}{
		{"ls", "kind", kinds}, {"register", "kind", kinds}, {"register", "overflow", []string{protocol.OverflowRing, protocol.OverflowStrict}},
		{"run", "algo", []string{algoArgs, algoJSON}}, {"run", "sandbox", []string{"on", "off"}}, {"manage", "status", []string{protocol.StatusActive, protocol.StatusInactive}},
	} {
		page := verbUsage(c.page)
		r := regexp.MustCompile(`(?m)^  --` + c.option + `=<([^>]+)>`)
		m := r.FindStringSubmatch(page)
		if len(m) != 2 {
			t.Fatalf("%s --%s missing enum", c.page, c.option)
		}
		got := strings.Split(m[1], "|")
		if len(got) != len(c.values) {
			t.Fatalf("%s enum %v != %v", c.page, got, c.values)
		}
		for _, v := range c.values {
			if !strings.Contains("|"+m[1]+"|", "|"+v+"|") {
				t.Fatalf("missing %s", v)
			}
		}
	}
}

func TestShortAliasesAreInHelpAndStillParse(t *testing.T) {
	// Inspect actual short-option literals, including token's own help parser.
	for _, path := range []string{"main.go", "../../internal/tokencmd/token.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, _ := strconv.Unquote(lit.Value)
			if regexp.MustCompile(`^-[A-Za-z]$`).MatchString(v) && !strings.Contains(usage, v+", --help") {
				t.Fatalf("undocumented short form %s in %s", v, path)
			}
			return true
		})
	}
	if !strings.Contains(verbUsage("ls"), "-h, --table") {
		t.Fatal("ls short alias is hidden")
	}
	if !strings.Contains(verbUsage("token"), "-h, --help") {
		t.Fatal("token help alias is hidden")
	}
	if !strings.Contains(verbUsage("run"), "-N") {
		t.Fatal("numeric concurrency form is hidden")
	}
	a, e := describe([]string{"fixture", "/bin/true", "-4"})
	if e != nil || a.Instances != 4 {
		t.Fatalf("numeric short form %v %+v", e, a)
	}
}

func TestOldUsageFactsStayOnTheirCommandPages(t *testing.T) {
	facts := map[string][]string{
		"ls":       {"agents being read now", "<name> shows one record", "-h, --table", "default JSON"},
		"register": {"external service", "--service-addr and --protocol", "resource card", "who may see and use"},
		"run":      {"JSON from stdin", "Confinement is off", "no network unless --network", "AB_ROLE_<NAME>=1", "AGENT_BUS_CONFIG"},
		"manage":   {"--add appends", "--add-to-set adds without duplicates", "--remove removes", "empty value clears"},
		"kv":       {"default 1", "atomically: all or none", "--add writes only if absent", "--replace only if present", "Owner, Maintainers and own Agent"},
		"consume":  {"several workers behind the same name"}, "unsubscribe": {"owner manages that list"},
		"config":     {"Owner, Maintainers and own Agent", "AGENT_BUS_CONFIG", "unset when unconfigured", "never Configuration"},
		"secret":     {"Owner, Maintainers and own Agent", "web never displays stored Secret"},
		"unregister": {"does not stop a process"}, "lock": {"Owner, Maintainers", "default 30s"},
	}
	for name, values := range facts {
		page := strings.Join(strings.Fields(verbUsage(name)), " ")
		for _, v := range values {
			if !strings.Contains(page, v) {
				t.Fatalf("%s lost %q", name, v)
			}
		}
	}
	// Examples must be shell commands, not unquoted placeholder redirections.
	for _, d := range commandDetails {
		if regexp.MustCompile(`agent-bus (ack|done|reply) <`).MatchString(d.examples) {
			t.Fatal("non-executable example")
		}
	}
}
