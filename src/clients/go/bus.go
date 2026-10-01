// Package bus connects one credentialed caller to agent-bus. It does not retry uncertain operations.
package bus

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrClosed = errors.New("bus closed")
var ErrReaderInUse = errors.New("inbox reader already in use")
var ErrUnreachable = errors.New("transport failed")
var ErrKVNotFound = errors.New("KV value or record not found")

type BusError struct {
	Status  int
	Message string
}

func (e *BusError) Error() string        { return fmt.Sprintf("daemon refused (%d): %s", e.Status, e.Message) }
func (e *BusError) Is(target error) bool { return target == ErrKVNotFound && e.Status == 404 }

type UncertainOutcome struct {
	Operation string
	Cause     error
}

func (e *UncertainOutcome) Error() string {
	return e.Operation + ": outcome uncertain: " + e.Cause.Error()
}
func (e *UncertainOutcome) Unwrap() error { return e.Cause }

type IdentityMismatch struct{ Expected, Actual string }

func (e *IdentityMismatch) Error() string {
	return fmt.Sprintf("expected caller %q, authenticated as %q", e.Expected, e.Actual)
}

type CallOutcome struct {
	Answered, Done, Acked bool
	Message               *Envelope
}
type ErrCallTimeout struct {
	Outcome CallOutcome
	Cause   error
}

func (e *ErrCallTimeout) Error() string { return "call ended before an answer: " + e.Cause.Error() }
func (e *ErrCallTimeout) Unwrap() error { return e.Cause }

type Options struct {
	Addr, Token, ExpectedName, TLSFingerprint string
	Timeout                                   time.Duration
}
type Status struct {
	You    string                     `json:"you"`
	Fields map[string]json.RawMessage `json:"-"`
}

func (s *Status) UnmarshalJSON(data []byte) error {
	type plain Status
	var v plain
	if e := json.Unmarshal(data, &v); e != nil {
		return e
	}
	*s = Status(v)
	return json.Unmarshal(data, &s.Fields)
}

type ReplyTo struct {
	Name  string `json:"name"`
	Topic string `json:"topic"`
	Tag   string `json:"tag"`
}
type Envelope struct {
	ID         string   `json:"message_id"`
	From       string   `json:"from"`
	To         string   `json:"to"`
	Body       string   `json:"body"`
	Topic      string   `json:"topic"`
	Tag        string   `json:"tag"`
	Receipt    string   `json:"receipt"`
	Re         string   `json:"re"`
	ReplyTo    *ReplyTo `json:"reply_to,omitempty"`
	Deadline   string   `json:"deadline,omitempty"`
	OriginalTo string   `json:"original_to,omitempty"`
	Forwards   int      `json:"forwards,omitempty"`
	TTL        string   `json:"ttl,omitempty"`
	Wait       string   `json:"wait,omitempty"`
	At         string   `json:"at,omitempty"`
	Expires    string   `json:"expires,omitempty"`
	roles      []string
	Fields     map[string]json.RawMessage `json:"-"`
}

func (e *Envelope) UnmarshalJSON(data []byte) error {
	type raw Envelope
	var v raw
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*e = Envelope(v)
	if err := json.Unmarshal(data, &e.Fields); err != nil {
		return err
	}
	if len(e.Fields["roles"]) > 0 {
		return json.Unmarshal(e.Fields["roles"], &e.roles)
	}
	return nil
}
func (e Envelope) Roles() []string { return append([]string(nil), e.roles...) }

// Roles are received authority metadata; they are never outgoing options.
type settings struct {
	topic, tag, inbox         string
	ttl, wait, timeout, drain time.Duration
	reply                     *ReplyTo
	workers                   int
	share                     bool
	waitSet                   bool
	onAck                     func(Envelope)
}
type Option func(*settings)

