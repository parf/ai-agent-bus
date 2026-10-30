package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

// secretBus serves /config and /secret for one agent; the secret is read from
// the pointer at every request, and a status other than 200 answers as it.
type secretBus struct {
	secret atomic.Value // string
	status atomic.Int32
	asked  atomic.Int32
	name   string
}

func newSecretBus(t *testing.T, name string) *secretBus {
	b := &secretBus{name: name}
	b.secret.Store("")
	b.status.Store(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/secret":
			if r.URL.Query().Get("name") != b.name {
				t.Errorf("secret asked for %q, not the agent %q", r.URL.Query().Get("name"), b.name)
			}
			b.asked.Add(1)
			if st := int(b.status.Load()); st != 200 {
				w.WriteHeader(st)
				return
			}
			w.Write([]byte(b.secret.Load().(string)))
		case "/config":
			w.Write([]byte("null"))
		default:
			w.Write([]byte(`{"id":"r"}`))
		}
	}))
	t.Cleanup(srv.Close)
	old := transport
	t.Cleanup(func() { transport = old })
	transport = func() (*http.Client, string) { return srv.Client(), srv.URL }
	return b
}

func secretProbe(t *testing.T, box ports.Sandbox) (agent, string, *bytes.Buffer) {
	dir := t.TempDir()
	seen := filepath.Join(dir, "env")
	log := &bytes.Buffer{}
	return agent{Name: "#sec@h", Algo: algoArgs, box: box, work: dir, say: log,
		Script: `env >` + seen + `; echo "leak $DB_PASS" >&2`}, seen, log
}

func TestRunScriptSeesItsBusSecretAtEveryMessage(t *testing.T) {
	bus := newSecretBus(t, "#sec@h")
	t.Setenv("DB_PASS", "from-the-shell")
	t.Setenv("AGENT_BUS_TOKEN", "the-token-sentinel")
	ag, seen, _ := secretProbe(t, sandbox.Off{})

	bus.secret.Store("DB_PASS=first-secret\nOTHER=\"two words\"\nAGENT_BUS_FROM=forged\nAGENT_BUS_TOKEN=forged-token\n")
	handle(ag, protocol.Envelope{ID: "m1", From: "alice@h", To: ag.Name})
	got, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	env := string(got)
	for _, want := range []string{"DB_PASS=first-secret\n", "OTHER=two words\n", "AGENT_BUS_FROM=alice@h\n"} {
		if !strings.Contains(env, want) {
			t.Errorf("script env lacks %q:\n%s", want, env)
		}
	}
	for _, bad := range []string{"from-the-shell", "forged", "the-token-sentinel"} {
		if strings.Contains(env, bad) {
			t.Errorf("script env holds %q:\n%s", bad, env)
		}
	}

	bus.secret.Store("DB_PASS=second-secret\n")
	handle(ag, protocol.Envelope{ID: "m2", From: "alice@h", To: ag.Name})
	got, _ = os.ReadFile(seen)
	if !strings.Contains(string(got), "DB_PASS=second-secret\n") || strings.Contains(string(got), "first-secret") {
		t.Fatalf("a changed secret did not reach the next message:\n%s", got)
	}
}

func TestRunScriptSecretNeverReachesTheLog(t *testing.T) {
	bus := newSecretBus(t, "#sec@h")
	bus.secret.Store("DB_PASS=hunter2-sentinel\n")
	ag, _, log := secretProbe(t, sandbox.Off{})
	handle(ag, protocol.Envelope{ID: "m1", From: "alice@h", To: ag.Name})
	if strings.Contains(log.String(), "hunter2-sentinel") {
		t.Fatalf("the log holds the secret value: %q", log)
	}
	if !strings.Contains(log.String(), "leak ***") {
		t.Fatalf("the script's line was lost, not masked: %q", log)
	}
}

func TestRunScriptWithoutASecretRunsAndAnUnreadableOneDoesNot(t *testing.T) {
	bus := newSecretBus(t, "#sec@h")
	ag, seen, _ := secretProbe(t, sandbox.Off{})
	bus.status.Store(404)
	handle(ag, protocol.Envelope{ID: "m1", From: "alice@h", To: ag.Name})
	if _, err := os.Stat(seen); err != nil {
		t.Fatalf("an agent with no secret did not run: %v", err)
	}
	os.Remove(seen)
	bus.status.Store(500)
	handle(ag, protocol.Envelope{ID: "m2", From: "alice@h", To: ag.Name})
	if _, err := os.Stat(seen); err == nil {
		t.Fatal("the script ran although its secret could not be read")
	}
}

// capture is a sandbox that records the job it was asked to wrap.
type capture struct{ job ports.Job }

func (c *capture) Name() string { return "capture" }
func (c *capture) Wrap(j ports.Job) []string {
	c.job = j
	return j.Argv
}

// A sandboxed child starts from the user manager's environment, so the
// secret must be handed to the sandbox as the job's; and the sandbox's command
// line names the key only.
func TestRunSandboxedJobCarriesTheSecretWithoutItsValueInArgv(t *testing.T) {
	bus := newSecretBus(t, "#sec@h")
	bus.secret.Store("DB_PASS=argv-sentinel\n")
	box := &capture{}
	ag, _, _ := secretProbe(t, box)
	handle(ag, protocol.Envelope{ID: "m1", From: "alice@h", To: ag.Name})
	if len(box.job.Secret) != 1 || box.job.Secret[0] != "DB_PASS=argv-sentinel" {
		t.Fatalf("the sandbox job's secret is %q", box.job.Secret)
	}
	for _, kv := range box.job.Env {
		if strings.Contains(kv, "argv-sentinel") {
			t.Fatalf("the secret is in the job's Env, which a sandbox writes on its command line: %q", kv)
		}
	}
	line := strings.Join(sandbox.SystemdRun{}.Wrap(box.job), " ")
	if strings.Contains(line, "argv-sentinel") || !strings.Contains(line, "--setenv=DB_PASS ") {
		t.Fatalf("sandbox command line: %s", line)
	}
}

// The log is masked line by line, so a value of several lines (a key) is
// masked by each line, and a value holding a shorter secret is masked whole.
func TestRunScriptSecretOfSeveralLinesOrHoldingAnotherNeverReachesTheLog(t *testing.T) {
	bus := newSecretBus(t, "#sec@h")
	bus.secret.Store("SHORT=abcd\nLONG=abcdEFGHsecret\nKEY=\"-----BEGIN KEY-----\\nMIIEsecretbody\\n-----END KEY-----\"\n")
	dir := t.TempDir()
	log := &bytes.Buffer{}
	ag := agent{Name: "#sec@h", Algo: algoArgs, box: sandbox.Off{}, work: dir, say: log,
		Script: `echo "long $LONG" >&2; printf 'key %s\n' "$KEY" >&2`}
	handle(ag, protocol.Envelope{ID: "m1", From: "alice@h", To: ag.Name})
	for _, leak := range []string{"EFGHsecret", "MIIEsecretbody", "BEGIN KEY"} {
		if strings.Contains(log.String(), leak) {
			t.Fatalf("the log shows %q: %q", leak, log)
		}
	}
	if !strings.Contains(log.String(), "long ***") {
		t.Fatalf("the longer value was not masked whole: %q", log)
	}
}
