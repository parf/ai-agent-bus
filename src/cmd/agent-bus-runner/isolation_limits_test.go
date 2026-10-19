package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

func TestKeptAgentRefusesAnAccountSocketEvenNamedBusSock(t *testing.T) {
	d := shortSocketDir(t)
	addr := filepath.Join(d, "bus.sock")
	ln, err := listenTestUnix(t, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	sv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { io.WriteString(w, `{"you":"person"}`) })}
	go sv.Serve(ln)
	defer sv.Close()
	r := &runner{ctx: context.Background(), say: io.Discard, addr: addr, bus: newClient(addr), host: t.TempDir(), agents: t.TempDir(), served: make(map[string]*served)}
	r.probe.Do(func() {})
	r.installed = []installed{{Key: "fixture", Dir: "fixture", Agent: agentFile{Command: "true", Serves: "itself"}, Span: span{Min: 1, Max: 1}}}
	r.mu.Lock()
	r.serve(r.ctx)
	r.mu.Unlock()
	if !strings.Contains(r.installed[0].Refused, "account socket") {
		t.Fatalf("sandboxed kept agent was not refused for account socket: %q", r.installed[0].Refused)
	}
}
func TestRunnerRefusesEnabledAncestorAndDescendantWorkKeys(t *testing.T) {
	r := &runner{person: "person", ctx: context.Background(), say: io.Discard, host: t.TempDir(), agents: t.TempDir(), served: make(map[string]*served)}
	for _, key := range []string{"a", "a/b"} {
		r.installed = append(r.installed, installed{Key: key, Dir: "fixture", Agent: agentFile{Command: "true", Serves: "itself"}, Host: hostEntry{Sandbox: "off"}, Span: span{Min: 1, Max: 1}})
	}
	r.bus = &client{base: "http://fixture", http: &http.Client{Transport: secretTransport(func(q *http.Request) (*http.Response, error) {
		body := `{}`
		if q.URL.Path == "/status" {
			body = fmt.Sprintf(`{"you":%q}`, q.Header.Get("X-Agent-Bus-Token"))
		}
		if q.URL.Path == "/lookup" {
			body = `{"owner":"person"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	for _, in := range r.installed {
		os.MkdirAll(filepath.Dir(r.tokenFile(r.agentName(in))), 0700)
		os.WriteFile(r.tokenFile(r.agentName(in)), []byte(r.agentName(in)), 0600)
	}
	t.Cleanup(r.stopKept)
	r.mu.Lock()
	r.serve(r.ctx)
	r.mu.Unlock()
	for _, in := range r.installed {
		if !strings.Contains(in.Refused, "work directory overlaps") {
			t.Errorf("%s not refused for overlapping work: %q", in.Key, in.Refused)
		}
	}
}
func TestAdoptionRefusesSymlinkWorkDirectoryOrParent(t *testing.T) {
	for _, key := range []string{"a", "a/b"} {
		t.Run(key, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			r := &runner{ctx: ctx, say: io.Discard, host: t.TempDir()}
			root := filepath.Join(r.host, "work")
			os.Mkdir(root, 0700)
			os.Symlink(t.TempDir(), filepath.Join(root, "a"))
			s, err := r.adopt(ctx, installed{Key: key, Dir: "fixture", Agent: agentFile{Serves: "itself", Command: "true"}}, "#fixture@h", &client{}, nil, sandbox.Off{}, &jail{})
			if err == nil {
				s.kept.stop()
				t.Fatal("symlink work directory was adopted")
			}
			if !strings.Contains(err.Error(), "symlink") {
				t.Fatal(err)
			}
		})
	}
}
func TestKeptRestartChecksDiskLimitAndRefusesFurtherStarts(t *testing.T) {
	first, most := backoffFirst, backoffMost
	backoffFirst = 5 * time.Millisecond
	backoffMost = 10 * time.Millisecond
	defer func() { backoffFirst, backoffMost = first, most }()
	r := &runner{ctx: context.Background(), say: io.Discard, served: make(map[string]*served)}
	dir := t.TempDir()
	in := installed{Key: "fixture", Agent: agentFile{Serves: "itself", Deadline: "100ms"}, Span: span{Min: 1, Max: 1}, Host: hostEntry{Name: "#fixture@h"}}
	r.installed = []installed{in}
	s := &served{bus: unconfiguredClient(), in: in, name: "#fixture@h", cmd: "sh -c 'echo start >> starts; printf 1234567890 > data; exit 1'", work: dir, dir: dir, box: sandbox.Off{}, jail: &jail{disk: 1}, cancel: func() {}}
	r.mu.Lock()
	r.served[s.name] = s
	k := r.keep(s, "")
	s.kept = k
	r.mu.Unlock()
	t.Cleanup(k.stop)
	until := time.Now().Add(2 * time.Second)
	var why string
	for time.Now().Before(until) {
		r.mu.Lock()
		why = r.installed[0].Refused
		r.mu.Unlock()
		if why != "" {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !strings.Contains(why, "disk limit") {
		t.Fatalf("kept restart never refused over-limit work: %q", why)
	}
	finished := make(chan struct{})
	go func() { k.wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("over-limit keeper loops did not stop")
	}
	b, err := os.ReadFile(filepath.Join(dir, "starts"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "start") != 1 {
		t.Fatalf("kept agent restarted after disk refusal: %q", b)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.served[s.name] != nil {
		t.Fatal("over-limit agent is still served")
	}
}

func TestKeptSocketRefusesUnexpectedListenersAndDropsToken(t *testing.T) {
	for _, code := range []int{401, 200, 403, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			r := &runner{addr: "/not-a-filename-heuristic", bus: &client{base: "http://fixture", token: "fixture-token", http: &http.Client{Transport: secretTransport(func(q *http.Request) (*http.Response, error) {
				calls++
				if q.Header.Get("X-Agent-Bus-Token") != "" {
					t.Error("listener classification sent the agent token")
				}
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(`{"you":"account"}`)), Header: make(http.Header)}, nil
			})}}}
			why := r.keptSocket(context.Background())
			if (why == "") != (code == 401) {
				t.Fatalf("listener %d refusal %q", code, why)
			}
			if calls != 1 {
				t.Fatalf("%d classification requests", calls)
			}
		})
	}
}
func TestKeptSocketIsClassifiedOncePerReload(t *testing.T) {
	calls := 0
	r := &runner{ctx: context.Background(), addr: "/arbitrary.sock", host: t.TempDir(), agents: t.TempDir(), say: io.Discard, served: make(map[string]*served)}
	r.bus = &client{base: "http://fixture", http: &http.Client{Transport: secretTransport(func(q *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}}
	r.probe.Do(func() {})
	for _, key := range []string{"a", "b"} {
		r.installed = append(r.installed, installed{Key: key, Dir: "fixture", Agent: agentFile{Serves: "itself", Command: "true"}, Span: span{Min: 1, Max: 1}})
	}
	r.mu.Lock()
	r.serve(r.ctx)
	r.mu.Unlock()
	if calls != 1 {
		t.Fatalf("two kept agents caused %d listener probes, want 1", calls)
	}
}
func TestDisabledAncestorRetiresBeforeChildAdoption(t *testing.T) {
	r, s, began, release, called := retiringFixture(t, "#child@h")
	no := false
	r.installed = []installed{s.in, s.in}
	r.installed[0].Host.Enabled = &no
	r.installed[1].Key = "fixture/child"
	r.installed[1].Host.Name = "#child@h"
	done := serveReload(r)
	awaitRetirement(t, began)
	assertStillRetiring(t, done)
	select {
	case <-called:
		t.Error("child contacted the bus before ancestor retired")
	default:
	}
	release()
	awaitRetirement(t, done)
	r.mu.Lock()
	child := r.served["#child@h"]
	r.mu.Unlock()
	if child == nil || child.kept == nil {
		t.Fatal("child was not adopted after disabled ancestor retired")
	}
	r.stopKept()
}
func TestOverlappingCandidatesStopAnAlreadyServedAncestor(t *testing.T) {
	r, s, began, release, _ := retiringFixture(t, "#child@h")
	r.bus.http.Transport = secretTransport(func(q *http.Request) (*http.Response, error) {
		body := `{}`
		if q.URL.Path == "/status" {
			name := "#child@h"
			if q.Header.Get("X-Agent-Bus-Token") == "" {
				name = s.name
			}
			body = fmt.Sprintf(`{"you":%q}`, name)
		}
		if q.URL.Path == "/lookup" {
			body = `{"owner":"person"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	r.installed = []installed{s.in, s.in}
	r.installed[1].Key = "fixture/child"
	r.installed[1].Host.Name = "#child@h"
	done := serveReload(r)
	awaitRetirement(t, done)
	awaitRetirement(t, began)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.served) != 0 {
		t.Fatal("overlapping ancestor remained served")
	}
	for _, in := range r.installed {
		if !strings.Contains(in.Refused, "work directory overlaps") {
			t.Errorf("overlap was not refused for %s: %q", in.Key, in.Refused)
		}
	}
	release()
}

func TestEachStartRefusesAReplacedSymlinkWorkDirectory(t *testing.T) {
	for _, kept := range []bool{false, true} {
		t.Run(fmt.Sprint(kept), func(t *testing.T) {
			r := &runner{ctx: context.Background(), say: io.Discard, host: t.TempDir()}
			root := filepath.Join(r.host, "work")
			os.MkdirAll(root, 0700)
			work := filepath.Join(root, "a")
			os.Mkdir(work, 0700)
			replies := &deadlineReplies{}
			s := &served{in: installed{Key: "a", Agent: agentFile{Deadline: "1s"}}, name: "#a@h", cmd: "printf should-not-run", work: work, dir: work, box: sandbox.Off{}, jail: &jail{}, bus: &client{base: "http://fixture", http: &http.Client{Transport: replies}}}
			os.Remove(work)
			os.Symlink(t.TempDir(), work)
			if kept {
				k := &keeper{r: r, s: s}
				if _, _, err := k.command(); err == nil || !strings.Contains(err.Error(), "symlink") {
					t.Fatalf("kept command accepted symlink work: %v", err)
				}
			} else {
				r.runOne(s, protocol.Envelope{ID: "fixture", From: "caller@h"})
				replies.mu.Lock()
				defer replies.mu.Unlock()
				for _, e := range replies.replies {
					if e.Body != "" {
						t.Fatal("script ran inside replaced symlink work")
					}
				}
			}
		})
	}
}

func TestUnconfiguredTemplateDoesNotConflictWithItsRunnableInstance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &runner{ctx: ctx, realm: "h", person: "person", say: io.Discard, host: t.TempDir(), agents: t.TempDir(), served: make(map[string]*served)}
	r.bus = &client{base: "http://fixture", http: &http.Client{Transport: secretTransport(func(q *http.Request) (*http.Response, error) {
		body := `{}`
		switch q.URL.Path {
		case "/secret":
			body = ""
		case "/status":
			body = fmt.Sprintf(`{"you":%q}`, q.Header.Get("X-Agent-Bus-Token"))
		case "/lookup":
			body = `{"owner":"person"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	if err := os.MkdirAll(filepath.Join(r.agents, "mail"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.agents, "mail", "env.dist"), []byte("FIXTURE_REQUIRED=\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(r.host, "mail", "work"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.host, "mail", "work", "env"), []byte("FIXTURE_REQUIRED=configured\n"), 0600); err != nil {
		t.Fatal(err)
	}
	list := []installed{}
	for _, key := range []string{"mail", "mail/work"} {
		in := installed{Key: key, Dir: "mail", Agent: agentFile{Command: "/bin/sleep 2", Serves: "itself", Deadline: "100ms"}, Host: hostEntry{Sandbox: "off"}, Span: span{Min: 1, Max: 1}}
		// Only runner-executed templates enforce env.dist requirements.
		// The already configured instance serves itself; its metadata does not.
		if key == "mail" {
			in.Agent.Serves = "runner"
		}
		list = append(list, in)
		if err := os.MkdirAll(filepath.Dir(r.tokenFile(r.agentName(in))), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(r.tokenFile(r.agentName(in)), []byte(r.agentName(in)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(r.stopKept)
	var prior *keeper
	for i := 0; i < 2; i++ {
		r.mu.Lock()
		r.installed = append([]installed(nil), list...)
		r.mu.Unlock()
		awaitRetirement(t, serveReload(r))
		r.mu.Lock()
		templateWhy := r.installed[0].Refused
		instanceWhy := r.installed[1].Refused
		s := r.served["#mail/work@h"]
		r.mu.Unlock()
		if !strings.Contains(templateWhy, "a template:") {
			t.Fatalf("template lost its environment refusal: %q", templateWhy)
		}
		if instanceWhy != "" || s == nil || s.kept == nil {
			t.Fatalf("configured instance was refused: %q", instanceWhy)
		}
		if prior != nil && s.kept != prior {
			t.Fatal("invalid template restarted the existing runnable instance")
		}
		prior = s.kept
	}
	r.stopKept()
}
