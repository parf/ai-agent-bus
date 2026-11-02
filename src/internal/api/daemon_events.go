package api

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/eventlog"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type eventCall struct {
	Method, Log, Level, Text string
	Count                    int
	ID                       uint64
	// A subscription call: the target to copy to, the filter, and the id of one
	// to delete.
	Target, Filter, SubID string
}

var eventWrite = regexp.MustCompile(`(?s)^write +(\S+) +(\S+) (.*)$`)

const daemonMaxInput = 16384

// Oversized input is never passed to JSON, regexp or an unbounded token scan.
func shortWriteLooking(body string) bool {
	prefix := strings.ToLower(strings.TrimSpace(body[:min(len(body), 256)]))
	fields := strings.Fields(prefix)
	if len(fields) > 0 && fields[0] == "write" {
		return true
	}
	if strings.HasPrefix(prefix, "{") {
		if _, rest, ok := strings.Cut(prefix, `"method"`); ok {
			rest = strings.TrimSpace(rest)
			if strings.HasPrefix(rest, ":") {
				return strings.HasPrefix(strings.TrimSpace(rest[1:]), `"write"`)
			}
		}
	}
	return false
}
func eventMethod(body string) string {
	if len(body) > daemonMaxInput {
		if shortWriteLooking(body) {
			return "write"
		}
		return ""
	}
	if strings.HasPrefix(strings.TrimSpace(body), "{") {
		var v struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal([]byte(body), &v)
		return v.Method
	}
	parts := strings.Fields(body)
	if len(parts) > 1 && parts[0] == "sub" && (parts[1] == "add" || parts[1] == "list" || parts[1] == "delete") {
		return "sub." + parts[1]
	}
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

var eventWriteMember = regexp.MustCompile(`(?i)"method"\s*:\s*"write"`)

func writeLooking(body string) bool {
	if len(body) > daemonMaxInput {
		return shortWriteLooking(body)
	}
	return strings.EqualFold(eventMethod(body), "write") || strings.HasPrefix(strings.TrimSpace(body), "{") && eventWriteMember.MatchString(body)
}

func isEventMethod(method string) bool {
	return method == "write" || method == "read" || method == "list" || method == "pull" || method == "clean" ||
		method == "sub.add" || method == "sub.list" || method == "sub.delete"
}
func parseEventCall(body string) (eventCall, error) {
	c := eventCall{Method: eventMethod(body), Count: 100}
	bad := eventlog.Error("invalid_request")
	if len(body) > daemonMaxInput || !utf8.ValidString(body) || strings.ContainsRune(body, 0) {
		return c, bad
	}
	if strings.HasPrefix(c.Method, "sub.") {
		return parseSubCall(body, c)
	}
	if strings.HasPrefix(strings.TrimSpace(body), "{") {
		d := json.NewDecoder(strings.NewReader(body))
		tok, err := d.Token()
		if err != nil || tok != json.Delim('{') {
			return c, bad
		}
		fields := map[string]json.RawMessage{}
		for d.More() {
			tok, err = d.Token()
			key, ok := tok.(string)
			if err != nil || !ok {
				return c, bad
			}
			if _, ok = fields[key]; ok {
				return c, bad
			}
			var v json.RawMessage
			if d.Decode(&v) != nil {
				return c, bad
			}
			fields[key] = v
		}
		if tok, err = d.Token(); err != nil || tok != json.Delim('}') {
			return c, bad
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			return c, bad
		}
		allowed := map[string]bool{"method": true, "log": c.Method != "list", "level": c.Method != "list", "text": c.Method == "write", "count": c.Method == "pull", "id": c.Method == "clean"}
		for k, v := range fields {
			if !allowed[k] || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				return c, bad
			}
			switch k {
			case "method":
				if json.Unmarshal(v, &c.Method) != nil {
					return c, bad
				}
			case "log":
				if json.Unmarshal(v, &c.Log) != nil {
					return c, bad
				}
			case "level":
				if json.Unmarshal(v, &c.Level) != nil {
					return c, bad
				}
			case "text":
				if json.Unmarshal(v, &c.Text) != nil {
					return c, bad
				}
			case "count":
				if json.Unmarshal(v, &c.Count) != nil {
					return c, bad
				}
			case "id":
				if json.Unmarshal(v, &c.ID) != nil || c.ID == 0 {
					return c, bad
				}
			}
		}
		if c.Method == "write" {
			if _, ok := fields["text"]; !ok {
				return c, bad
			}
			if _, ok := fields["level"]; !ok {
				return c, bad
			}
		}
	} else {
		body = strings.TrimLeft(body, " ")
		if c.Method == "write" {
			m := eventWrite.FindStringSubmatch(body)
			if m == nil {
				return c, bad
			}
			c.Log, c.Level, c.Text = m[1], m[2], m[3]
		} else {
			args := strings.Fields(body)
			if len(args) < 1 {
				return c, bad
			}
			if c.Method == "list" {
				if len(args) != 1 {
					return c, bad
				}
			} else {
				if len(args) < 2 || len(args) > 4 {
					return c, bad
				}
				c.Log = args[1]
				args = args[2:]
				if len(args) > 0 && eventlog.IsLevel(args[0]) {
					c.Level = args[0]
					args = args[1:]
				}
				if len(args) > 0 {
					if len(args) > 1 || c.Method == "read" {
						return c, bad
					}
					n, err := strconv.ParseUint(args[0], 10, 64)
					if err != nil || n == 0 {
						return c, bad
					}
					if c.Method == "pull" {
						if n > 500 {
							return c, bad
						}
						c.Count = int(n)
					} else {
						c.ID = n
					}
				}
			}
		}
	}
	if !isEventMethod(c.Method) || c.Method != "list" && c.Log == "" || c.Count < 1 || c.Count > 500 || c.ID > eventlog.Safe {
		return c, bad
	}
	if c.Level != "" && !eventlog.IsLevel(c.Level) || c.Method == "write" && c.Level == "" {
		return c, eventlog.Error("invalid_level")
	}
	return c, nil
}
func eventFailure(err error, write bool) string {
	code := eventlog.Code(err)
	texts := map[string]string{"forbidden": "Only the log’s Owner, Maintainers or daemon Owner may read or manage it.", "not_found": "No such log or subscription.", "invalid_request": "Invalid call; ask help.", "invalid_level": "Use notice, warning or alert.", "invalid_log": "Invalid agent name.", "invalid_text": "Invalid entry text.", "busy": "Agent event dispatch is busy.", "invalid_target": "Invalid subscription target.", "invalid_selector": "Invalid subscription level.", "invalid_filter": "Invalid subscription filter.", "invalid_id": "Invalid subscription id.", "limit": "Subscription limit reached.", "not_author": "Only the author or a manager of the log may change this subscription.", "admission_refused": "The author may not send to that target.", "admission_unavailable": "The target could not be checked.", "store_unknown": "Subscription storage outcome is unknown.", "rate_limit": "Agent event rate limit reached.", "storage_invalid": "Persistent ring state is invalid.", "storage_unknown": "Ring storage outcome is unknown; reopen validated state.", "deadline": "Insufficient time to store this entry."}
	text, ok := texts[code]
	if !ok {
		code = "internal_error"
		text = "The operation could not be completed."
	}
	if write {
		if code == "forbidden" {
			text = "Only the agent itself or daemon Owner may write this log."
		}
		return "error: " + code + ": " + text
	}
	out, _ := json.Marshal(map[string]any{"error": map[string]string{"code": code, "text": text}})
	return string(out)
}
func (s *Server) daemonEventAnswer(e protocol.Envelope) string {
	c, err := parseEventCall(e.Body)
	if err != nil {
		return eventFailure(err, c.Method == "write")
	}
	var result any
	switch c.Method {
	case "write":
		v, deliveries, er := s.bus.EventWriteWithin(e.From, c.Log, c.Level, c.Text, e.Deadline)
		err = er
		result = map[string]any{"entry": v, "deliveries": deliveries}
	case "sub.add":
		result, err = s.bus.SubAdd(e.From, c.Log, c.Target, c.Level, c.Filter)
	case "sub.list":
		var rows []core.SubscriptionEntry
		rows, err = s.bus.SubList(e.From, c.Log)
		result = map[string]any{"subscriptions": rows}
	case "sub.delete":
		err = s.bus.SubDelete(e.From, c.SubID)
		result = map[string]any{"deleted": true}
	case "read":
		result, err = s.bus.EventRead(e.From, c.Log, c.Level)
	case "list":
		result, err = s.bus.EventList(e.From)
	case "pull":
		result, err = s.bus.EventPull(e.From, c.Log, c.Level, c.Count)
	case "clean":
		result, err = s.bus.EventClean(e.From, c.Log, c.Level, c.ID)
	}
	if err != nil {
		return eventFailure(err, c.Method == "write")
	}
	raw, err := json.Marshal(result)
	if err != nil || len(raw) > 1024*1024 {
		return eventFailure(eventlog.Error("internal_error"), c.Method == "write")
	}
	return string(raw)
}

