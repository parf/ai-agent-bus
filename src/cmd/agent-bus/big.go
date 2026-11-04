// The bigfile verbs: named values too big for a message, held by the daemon in
// a file or in memory (Plans/R1.0-Release/bigfile.md#help-text). This file
// checks what it can before any request and speaks the wire only through
// bigWire; the daemon decides access, size and expiry.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
)

const (
	bigDefaultTTL = 24 * time.Hour
	bigMaxTTL     = 30 * 24 * time.Hour
)

// bigPutPlan is a put with every local check done and nothing sent yet.
type bigPutPlan struct {
	Record, Name string
	Source       string // a file path, or "" for stdin
	TTL          time.Duration
	Memory       bool
}

// bigGetPlan is a get: Out is a file path, or "" for stdout.
type bigGetPlan struct{ Record, Name, Out string }

// parseBigTTL reads h, m and s durations and whole days: 90m, 24h, 30d.
func parseBigTTL(s string) (time.Duration, error) {
	var d time.Duration
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("--ttl is a duration such as 90m, 24h or 30d, not %q", s)
		}
		d = time.Duration(n) * 24 * time.Hour
	} else {
		var err error
		if d, err = time.ParseDuration(s); err != nil || d < time.Second {
			return 0, fmt.Errorf("--ttl is a duration such as 90m, 24h or 30d, not %q", s)
		}
	}
	if d < time.Second {
		return 0, fmt.Errorf("--ttl is at least 1s, not %q", s)
	}
	if d > bigMaxTTL {
		return 0, fmt.Errorf("--ttl is at most 30d, not %q", s)
	}
	return d, nil
}

// bigEmpty is the one refusal for a zero-byte file and for empty stdin.
var bigEmpty = errors.New("bigfile put wants a value with at least one byte; nothing was sent")

// takeShortO removes "-o <file>" from args: the option parser knows only --name.
func takeShortO(args []string) ([]string, string, bool, error) {
	var rest []string
	out, seen := "", false
	for i := 0; i < len(args); i++ {
		if args[i] != "-o" {
			rest = append(rest, args[i])
			continue
		}
		if seen || i+1 >= len(args) {
			return nil, "", false, fmt.Errorf("-o wants one file")
		}
		out, seen = args[i+1], true
		i++
	}
	return rest, out, seen, nil
}

func planBigPut(args []string, stdin io.Reader) (bigPutPlan, []byte, error) {
	pos, f, err := split(args)
	if err := only(f, err, "ttl", "memory"); err != nil {
		return bigPutPlan{}, nil, err
	}
	if len(pos) < 2 || len(pos) > 3 {
		return bigPutPlan{}, nil, fmt.Errorf("bigfile put wants <record> <name> [<file>|-]")
	}
	p := bigPutPlan{Record: pos[0], Name: pos[1], TTL: bigDefaultTTL, Memory: has(f, "memory")}
	if v, ok := f["ttl"]; ok {
		if p.TTL, err = parseBigTTL(v); err != nil {
			return p, nil, err
		}
	}
	if len(pos) == 3 && pos[2] != "-" {
		p.Source = pos[2]
	}
	return p, nil, nil
}

func planBigGet(args []string) (bigGetPlan, error) {
	rest, out, seen, err := takeShortO(args)
	if err != nil {
		return bigGetPlan{}, err
	}
	pos, f, err := split(rest)
	if err := only(f, err); err != nil {
		return bigGetPlan{}, err
	}
	if len(pos) != 2 {
		return bigGetPlan{}, fmt.Errorf("bigfile get wants <record> <name> [-o <file>]")
	}
	if seen && out == "" {
		return bigGetPlan{}, fmt.Errorf("-o wants one file")
	}
	return bigGetPlan{Record: pos[0], Name: pos[1], Out: out}, nil
}

