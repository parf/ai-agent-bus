// Package agentmeta reads package-owned self descriptions, never host secrets.
package agentmeta

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

type Type json.RawMessage

func (t *Type) UnmarshalJSON(b []byte) error { *t = append((*t)[:0], b...); return nil }

func (t Type) Values() (string, []string, error) {
	if len(t) == 0 {
		return "string", nil, nil
	}
	var s string
	if json.Unmarshal(t, &s) == nil {
		switch s {
		case "string", "int", "bool", "list", "json":
			return s, nil, nil
		}
		return "", nil, fmt.Errorf("unknown type %q", s)
	}
	var values []string
	if json.Unmarshal(t, &values) != nil || len(values) == 0 {
		return "", nil, fmt.Errorf("type must be string, int, bool, list, json or a nonempty string enum")
	}
	seen := map[string]bool{}
	for _, v := range values {
		if seen[v] {
			return "", nil, fmt.Errorf("duplicate enum value %q", v)
		}
		seen[v] = true
	}
	return "enum", values, nil
}
func (t Type) MarshalJSON() ([]byte, error) {
	if len(t) == 0 {
		return []byte(`"string"`), nil
	}
	return t, nil
}

// Role accepts a role or an explicit any-of list; there is no inferred hierarchy.
type Role json.RawMessage

func (r *Role) UnmarshalJSON(b []byte) error { *r = append((*r)[:0], b...); return nil }
func (r Role) MarshalJSON() ([]byte, error)  { return r, nil }
func (r Role) Values() ([]string, error) {
	if len(r) == 0 {
		return nil, nil
	}
	var one string
	if json.Unmarshal(r, &one) == nil && one != "" {
		if !protocol.ValidRole(one) {
			return nil, fmt.Errorf("invalid role %q", one)
		}
		return []string{one}, nil
	}
	var roles []string
	if json.Unmarshal(r, &roles) != nil || len(roles) == 0 {
		return nil, fmt.Errorf("role must be a role name or a nonempty array of roles")
	}
	seen := map[string]bool{}
	for _, role := range roles {
		if !protocol.ValidRole(role) || seen[role] {
			return nil, fmt.Errorf("invalid or duplicate role %q", role)
		}
		seen[role] = true
	}
	return roles, nil
}

type Argument struct {
	Name     string          `json:"name"`
	Help     string          `json:"help"`
	Type     Type            `json:"type,omitempty"`
	Optional bool            `json:"optional,omitempty"`
	Rest     bool            `json:"rest,omitempty"`
	JSONOnly bool            `json:"json_only,omitempty"`
	Default  json.RawMessage `json:"default,omitempty"`
	shared   bool
}
type Method struct {
	Text        string `json:"text,omitempty"`
	Example     string `json:"example,omitempty"`
	order       int
	Help        string     `json:"help"`
	Type        Type       `json:"type,omitempty"`
	Args        []Argument `json:"args,omitempty"`
	Returns     string     `json:"returns,omitempty"`
	Role        Role       `json:"role,omitempty"`
	Destructive bool       `json:"destructive,omitempty"`
}
type Config struct {
	Help    string          `json:"help"`
	Type    Type            `json:"type,omitempty"`
	Example json.RawMessage `json:"example,omitempty"`
	Default json.RawMessage `json:"default,omitempty"`
}
type Needs struct {
	Network  []string `json:"network,omitempty"`
	Memory   string   `json:"memory,omitempty"`
	Writable []string `json:"writable,omitempty"`
}

// Manifest includes the existing runner fields so both consumers reject the
// same misspelled fields, including those nested in method and config entries.
type Manifest struct {
	BodyFormats []string            `json:"body_formats,omitempty"`
	Description string              `json:"description"`
	License     string              `json:"license,omitempty"`
	Version     string              `json:"version,omitempty"`
	Author      string              `json:"author,omitempty"`
	Homepage    string              `json:"homepage,omitempty"`
	Methods     Methods             `json:"methods,omitempty"`
	Arguments   map[string]Argument `json:"arguments,omitempty"`
	Config      map[string]Config   `json:"config,omitempty"`
	Command     string              `json:"command,omitempty"`
	Form        string              `json:"form,omitempty"`
	Serves      string              `json:"serves,omitempty"`
	Processes   json.RawMessage     `json:"processes,omitempty"`
	InFlight    int                 `json:"in_flight,omitempty"`
	Deadline    string              `json:"deadline,omitempty"`
	Idle        string              `json:"idle,omitempty"`
	Roles       []string            `json:"roles,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Depends     []string            `json:"depends,omitempty"`
	Needs       *Needs              `json:"needs,omitempty"`
}

func Decode(data []byte) (Manifest, error) {
	var m Manifest
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, fmt.Errorf("agent.json: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return m, fmt.Errorf("agent.json: expected one JSON object")
	}
	return m, m.Check()
}
func Load(dir string) (Manifest, error) {
	b, e := os.ReadFile(filepath.Join(dir, "agent.json"))
	if e != nil {
		return Manifest{}, e
	}
	return Decode(b)
}
func field(where, help string, t Type) error {
	if strings.TrimSpace(help) == "" {
		return fmt.Errorf("agent.json: %s.help is required", where)
	}
	if _, _, err := t.Values(); err != nil {
		return fmt.Errorf("agent.json: %s.type: %w", where, err)
	}
	return nil
}
func (m Manifest) Check() error {
	if m.BodyFormats != nil {
		if len(m.BodyFormats) == 0 {
			return fmt.Errorf("agent.json: body_formats must not be empty")
		}
		seen := map[string]bool{}
		for _, format := range m.BodyFormats {
			if (format != "text" && format != "json") || seen[format] {
				return fmt.Errorf("agent.json: invalid or duplicate body_formats value %q", format)
			}
			seen[format] = true
		}
	}

	if strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf("agent.json: description is required")
	}
	for name, argument := range m.Arguments {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("agent.json: empty shared argument name")
		}
		if err := field("arguments."+name, argument.Help, argument.Type); err != nil {
			return err
		}
	}
	for name, method := range m.Methods {
		where := "methods." + name
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("agent.json: empty method name")
		}
		if err := field(where, method.Help, method.Type); err != nil {
			return err
		}
		if _, err := method.Role.Values(); err != nil {
			return fmt.Errorf("agent.json: %s.role: %w", where, err)
		}
		seen := map[string]bool{}
		for i, a := range method.Args {
			var err error
			a, err = m.resolveArgument(a)
			if err != nil {
				return fmt.Errorf("agent.json: %s: %w", where, err)
			}
			if strings.TrimSpace(a.Name) == "" || seen[a.Name] {
				return fmt.Errorf("agent.json: %s.args has empty or duplicate name %q", where, a.Name)
			}
			seen[a.Name] = true
			if err := field(where+".args."+a.Name, a.Help, a.Type); err != nil {
				return err
			}
			if a.Rest && i != len(method.Args)-1 {
				return fmt.Errorf("agent.json: %s.args.%s: rest must be last", where, a.Name)
			}
		}
	}
	for name, c := range m.Config {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("agent.json: empty config key")
		}
		if err := field("config."+name, c.Help, c.Type); err != nil {
			return err
		}
	}
	return nil
}