func (s *Server) eventMethods(caller string) map[string]any {
	write, read := s.bus.EventCapabilities(caller)
	out := map[string]any{}
	for _, row := range []struct {
		name, help, args string
		allowed          bool
	}{
		{"write", "Append to your own agent log; wait for an answer only when requested", "<agent> notice|warning|alert <text>", write},
		{"read", "Read a managed agent's complete private log", "<agent> [level]", read},
		{"list", "List private logs you manage", "", read},
		{"pull", "Remove oldest entries once from a managed agent log", "<agent> [level] [count]", read},
		{"clean", "Clear a managed agent log, optionally through an inclusive entry id", "<agent> [level] [id]", read},
		{"sub add", "Copy each new entry of a managed agent log to a queue or agent, from now on", "<target> <agent> [level] [/regex/]", read},
		{"sub list", "List the subscriptions you made or can manage", "[agent]", read},
		{"sub delete", "Delete a subscription", "<id>", read},
	} {
		if row.allowed {
			method := map[string]any{"help": row.help}
			if row.name == "pull" || row.name == "clean" || row.name == "sub delete" {
				method["destructive"] = true
			}
			if row.args != "" {
				method["args"] = []any{map[string]any{"name": "args", "rest": true, "help": row.args}}
			}
			out[row.name] = method
		}
	}
	return out
}
func (s *Server) eventHelp(caller string) string {
	write, read := s.bus.EventCapabilities(caller)
	text := ""
	if write {
		text += "write <agent> notice|warning|alert <text> — append to your own log; answers only when wait is set\n"
	}
	if read {
		text += "read <agent> [level] — complete private log\nlist — private log summaries\npull <agent> [level] [count] — remove oldest entries\nclean <agent> [level] [id] — clear entries through id\nsub add <target> <agent> [level] [/regex/] — copy each new entry to a queue or agent\nsub list [agent] — subscriptions you made or manage\nsub delete <id> — delete a subscription\n"
	}
	return text
}

