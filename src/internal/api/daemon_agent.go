package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

// ServeDaemonAgent starts one in-process reader, after the reserved record is
// established. Closing joins it before the daemon's final queue checkpoint.
func (s *Server) ServeDaemonAgent(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if ctx.Err() != nil {
				return
			}
			e, err := s.bus.Consume(ctx, core.DaemonAgent, "", "", false, false)
			if err != nil {
				if ctx.Err() == nil {
					s.logs().Report(ports.Error, "#bus reader stopped: "+err.Error())
				}
				return
			}
			if e.Receipt != "" || e.Re != "" {
				continue
			}
			body, err := s.daemonAnswer(e)
			if err != nil {
				body = "error: " + err.Error()
			}
			back := protocol.ReplyTo{Name: e.From, Topic: e.Topic, Tag: e.Tag}
			if e.ReplyTo != nil {
				back = *e.ReplyTo
			}
			_, err = s.bus.Send(protocol.Envelope{From: core.DaemonAgent, To: back.Name, Topic: back.Topic, Tag: back.Tag, Re: e.ID, Body: body})
			if err != nil {
				s.logs().Report(ports.Warning, "#bus answer could not be delivered: "+err.Error())
			}
		}
	}()
	return func() { cancel(); <-done }
}

func (s *Server) daemonAnswer(e protocol.Envelope) (string, error) {
	// Trusted From was established by Send, never read from method arguments.
	if err := s.bus.Authenticate(e.From); err != nil {
		return "", err
	}
	if !e.Deadline.IsZero() && time.Now().After(e.Deadline) {
		return "", context.DeadlineExceeded
	}
	caller, err := protocol.ParseName(e.From)
	if err != nil {
		return "", err
	}
	// TODO(RT23/Q191): add the settled capabilities line to human help.
	methods := map[string]any{
		"help":    map[string]string{"help": "List the methods you may call"},
		"status":  map[string]string{"help": "Read daemon counts and uptime"},
		"version": map[string]string{"help": "Read daemon version and build"},
		"ls":      map[string]any{"help": "List records visible to you; same selection as agent-bus ls", "args": []any{map[string]any{"name": "args", "optional": true, "rest": true, "help": "[name] [--kind kind] [--tag tag] [--all]"}}},
	}
	if e.Ask != "" {
		body, _, err := agentmeta.DispatchAsk(e.Ask, map[string]agentmeta.AskHandler{
			"help-json": func() (string, error) {
				data, err := json.Marshal(map[string]any{"description": display.BusDescription, "version": version.String, "license": display.License, "author": display.Author, "homepage": display.Homepage, "docs": display.BusDocs, "methods": methods, "config": busConfigDeclared(), "secrets": []any{}, "asks": []string{"help", "help-json", "help-config", "help-secret", "mcp", "status"}, "body_formats": []string{"text"}})
				return string(data), err
			},
			"help": func() (string, error) {
				return s.daemonAnswer(protocol.Envelope{From: e.From, Body: "help", Deadline: e.Deadline})
			},
			"status": func() (string, error) {
				return s.daemonAnswer(protocol.Envelope{From: e.From, Body: "status", Deadline: e.Deadline})
			},
			"help-config": func() (string, error) {
				return agentmeta.Manifest{Config: busConfigDeclared()}.Answer("", "help-config", nil, false)
			},
			"help-secret": func() (string, error) { return "none", nil },
			"mcp":         func() (string, error) { return `{"tools":[]}`, nil },
		})
		return body, err
	}
	args := strings.Fields(e.Body)
	if len(args) == 0 {
		return "", fmt.Errorf("name a method: help, status, version, ls")
	}
	switch args[0] {
	case "help", "status", "version":
		if len(args) != 1 {
			return "", fmt.Errorf("%s takes no arguments", args[0])
		}
		switch args[0] {
		case "help":
			return "help — list methods\nstatus — daemon counts and uptime\nversion — daemon version and build\nls [name] [--kind kind] [--tag tag] [--all] — caller-visible records\n\nLicense: " + display.License + "\nAuthor: " + display.Author + "\nHomepage: " + display.Homepage + "\nDocs: " + display.BusDocs, nil
		case "status":
			st := s.bus.StatusFor(caller.String())
			return fmt.Sprintf("ok: %d records, %d readers, up %s", st.Records, st.Waiting, st.Up), nil
		default:
			return version.String + "\nbuild_info: " + version.Build, nil
		}
	case "ls":
		return s.daemonList(caller, args[1:])
	default:
		return "", fmt.Errorf("unknown method; ask help")
	}
}

// busConfigDeclared is the bus setup's keys as any agent declares its
// Configuration, so the ordinary edit page renders them.
func busConfigDeclared() map[string]agentmeta.Config {
	out := map[string]agentmeta.Config{}
	for _, k := range core.BusSettings {
		c := agentmeta.Config{Help: k.Help, Type: agentmeta.Type(`"string"`), Example: json.RawMessage(`"node.example"`)}
		if k.Max > 0 {
			c.Type = agentmeta.Type(`"int"`)
			c.Help += fmt.Sprintf(", 1 to %d", k.Max)
			c.Example = nil
			c.Default, _ = json.Marshal(k.Default)
		}
		out[k.Key] = c
	}
	return out
}

func (s *Server) daemonList(caller protocol.Name, args []string) (string, error) {
	query := url.Values{}
	name, all := "", false
	options := true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if options && arg == "--" {
			options = false
			continue
		}
		if options && (arg == "--all" || arg == "-all") {
			all = true
			continue
		}
		if options && arg == "-h" {
			continue
		} // #bus always renders text.
		if !options || !strings.HasPrefix(arg, "-") {
			if name != "" {
				return "", fmt.Errorf("ls takes one name, or none")
			}
			name = arg
			continue
		}
		key, value, has := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if key == "all" {
			all = true
			continue
		}
		if key != "kind" && key != "tag" {
			return "", fmt.Errorf("unknown ls option %q", arg)
		}
		if !has {
			i++
			if i >= len(args) || strings.HasPrefix(args[i], "-") {
				return "", fmt.Errorf("--%s needs a value", key)
			}
			value = args[i]
		}
		query.Set(key, value)
	}
	var records []protocol.Record
	if name != "" {
		record, known := s.bus.Lookup(caller.String(), name)
		if !known {
			return "", fmt.Errorf("no such name: %s", name)
		}
		records = []protocol.Record{record}
	} else {
		live := !all && query.Get("kind") == "" && query.Get("tag") == ""
		if live {
			query.Set("kind", protocol.KindAgent)
		}
		var err error
		records, err = s.listRecords(caller, query)
		if err != nil {
			return "", err
		}
		if live {
			kept := records[:0]
			for _, r := range records {
				if r.Readers != nil && *r.Readers > 0 {
					kept = append(kept, r)
				}
			}
			records = kept
		}
	}
	if len(records) == 0 {
		return "No matching records.", nil
	}
	lines := make([]string, 0, len(records))
	for _, r := range records {
		// Keep metadata on one line, as CLI human output does.
		descr := strings.Join(strings.Fields(strings.Map(func(c rune) rune {
			if unicode.IsControl(c) {
				return ' '
			}
			return c
		}, r.Descr)), " ")
		lines = append(lines, fmt.Sprintf("%s\t%s\t%s", r.Name, r.Kind, descr))
	}
	return strings.Join(lines, "\n"), nil
}