// bigSource opens the put's bytes: a file, or stdin. A zero-byte file and
// empty stdin are refused alike, before any request.
func bigSource(p bigPutPlan, stdin io.Reader) (io.Reader, int64, error) {
	if p.Source == "" {
		// One byte past the cap tells an oversized value without reading all of it.
		b, err := io.ReadAll(io.LimitReader(stdin, bigMaxBytes+1))
		if err != nil {
			return nil, 0, err
		}
		if len(b) == 0 {
			return nil, 0, bigEmpty
		}
		if len(b) > bigMaxBytes {
			return nil, 0, fmt.Errorf("a big value is at most %d bytes; stdin held more", bigMaxBytes)
		}
		return strings.NewReader(string(b)), int64(len(b)), nil
	}
	st, err := os.Stat(p.Source)
	if err != nil {
		return nil, 0, err
	}
	if !st.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("%s is not a regular file", p.Source)
	}
	if st.Size() == 0 {
		return nil, 0, bigEmpty
	}
	f, err := os.Open(p.Source)
	return f, st.Size(), err
}

const bigMaxBytes = 32 << 20

// A bigfile transfer is bounded by one budget, not the 2 minutes api.Dial gives
// every call: the stall allowance plus the time the slowest promised rate
// (64 KiB/s, as the daemon's upload budget has it) needs for the bytes. A get
// does not know its size, so it gets the budget of the largest value.
var (
	bigIdle = 30 * time.Second
	bigRate = int64(64 << 10)
)

func bigBudget(size int64) time.Duration {
	if size < 1 || size > bigMaxBytes {
		size = bigMaxBytes
	}
	return bigIdle + time.Duration(size)*time.Second/time.Duration(bigRate)
}

// cancelOnClose ends the transfer's context when the caller is done with the body.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelOnClose) Close() error { err := c.ReadCloser.Close(); c.cancel(); return err }

// bigDo sends one streamed request; a refusal comes back as its text, as call's does.
func bigDo(method, path string, q url.Values, body io.Reader, size int64, header map[string]string) (*http.Response, error) {
	token := os.Getenv("AGENT_BUS_TOKEN")
	if token == "" && !onOwnSocket() {
		return nil, fmt.Errorf("set AGENT_BUS_TOKEN")
	}
	err := ensureDaemonProtocol(context.Background())
	if err != nil {
		return nil, err
	}
	client, base := transport()
	u := base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	ctx, cancel := context.WithTimeout(context.Background(), bigBudget(size))
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		cancel()
		return nil, err
	}
	// The client's own Timeout covers a whole request and is 2 minutes; the budget above replaces it.
	c := *client
	c.Timeout = 0
	client = &c
	if body != nil {
		req.ContentLength = size
		// Expect: 100-continue lets an early refusal arrive before the body is
		// sent, so a large put reports it rather than a reset connection.
		req.Header.Set("Expect", "100-continue")
		if t, ok := client.Transport.(*http.Transport); ok {
			t = t.Clone()
			t.ExpectContinueTimeout = 2 * time.Second
			defer t.CloseIdleConnections()
			client.Transport = t
		}
	}
	if token != "" {
		req.Header.Set(api.HeaderToken, token)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, away{err}
	}
	resp.Body = cancelOnClose{resp.Body, cancel}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		out, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(protocolRefusal(out, resp.StatusCode))))
	}
	return resp, nil
}

func bigVerb(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("bigfile wants put, get, ls or rm")
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "put":
		return bigPutVerb(rest)
	case "get":
		return bigGetVerb(rest)
	case "ls":
		return bigLsVerb(rest)
	case "rm":
		return bigRmVerb(rest)
	}
	return fmt.Errorf("bigfile wants put, get, ls or rm, not %q", verb)
}