// inspectDaemonInput is the reader's first input operation. Keeping the scanner
// behind this guard lets the owning check count full scans, not time a machine.
func inspectDaemonInput(e protocol.Envelope, scan func(string) (eventCall, error)) (eventCall, error) {
	if len(e.Body) > daemonMaxInput {
		c := eventCall{}
		if shortWriteLooking(e.Body) {
			c.Method = "write"
		}
		return c, eventlog.Error("invalid_request")
	}
	if e.Ask != "" {
		return eventCall{}, nil
	}
	return scan(e.Body)
}
func scanDaemonInput(body string) (eventCall, error) {
	method := eventMethod(body)
	if isEventMethod(method) {
		return parseEventCall(body)
	}
	return eventCall{Method: method}, nil
}
func (s *Server) dispatchDaemonTask(d *daemonDispatch, e protocol.Envelope) bool {
	c, err := inspectDaemonInput(e, scanDaemonInput)
	if err != nil {
		s.replyDaemonTask(e, eventFailure(err, c.Method == "write"))
		return false
	}
	// Reject forbidden subjects before they can reserve anybody's lane. This
	// check does no store I/O; the execution and precommit checks still decide.
	key := ""
	mutation := false
	if e.Ask == "" && isEventMethod(c.Method) && c.Method != "list" && c.Log != "" {
		_, err = s.bus.EventAdmit(e.From, c.Log, c.Method == "write")
		if err != nil {
			s.replyDaemonTask(e, eventFailure(err, c.Method == "write"))
			return false
		}
		key = c.Log
		if n, er := protocol.ParseName(key); er == nil {
			key = n.String()
		}
		mutation = c.Method == "write" || c.Method == "pull" || c.Method == "clean" || c.Method == "sub.add"
	} else if e.Ask == "" && c.Method == "sub.delete" {
		// A delete is ordered with the caller's own, not with any log's writes.
		key, mutation = e.From, true
	} else if e.Ask == "" && writeLooking(e.Body) {
		key = e.From
		mutation = true
	}
	health := e.Ask != "" || (!isEventMethod(c.Method) && !mutation)
	if d.enqueueAt(e, key, mutation, health) {
		return true
	}
	if c.Method == "write" {
		s.bus.EventAdmissionDrop(e.From, c.Log)
	}
	s.replyDaemonTask(e, eventFailure(eventlog.Error("busy"), writeLooking(e.Body)))
	return false
}