func WithTopic(v string) Option               { return func(s *settings) { s.topic = v } }
func WithTag(v string) Option                 { return func(s *settings) { s.tag = v } }
func WithInbox(v string) Option               { return func(s *settings) { s.inbox = v } }
func WithTTL(v time.Duration) Option          { return func(s *settings) { s.ttl = v } }
func WithWait(v time.Duration) Option         { return func(s *settings) { s.wait = v; s.waitSet = true } }
func WithTimeout(v time.Duration) Option      { return func(s *settings) { s.timeout = v } }
func WithWorkers(v int) Option                { return func(s *settings) { s.workers = v } }
func WithDrainTimeout(v time.Duration) Option { return func(s *settings) { s.drain = v } }
func WithReplyTo(v ReplyTo) Option            { return func(s *settings) { s.reply = &v } }
func WithAckObserver(v func(Envelope)) Option { return func(s *settings) { s.onAck = v } }
func WithShare(v bool) Option                 { return func(s *settings) { s.share = v } }
func config(opts []Option) settings {
	s := settings{workers: 1, drain: 30 * time.Second}
	for _, o := range opts {
		o(&s)
	}
	return s
}

type pending struct {
	inbox, topic, tag string
	ctx               context.Context
	ch                chan delivery
	poll              bool
}
type delivery struct {
	msg *Envelope
	err error
}
type work struct {
	delivery
	release func()
}
type serving struct {
	ctx        context.Context
	pollCtx    context.Context
	inbox      string
	topic, tag string
	polls      sync.WaitGroup
	inflight   atomic.Int32
	share      bool
	slots      chan struct{}
	ch         chan work
}
type Bus struct {
	client                        *http.Client
	transport                     *http.Transport
	base, token, expected, prefix string
	timeout                       time.Duration
	ctx                           context.Context
	cancel                        context.CancelFunc
	mu                            sync.Mutex
	you                           string
	verified                      bool
	verifying                     chan struct{}
	pending                       map[string]*pending
	raw                           bool
	serving                       *serving
	notify                        chan struct{}
	poll                          chan struct{}
}

