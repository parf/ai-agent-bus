package agentmeta

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/parf/ai-agent-bus/internal/envfile"
	"github.com/parf/ai-agent-bus/internal/version"
)

func IsAsk(ask string) bool {
	return ask == "help" || ask == "help-config" || ask == "help-secret" || ask == "help-json"
}
func allowed(role Role, roles []string) bool {
	needed, _ := role.Values()
	if len(needed) == 0 {
		return true
	}
	for _, r := range roles {
		if r == "owner" || r == "maintainer" {
			return true
		}
		for _, n := range needed {
			if r == n {
				return true
			}
		}
	}
	return false
}
func typeText(t Type) string {
	kind, values, _ := t.Values()
	if kind == "enum" {
		return strings.Join(values, "|")
	}
	return kind
}
func keys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func normalized(t Type) Type {
	if len(t) == 0 {
		return Type(`"string"`)
	}
	return t
}

// Answer returns package help without executing its command or reading any host
// env file, bus Secret or Configuration value. jsonForm selects data, not prose.
func (m Manifest) Answer(dir, ask string, roles []string, jsonForm bool, target ...string) (string, error) {
	var data any
	var b strings.Builder
	switch ask {
	case "help-json":
		// Compose the same filtered fields without converting JSON numbers through float64.
		whole := map[string]json.RawMessage{}
		for _, part := range []string{"help", "help-config", "help-secret"} {
			answer, err := m.Answer(dir, part, roles, true)
			if err != nil {
				return "", err
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(answer), &fields); err != nil {
				return "", err
			}
			for key, value := range fields {
				whole[key] = value
			}
		}

		formats := m.BodyFormats
		if len(formats) == 0 {
			formats = []string{"text"}
		}
		whole["body_formats"], _ = json.Marshal(formats)
		hidden := struct {
			Count int      `json:"count"`
			Roles []string `json:"roles"`
		}{Roles: []string{}}
		needed := map[string]bool{}
		for _, method := range m.Methods {
			if allowed(method.Role, roles) {
				continue
			}
			hidden.Count++
			names, _ := method.Role.Values()
			for _, role := range names {
				needed[role] = true
			}
		}
		for role := range needed {
			hidden.Roles = append(hidden.Roles, role)
		}
		sort.Strings(hidden.Roles)
		whole["hidden_methods"], _ = json.Marshal(hidden)
		raw, err := json.Marshal(whole)
		return string(raw), err
	case "help":
		methods := Methods{}
		for name, method := range m.Methods {
			if allowed(method.Role, roles) {
				method.Type = normalized(method.Type)
				method.Args = append([]Argument(nil), method.Args...)
				for i := range method.Args {
					a, err := m.resolveArgument(method.Args[i])
					if err != nil {
						return "", err
					}
					a.Type = normalized(a.Type)
					method.Args[i] = a
				}
				methods[name] = method
			}
		}
		v := m.Version
		if v == "" {
			v = version.String
		}
		data = struct {
			Description string   `json:"description"`
			License     string   `json:"license,omitempty"`
			Version     string   `json:"version"`
			Author      string   `json:"author,omitempty"`
			Homepage    string   `json:"homepage,omitempty"`
			Docs        string   `json:"docs,omitempty"`
			Methods     Methods  `json:"methods"`
			Asks        []string `json:"asks"`
		}{m.Description, m.License, v, m.Author, m.Homepage, m.Docs, methods, []string{"help", "help-config", "help-secret", "help-json"}}
		b.WriteString(m.Description + "\n")
		shared := map[string]Argument{}
		examples := []string{}
		for _, name := range methods.names() {
			if name == "help" {
				continue
			}
			method := methods[name]
			usage := method.Text
			if usage == "" {
				usage = name
				for _, a := range method.Args {
					if a.JSONOnly {
						continue
					}
					arg := a.Name
					if a.Rest {
						arg += "..."
					}
					if a.Optional {
						usage += " [" + arg + "]"
					} else {
						usage += " <" + arg + ">"
					}
				}
			}
			if method.Destructive {
				b.WriteString("[destructive] ")
			}
			fmt.Fprintf(&b, "%s: %s", usage, method.Help)
			if method.Returns != "" {
				fmt.Fprintf(&b, "; returns: %s", method.Returns)
			}
			if len(method.Role) > 0 {
				roles, _ := method.Role.Values()
				fmt.Fprintf(&b, "; role: %s", strings.Join(roles, "|"))
			}
			b.WriteByte('\n')
			for _, a := range method.Args {
				if a.JSONOnly {
					continue
				}
				if a.shared {
					shared[a.Name] = a
					continue
				}
				writeArgument(&b, "  ", a.Name, a)
			}
			if method.Example != "" {
				examples = append(examples, method.Example)
			}
		}
		if len(shared) > 0 {
			b.WriteString("\nArguments\n")
			for _, name := range keys(shared) {
				writeArgument(&b, "", name, shared[name])
			}
		}
		if len(examples) > 0 {
			b.WriteString("\nExamples\n" + strings.Join(examples, "\n") + "\n")
		}
		license := strings.ReplaceAll(m.License, "PolyForm-Noncommercial-1.0.0", "PolyForm-NC")
		footer := []string{"v" + v}
		if license != "" {
			footer = append(footer, license)
		}
		if m.Homepage != "" {
			footer = append(footer, strings.TrimPrefix(strings.TrimPrefix(m.Homepage, "https://"), "http://"))
		}
		name := "<name>"
		if len(target) > 0 && target[0] != "" {
			name = "'" + strings.ReplaceAll(target[0], "'", "'\"'\"'") + "'"
		}
		b.WriteString("\n" + strings.Join(footer, " · ") + "\n")
		if m.Docs != "" {
			b.WriteString("Docs: " + m.Docs + "\n")
		}
		b.WriteString("More: agent-bus help " + name + " config | secret | json\n")
	case "help-config":
		config := map[string]Config{}
		for name, c := range m.Config {
			c.Type = normalized(c.Type)
			config[name] = c
		}
		data = map[string]any{"config": config}
		for _, name := range keys(config) {
			c := config[name]
			fmt.Fprintf(&b, "%s (%s): %s", name, typeText(c.Type), c.Help)
			if len(c.Example) > 0 {
				fmt.Fprintf(&b, "; example: %s", c.Example)
			}
			if len(c.Default) > 0 {
				fmt.Fprintf(&b, "; default: %s", c.Default)
			}
			b.WriteByte('\n')
		}
		if len(config) == 0 {
			b.WriteString("No Configuration keys declared.\n")
		}
	case "help-secret":
		src, err := ReadFile(filepath.Join(dir, "env.dist"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		secrets, err := envfile.Describe(src)
		if err != nil {
			return "", fmt.Errorf("env.dist: %w", err)
		}
		data = map[string]any{"secrets": secrets}
		for _, s := range secrets {
			required := "optional"
			if s.Required {
				required = "required"
			}
			defaultNote := ""
			if s.HasDefault {
				defaultNote = "; has default"
			}
			fmt.Fprintf(&b, "%s (%s%s): %s\n", s.Name, required, defaultNote, s.Help)
		}
		if len(secrets) == 0 {
			b.WriteString("No environment keys declared.\n")
		}
	default:
		return "", fmt.Errorf("unknown metadata ask %q", ask)
	}
	if jsonForm {
		out, err := json.Marshal(data)
		return string(out), err
	}
	return strings.TrimSpace(b.String()), nil
}

func writeArgument(b *strings.Builder, prefix, name string, a Argument) {
	fmt.Fprintf(b, "%s%s (%s): %s", prefix, name, typeText(a.Type), a.Help)
	if len(a.Default) > 0 {
		fmt.Fprintf(b, "; default: %s", a.Default)
	}
	b.WriteByte('\n')
}