type bigAnswer struct {
	Record   string `json:"record"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	Held     string `json:"held"`
	Expires  string `json:"expires"`
	Replaced bool   `json:"replaced"`
}

func bigPutVerb(args []string) error {
	plan, _, err := planBigPut(args, os.Stdin)
	if err != nil {
		return err
	}
	src, size, err := bigSource(plan, os.Stdin)
	if err != nil {
		return err
	}
	if c, ok := src.(io.Closer); ok {
		defer c.Close()
	}
	if size > bigMaxBytes {
		return fmt.Errorf("a big value is at most %d bytes, not %d", bigMaxBytes, size)
	}
	// Hash first so the daemon refuses a mismatch before it stores anything.
	var data []byte
	h := sha256.New()
	if f, ok := src.(*os.File); ok {
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
	} else {
		data, _ = io.ReadAll(src)
		h.Write(data)
		src = bytes.NewReader(data)
	}
	local := hex.EncodeToString(h.Sum(nil))
	q := url.Values{"record": {plan.Record}, "name": {plan.Name},
		"ttl": {strconv.FormatInt(int64(plan.TTL/time.Second), 10)}, "memory": {strconv.FormatBool(plan.Memory)}}
	resp, err := bigDo("PUT", "/bigfile", q, src, size, map[string]string{"Content-Type": "application/octet-stream", "X-Bigfile-Sha256": local})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var a bigAnswer
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&a); err != nil {
		return fmt.Errorf("the put was sent but its answer was unreadable, so it may be stored: %v", err)
	}
	if a.SHA256 != local {
		return fmt.Errorf("the daemon stored SHA-256 %s, this client sent %s", a.SHA256, local)
	}
	note := ""
	if a.Replaced {
		note = " (replaced)"
	}
	fmt.Printf("%s %s stored: %d bytes, SHA-256 %s, held in %s, expires %s%s\n", plan.Record, plan.Name, a.Size, a.SHA256, a.Held, a.Expires, note)
	return nil
}

func bigGetVerb(args []string) error {
	plan, err := planBigGet(args)
	if err != nil {
		return err
	}
	resp, err := bigDo("GET", "/bigfile", url.Values{"record": {plan.Record}, "name": {plan.Name}}, nil, 0, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	want := strings.ToLower(resp.Header.Get("X-Bigfile-Sha256"))
	if want == "" {
		return fmt.Errorf("the daemon sent the value without its SHA-256")
	}
	h := sha256.New()
	// Nothing reaches stdout or the named file until the whole value is verified.
	var dst io.Writer
	var tmp *os.File
	var buf bytes.Buffer
	if plan.Out != "" {
		if tmp, err = os.CreateTemp(filepath.Dir(plan.Out), ".big-get-*"); err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		dst = tmp
	} else {
		dst = &buf
	}
	n, err := io.Copy(io.MultiWriter(dst, h), io.LimitReader(resp.Body, bigMaxBytes+1))
	if err != nil {
		return away{err}
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return away{fmt.Errorf("the value ended after %d of %d bytes", n, resp.ContentLength)}
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("the value's SHA-256 is %s, the daemon said %s; nothing was written", got, want)
	}
	if tmp == nil {
		_, err = os.Stdout.Write(buf.Bytes())
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), plan.Out)
}

func bigLsVerb(args []string) error {
	pos, f, err := split(args)
	if err := only(f, err); err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("bigfile ls wants <record>")
	}
	out, code, err := call("GET", "/bigfile/list", url.Values{"record": {pos[0]}}, nil)
	if err != nil || code >= 400 {
		return show(out, code, err)
	}
	var res struct {
		Keys []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			SHA256  string `json:"sha256"`
			Held    string `json:"held"`
			Expires string `json:"expires"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return err
	}
	for _, k := range res.Keys {
		left := k.Expires
		if t, e := time.Parse(time.RFC3339, k.Expires); e == nil {
			left = time.Until(t).Round(time.Second).String()
		}
		fmt.Printf("%s\t%d\t%s\t%s\t%s\n", k.Name, k.Size, k.SHA256, k.Held, left)
	}
	return nil
}

func bigRmVerb(args []string) error {
	pos, f, err := split(args)
	if err := only(f, err); err != nil {
		return err
	}
	if len(pos) != 2 {
		return fmt.Errorf("bigfile rm wants <record> <name>")
	}
	out, code, err := call("POST", "/bigfile/delete", nil, map[string]string{"record": pos[0], "name": pos[1]})
	if err != nil || code >= 400 {
		return show(out, code, err)
	}
	var res struct {
		Deleted bool `json:"deleted"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return err
	}
	if res.Deleted {
		fmt.Printf("%s %s deleted\n", pos[0], pos[1])
	} else {
		fmt.Printf("%s %s held no bigfile\n", pos[0], pos[1])
	}
	return nil
}