func random() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func Dial(o Options) (*Bus, error) {
	if o.Addr == "" {
		o.Addr = os.Getenv("AGENT_BUS_ADDR")
	}
	if o.Token == "" {
		o.Token = os.Getenv("AGENT_BUS_TOKEN")
	}
	if o.TLSFingerprint == "" {
		o.TLSFingerprint = os.Getenv("AGENT_BUS_TLS_FINGERPRINT")
	}
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
	if o.Timeout < 0 {
		return nil, errors.New("positive timeout required")
	}
	if o.Addr == "" {
		o.Addr = defaultSocket(o.Token != "")
	}
	t := &http.Transport{DisableKeepAlives: true, ForceAttemptHTTP2: false, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, DialContext: (&net.Dialer{Timeout: o.Timeout}).DialContext}
	base := strings.TrimRight(o.Addr, "/")
	if strings.HasPrefix(base, "unix:") || strings.HasPrefix(base, "/") {
		path := strings.TrimPrefix(base, "unix:")
		t.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		}
		base = "http://localhost"
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("address must be a Unix path or HTTP(S) origin")
	}
	if o.TLSFingerprint != "" {
		if u.Scheme != "https" {
			return nil, errors.New("TLS pin requires HTTPS")
		}
		pin := strings.ToLower(strings.TrimSpace(o.TLSFingerprint))
		pin = strings.TrimPrefix(pin, "sha256:")
		pin = strings.ReplaceAll(pin, ":", "")
		want, e := hex.DecodeString(pin)
		if e != nil || len(want) != 32 {
			return nil, errors.New("expected SHA256 DER fingerprint")
		}
		t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true, VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("no certificate")
			}
			got := sha256.Sum256(cs.PeerCertificates[0].Raw)
			if subtle.ConstantTimeCompare(got[:], want) != 1 {
				return errors.New("TLS certificate fingerprint mismatch")
			}
			return nil
		}}
	}
	ctx, cancel := context.WithCancel(context.Background())
	b := &Bus{client: &http.Client{Transport: t, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, transport: t, base: base, token: o.Token, expected: o.ExpectedName, prefix: "call-" + random() + "-", timeout: o.Timeout, ctx: ctx, cancel: cancel, pending: map[string]*pending{}, notify: make(chan struct{}, 1), poll: make(chan struct{}, 1)}
	go b.dispatch()
	return b, nil
}
func defaultSocket(token bool) string {
	u, _ := user.Current()
	account := ""
	if u != nil {
		account = u.Username
	}
	dirs := []string{}
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		dirs = append(dirs, filepath.Join(d, "agent-bus"))
	}
	dirs = append(dirs, "/run/agent-bus")
	for _, d := range dirs {
		name := "bus.sock"
		if !token {
			name = "user-" + account + ".sock"
		}
		p := filepath.Join(d, name)
		if info, e := os.Stat(p); e == nil && info.Mode()&os.ModeSocket != 0 {
			return p
		}
	}
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "agent-bus", "bus.sock")
	}
	uid := ""
	if u != nil {
		uid = u.Uid
	}
	return filepath.Join(os.TempDir(), "agent-bus-"+uid+".sock")
}
func (b *Bus) wake() {
	select {
	case b.notify <- struct{}{}:
	default:
	}
}
func (b *Bus) Close() error { b.cancel(); b.transport.CloseIdleConnections(); b.wake(); return nil }
func (b *Bus) ensure(ctx context.Context) error {
	for {
		if b.ctx.Err() != nil {
			return ErrClosed
		}
		b.mu.Lock()
		if b.verified {
			b.mu.Unlock()
			return nil
		}
		if ch := b.verifying; ch != nil {
			b.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-b.ctx.Done():
				return ErrClosed
			case <-ch:
			}
			continue
		}
		ch := make(chan struct{})
		b.verifying = ch
		b.mu.Unlock()
		var s Status
		data, e := b.request(ctx, "GET", "/status", nil, nil)
		if e == nil {
			e = json.Unmarshal(data, &s)
		}
		if e == nil && s.You == "" {
			e = errors.New("status omitted caller")
		}
		if e == nil && b.expected != "" && b.expected != s.You {
			e = &IdentityMismatch{b.expected, s.You}
		}
		b.mu.Lock()
		if e == nil {
			b.verified = true
			b.you = s.You
		}
		b.verifying = nil
		close(ch)
		b.mu.Unlock()
		return e
	}
}
func (b *Bus) request(ctx context.Context, method, path string, q url.Values, body any) ([]byte, error) {
	if b.ctx.Err() != nil {
		return nil, ErrClosed
	}
	budget := b.timeout
	if path == "/consume" || path == "/lock" || path == "/try-lock" {
		if wait, e := time.ParseDuration(q.Get("wait")); e == nil && wait > 0 && wait <= 60*time.Second {
			budget += wait
		}
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	stop := context.AfterFunc(b.ctx, cancel)
	defer stop()
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	u := b.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	r, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	r.GetBody = nil // no replayable request body
	var sent atomic.Bool
	r = r.WithContext(httptrace.WithClientTrace(r.Context(), &httptrace.ClientTrace{WroteHeaders: func() { sent.Store(true) }}))
	if b.token != "" {
		r.Header.Set("X-Agent-Bus-Token", b.token)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	uncertain := method != "GET" || path == "/consume"
	failure := func(e error) error {
		if uncertain && sent.Load() {
			return &UncertainOutcome{method + " " + path, e}
		}
		if b.ctx.Err() != nil {
			return ErrClosed
		}
		return fmt.Errorf("%w: %w", ErrUnreachable, e)
	}
	resp, err := b.client.Do(r)
	if err != nil {
		return nil, failure(err)
	}
	defer resp.Body.Close()
	data, err = io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode >= 300 {
		var v struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &v)
		if v.Error == "" {
			v.Error = string(data)
		}
		return nil, &BusError{resp.StatusCode, v.Error}
	}
	if err != nil {
		return nil, failure(err)
	}
	return data, nil
}
func (b *Bus) json(ctx context.Context, method, path string, q url.Values, body, result any) error {
	if e := b.ensure(ctx); e != nil {
		return e
	}
	data, e := b.request(ctx, method, path, q, body)
	if e != nil {
		return e
	}
	if result == nil || len(data) == 0 {
		return nil
	}
	if e = json.Unmarshal(data, result); e != nil && (method != "GET" || path == "/consume") {
		return &UncertainOutcome{method + " " + path, e}
	}
	return e
}
func (b *Bus) Status(ctx context.Context) (Status, error) {
	var s Status
	e := b.json(ctx, "GET", "/status", nil, nil, &s)
	return s, e
}
func (b *Bus) Identity(ctx context.Context) (json.RawMessage, error) {
	var v json.RawMessage
	data, e := b.request(ctx, "GET", "/identity", nil, nil)
	if e == nil {
		e = json.Unmarshal(data, &v)
	}
	return v, e
}
func (b *Bus) Records(ctx context.Context, filters ...map[string]string) (json.RawMessage, error) {
	q := url.Values{}
	if len(filters) > 0 {
		for k, v := range filters[0] {
			q.Set(k, v)
		}
	}
	var v json.RawMessage
	e := b.json(ctx, "GET", "/ls", q, nil, &v)
	return v, e
}
func (b *Bus) Record(name string) Record {
	return Record{Name: name, bus: b, KV: KV{b, name}, Locks: Locks{b, name}}
}
func (b *Bus) Register(ctx context.Context, metadata map[string]any) (json.RawMessage, error) {
	var v json.RawMessage
	e := b.json(ctx, "POST", "/register", nil, metadata, &v)
	return v, e
}
func (b *Bus) Send(ctx context.Context, to, body string, options ...Option) (*Envelope, error) {
	s := config(options)
	v := map[string]any{"to": to, "body": body}
	if s.topic != "" {
		v["topic"] = s.topic
	}
	if s.tag != "" {
		v["tag"] = s.tag
	}
	if s.ttl > 0 {
		v["ttl"] = s.ttl.String()
	}
	if s.wait > 0 {
		v["wait"] = s.wait.String()
	}
	if s.reply != nil {
		v["reply_to"] = s.reply
	}
	return b.send(ctx, v)
}
func (b *Bus) send(ctx context.Context, v map[string]any) (*Envelope, error) {
	var m Envelope
	if e := b.json(ctx, "POST", "/send", nil, v, &m); e != nil {
		return nil, e
	}
	if m.ID == "" {
		return nil, &UncertainOutcome{"POST /send", errors.New("missing acceptance id")}
	}
	return &m, nil
}
func (b *Bus) Publish(ctx context.Context, channel, body string, opts ...Option) (*Envelope, error) {
	return b.Send(ctx, channel, body, opts...)
}
func (b *Bus) Unsubscribe(ctx context.Context, channel string) error {
	return b.json(ctx, "POST", "/subscribe", nil, map[string]any{"channel": channel, "off": true}, nil)
}
func (b *Bus) response(ctx context.Context, m *Envelope, body, receipt string) (*Envelope, error) {
	if m == nil || m.ID == "" || m.Receipt != "" {
		return nil, errors.New("response needs a request")
	}
	to, topic, tag := m.From, m.Topic, m.Tag
	if r := m.ReplyTo; r != nil {
		to, topic, tag = r.Name, r.Topic, r.Tag
	}
	v := map[string]any{"to": to, "body": body, "topic": topic, "tag": tag, "re": m.ID}
	if receipt != "" {
		v["receipt"] = receipt
	}
	return b.send(ctx, v)
}
func (b *Bus) Reply(ctx context.Context, m *Envelope, body string) (*Envelope, error) {
	return b.response(ctx, m, body, "")
}
func (b *Bus) Ack(ctx context.Context, m *Envelope) (*Envelope, error) {
	return b.response(ctx, m, "", "ack")
}
func (b *Bus) Done(ctx context.Context, m *Envelope) (*Envelope, error) {
	return b.response(ctx, m, "", "done")
}
func (b *Bus) consumeWire(ctx context.Context, s settings) (*Envelope, error) {
	select {
	case b.poll <- struct{}{}:
		defer func() { <-b.poll }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.ctx.Done():
		return nil, ErrClosed
	}
	if s.wait < 0 || s.wait > 60*time.Second {
		return nil, errors.New("consume wait must be between zero and 60s")
	}
	q := url.Values{"wait": {s.wait.String()}}
	if s.inbox != "" {
		q.Set("inbox", s.inbox)
	}
	if s.topic != "" {
		q.Set("topic", s.topic)
	}
	if s.tag != "" {
		q.Set("tag", s.tag)
	}
	if s.share {
		q.Set("share", "1")
	}
	var m *Envelope
	e := b.json(ctx, "GET", "/consume", q, nil, &m)
	return m, e
}
func (b *Bus) Consume(ctx context.Context, opts ...Option) (*Envelope, error) {
	b.mu.Lock()
	if b.raw || b.serving != nil || len(b.pending) > 0 {
		b.mu.Unlock()
		return nil, ErrReaderInUse
	}
	b.raw = true
	b.mu.Unlock()
	defer func() { b.mu.Lock(); b.raw = false; b.mu.Unlock(); b.wake() }()
	s := config(opts)
	if !s.waitSet {
		s.wait = 100 * time.Millisecond
	}
	return b.consumeWire(ctx, s)
}
func (b *Bus) Call(ctx context.Context, to, body string, opts ...Option) (CallOutcome, error) {
	s := config(opts)
	if s.timeout < 0 {
		return CallOutcome{}, errors.New("positive call timeout required")
	}
	if s.timeout == 0 {
		s.timeout = b.timeout
	}
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	if e := b.ensure(ctx); e != nil {
		if ctx.Err() != nil {
			return CallOutcome{}, &ErrCallTimeout{CallOutcome{}, ctx.Err()}
		}
		return CallOutcome{}, e
	}
	b.mu.Lock()
	if b.raw {
		b.mu.Unlock()
		return CallOutcome{}, ErrReaderInUse
	}
	inbox := s.inbox
	if s.reply != nil {
		inbox = s.reply.Name
	}
	if inbox == "" {
		inbox = b.you
	}
	tag := b.prefix + random()
	topic := s.topic
	if topic == "" {
		topic = "call"
	}
	p := &pending{inbox: inbox, topic: topic, tag: tag, ctx: ctx, ch: make(chan delivery, 32), poll: true}
	b.pending[tag] = p
	b.mu.Unlock()
	b.wake()
	defer func() { b.mu.Lock(); delete(b.pending, tag); b.mu.Unlock(); b.wake() }()
	options := []Option{WithTopic(topic), WithTag(tag), WithTTL(s.ttl)}
	if deadline, ok := ctx.Deadline(); ok {
		options = append(options, WithWait(time.Until(deadline)))
	}
	if inbox != b.you {
		options = append(options, WithReplyTo(ReplyTo{inbox, topic, tag}))
	}
	sent, e := b.Send(ctx, to, body, options...)
	if e != nil {
		return CallOutcome{}, e
	}
	out := CallOutcome{}
	for {
		select {
		case <-b.ctx.Done():
			return out, ErrClosed
		case <-ctx.Done():
			return out, &ErrCallTimeout{out, ctx.Err()}
		case d := <-p.ch:
			if b.ctx.Err() != nil {
				return out, ErrClosed
			}
			if ctx.Err() != nil {
				return out, &ErrCallTimeout{out, ctx.Err()}
			}
			if d.err != nil {
				return out, d.err
			}
			m := d.msg
			if m.Receipt != "" && m.Re != sent.ID {
				continue
			}
			switch m.Receipt {
			case "ack":
				out.Acked = true
				if s.onAck != nil {
					s.onAck(*m)
				}
			case "done":
				out.Done = true
				out.Message = m
				return out, nil
			case "":
				out.Answered = true
				out.Message = m
				return out, nil
			default:
				continue
			}
		}
	}
}
