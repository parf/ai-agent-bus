package bus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Bigfiles are named values too big for a message, kept by the daemon for a
// limited time and passed between callers by name.
// Plans/R1.0-Release/bigfile.md#help-text owns the rules; the daemon decides.

const (
	BigfileMaxBytes   = 32 << 20
	BigfileDefaultTTL = 24 * time.Hour
	BigfileMaxTTL     = 30 * 24 * time.Hour
)

// BigfileHashMismatch reports bytes whose SHA-256 is not the one the daemon named.
// A Get that returns it has already written to its writer; discard that.
type BigfileHashMismatch struct{ Want, Got string }

func (e *BigfileHashMismatch) Error() string {
	return "big value SHA-256 is " + e.Got + ", the daemon said " + e.Want
}

type BigfileInfo struct {
	Record   string    `json:"record"`
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	SHA256   string    `json:"sha256"`
	Held     string    `json:"held"`
	Expires  time.Time `json:"expires"`
	Replaced bool      `json:"replaced"`
}

type BigfileKey struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	SHA256  string    `json:"sha256"`
	Held    string    `json:"held"`
	Expires time.Time `json:"expires"`
}

type bigPut struct {
	ttl    time.Duration
	memory bool
	sha    string
}
type BigfileOption func(*bigPut)

// BigfileTTL sets the lifetime: whole seconds, one second to 30 days. Default 24h.
func BigfileTTL(d time.Duration) BigfileOption { return func(p *bigPut) { p.ttl = d } }

// BigfileMemory keeps the value in daemon memory only: gone at a restart.
func BigfileMemory() BigfileOption { return func(p *bigPut) { p.memory = true } }

// BigfileSHA256 sends the expected hex digest; the daemon refuses a mismatch before storing.
func BigfileSHA256(hexDigest string) BigfileOption { return func(p *bigPut) { p.sha = hexDigest } }

type Bigfile struct {
	bus    *Bus
	record string
}

// bigRequest sends one streamed request and returns the open response. The
// caller closes it. A refusal is a *BusError; a failure after the headers of
// a PUT or POST went out is an *UncertainOutcome.
func (b *Bigfile) bigRequest(ctx context.Context, method, path string, q url.Values, body io.Reader, size int64, header map[string]string) (*http.Response, error) {
	if e := b.bus.ensure(ctx); e != nil {
		return nil, e
	}
	if b.bus.ctx.Err() != nil {
		return nil, ErrClosed
	}
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(b.bus.ctx, cancel)
	u := b.bus.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	r, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		stop()
		cancel()
		return nil, err
	}
	r.GetBody = nil
	if body != nil {
		r.ContentLength = size
	}
	var sent atomic.Bool
	r = r.WithContext(httptrace.WithClientTrace(r.Context(), &httptrace.ClientTrace{WroteHeaders: func() { sent.Store(true) }}))
	if b.bus.token != "" {
		r.Header.Set("X-Agent-Bus-Token", b.bus.token)
	}
	for k, v := range header {
		r.Header.Set(k, v)
	}
	resp, err := b.bus.client.Do(r)
	if err != nil {
		stop()
		cancel()
		if method != "GET" && sent.Load() {
			return nil, &UncertainOutcome{method + " " + path, err}
		}
		if b.bus.ctx.Err() != nil {
			return nil, ErrClosed
		}
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	resp.Body = &closeAll{resp.Body, func() { stop(); cancel() }}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, b.bus.refusal(resp.StatusCode, data, method+" "+path)
	}
	return resp, nil
}

type closeAll struct {
	io.ReadCloser
	after func()
}

func (c *closeAll) Close() error { e := c.ReadCloser.Close(); c.after(); return e }

// Put uploads size bytes from r as the named bigfile. Nothing is sent when
// the size or lifetime is out of range. It is never retried.
func (b Bigfile) Put(ctx context.Context, name string, r io.Reader, size int64, opts ...BigfileOption) (BigfileInfo, error) {
	p := bigPut{ttl: BigfileDefaultTTL}
	for _, o := range opts {
		o(&p)
	}
	if size < 1 || size > BigfileMaxBytes {
		return BigfileInfo{}, fmt.Errorf("a big value is 1 byte to %d bytes, not %d", BigfileMaxBytes, size)
	}
	if p.ttl < time.Second || p.ttl > BigfileMaxTTL {
		return BigfileInfo{}, errors.New("a bigfile lives 1s to 30d")
	}
	h := sha256.New()
	q := url.Values{"record": {b.record}, "name": {name},
		"ttl": {strconv.FormatInt(int64(p.ttl/time.Second), 10)}, "memory": {strconv.FormatBool(p.memory)}}
	// Expect: 100-continue lets an early refusal (403, 413, 503) arrive before the body is sent,
	// so a large put reports it rather than a reset connection.
	header := map[string]string{"Content-Type": "application/octet-stream", "Expect": "100-continue"}
	if p.sha != "" {
		header["X-Bigfile-Sha256"] = strings.ToLower(p.sha)
	}
	resp, err := (&b).bigRequest(ctx, "PUT", "/bigfile", q, io.TeeReader(io.LimitReader(r, size), h), size, header)
	if err != nil {
		return BigfileInfo{}, err
	}
	defer resp.Body.Close()
	var info BigfileInfo
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return BigfileInfo{}, &UncertainOutcome{"PUT /bigfile", err}
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return BigfileInfo{}, &UncertainOutcome{"PUT /bigfile", err}
	}
	if got := hex.EncodeToString(h.Sum(nil)); info.SHA256 != got {
		return info, &BigfileHashMismatch{Want: got, Got: info.SHA256}
	}
	return info, nil
}

// Get streams the named bigfile to w and checks it against the digest and
// length the daemon sent. A *BigfileHashMismatch means discard what w received.
func (b Bigfile) Get(ctx context.Context, name string, w io.Writer) (BigfileInfo, error) {
	resp, err := (&b).bigRequest(ctx, "GET", "/bigfile", url.Values{"record": {b.record}, "name": {name}}, nil, 0, nil)
	if err != nil {
		return BigfileInfo{}, err
	}
	defer resp.Body.Close()
	want := strings.ToLower(resp.Header.Get("X-Bigfile-Sha256"))
	if want == "" {
		return BigfileInfo{}, errors.New("the daemon sent a big value without its SHA-256")
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), resp.Body)
	if err != nil {
		return BigfileInfo{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return BigfileInfo{}, fmt.Errorf("%w: big value ended after %d of %d bytes", ErrUnreachable, n, resp.ContentLength)
	}
	info := BigfileInfo{Record: b.record, Name: name, Size: n, SHA256: want, Held: resp.Header.Get("X-Bigfile-Held")}
	if t, e := time.Parse(time.RFC3339, resp.Header.Get("X-Bigfile-Expires")); e == nil {
		info.Expires = t
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return info, &BigfileHashMismatch{Want: want, Got: got}
	}
	return info, nil
}

func (b Bigfile) List(ctx context.Context) ([]BigfileKey, error) {
	var v struct {
		Keys []BigfileKey `json:"keys"`
	}
	if e := b.bus.json(ctx, "GET", "/bigfile/list", url.Values{"record": {b.record}}, nil, &v); e != nil {
		return nil, e
	}
	return v.Keys, nil
}

func (b Bigfile) Delete(ctx context.Context, name string) (bool, error) {
	var v struct {
		Deleted bool `json:"deleted"`
	}
	e := b.bus.json(ctx, "POST", "/bigfile/delete", nil, map[string]any{"record": b.record, "name": name}, &v)
	return v.Deleted, e
}
