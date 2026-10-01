// Package bus is the Go client library for agent-bus: one authenticated
// caller, a registry of named records, messaging over inboxes, per-record
// key-value stores and shared locks
// (Plans/R1.0-Release/client-libraries/go.md).
//
// Every method takes a context.Context. Transport failures and daemon
// refusals are distinct error types; the client does not retry an
// uncertain send.
package bus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Options configures a Bus. Zero value discovers the account socket by uid
// and authenticates by the caller's uid on it; a Token switches to the
// shared socket. Name, when set, is checked on the first authenticated
// call.
type Options struct {
	Addr  string // unix socket path, http:// or https://; discovered when empty
	Token string // shared-socket credential; empty on an account socket
	Name  string // the expected identity; checked on the first authenticated call
}

// Bus is one authenticated agent-bus caller. Safe for concurrent use.
type Bus struct {
	addr  string
	token string
	http  *http.Client
}

// Identity is what GET /status answers about the caller.
type Identity struct {
	You          string `json:"you"`
	Administrator bool   `json:"administrator,omitempty"`
	DaemonOwner  bool   `json:"daemon_owner,omitempty"`
}

// Status is the node's current state.
type Status struct {
	Up        string         `json:"up"`
	Owner     string         `json:"owner"`
	Kinds     map[string]int `json:"kinds"`
	Queued    int            `json:"queued"`
	Waiting   int            `json:"waiting"`
	You       string         `json:"you"`
	Admin     bool           `json:"administrator,omitempty"`
	DaemonOwn bool           `json:"daemon_owner,omitempty"`
}

// ErrUnreachable is a transport failure: nothing listening, a dropped
// connection, a timeout. Not the daemon saying no.
var ErrUnreachable = fmt.Errorf("the daemon did not answer")

// Dial returns a Bus on the named principal. It discovers the account
// socket by uid, or takes an explicit address. Construction performs no
// I/O; the first authenticated call checks the daemon's answer.
func Dial(o Options) (*Bus, error) {
	addr := o.Addr
	if addr == "" {
		addr = discover()
		if addr == "" {
			return nil, fmt.Errorf("%w: no bus found; set AGENT_BUS_ADDR", ErrUnreachable)
		}
	}
	transport := &http.Client{}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		transport = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", addr)
				},
			},
		}
	}
	return &Bus{addr: addr, token: o.Token, http: transport}, nil
}

// Status reads the node's current state and checks the caller's identity.
func (b *Bus) Status(ctx context.Context) (*Status, error) {
	out, code, err := b.do(ctx, "GET", "/status", nil, nil)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, refusalError(code, out)
	}
	var st Status
	if err := json.Unmarshal(out, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// call sends one HTTP request to the daemon and decodes the answer.
func (b *Bus) call(ctx context.Context, method, path string, query map[string]string, body any) ([]byte, int, error) {
	url := b.url(path, query)
	var buf []byte
	if body != nil {
		var err error
		buf, err = json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if buf != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if b.token != "" {
		req.Header.Set("X-Agent-Bus-Token", b.token)
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()
	var out []byte
	if resp.Body != nil {
		out, err = readAll(resp.Body)
		if err != nil {
			return nil, resp.StatusCode, fmt.Errorf("%w: reading body", ErrUnreachable)
		}
	}
	return out, resp.StatusCode, nil
}

func (b *Bus) do(ctx context.Context, method, path string, query map[string]string, body any) ([]byte, int, error) {
	return b.call(ctx, method, path, query, body)
}

// url is the daemon's HTTP address. A unix socket path becomes
// http://unix; the transport dials it.
func (b *Bus) url(path string, query map[string]string) string {
	base := "http://unix"
	if isTCP(b.addr) {
		base = strings.TrimSuffix(b.addr, "/")
	}
	if len(query) == 0 {
		return base + path
	}
	parts := make([]string, 0, len(query))
	for k, v := range query {
		parts = append(parts, k+"="+v)
	}
	return base + path + "?" + strings.Join(parts, "&")
}

// isTCP says whether addr is a TCP port (plain or TLS) rather than a unix
// socket path.
func isTCP(addr string) bool {
	return strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://")
}

// discovery finds the account or shared socket on this host.
func discover() string {
	account := accountName()
	for _, dir := range discoveryDirs() {
		if account != "" {
			p := filepath.Join(dir, "user-"+account+".sock")
			if isSocket(p) {
				return p
			}
		}
		p := filepath.Join(dir, "bus.sock")
		if isSocket(p) {
			return p
		}
	}
	return ""
}

func discoveryDirs() []string {
	var dirs []string
	if x := os.Getenv("XDG_RUNTIME_DIR"); x != "" {
		dirs = append(dirs, filepath.Join(x, "agent-bus"))
	}
	dirs = append(dirs, "/run/agent-bus")
	return dirs
}

func accountName() string {
	uid := fmt.Sprint(os.Getuid())
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 && parts[2] == uid {
			return parts[0]
		}
	}
	return ""
}

func isSocket(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&os.ModeSocket != 0
}

func readAll(r io.Reader) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			if err.Error() == "EOF" {
				return buf, nil
			}
			return buf, err
		}
	}
}

func refusalError(code int, body []byte) error {
	msg := ""
	if len(body) > 0 {
		var e struct{ Error string }
		if json.Unmarshal(body, &e) == nil {
			msg = e.Error
		}
	}
	return fmt.Errorf("agent-bus: %d %s", code, msg)
}

// Close releases the underlying transport.
func (b *Bus) Close() { b.http.CloseIdleConnections() }
