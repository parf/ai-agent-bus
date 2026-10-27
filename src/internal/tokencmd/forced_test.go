package tokencmd

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/tlsdir"
)

// Exercise the installed wrapper and the real CLI, not a token-only test binary.
func TestForcedWrapperNeverDispatchesSSHVerbs(t *testing.T) {
	root := t.TempDir()
	runtime := filepath.Join(root, "runtime")
	if err := os.Mkdir(runtime, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(runtime, "agent-bus")
	build := exec.Command("go", "build", "-o", binary, "./cmd/agent-bus")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v %s", err, out)
	}
	wrapper, err := os.ReadFile("../../agent-bus-token")
	if err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(runtime, "agent-bus-token")
	if err := os.WriteFile(installed, wrapper, 0755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(root, "old-forced-command")
	if err := os.Symlink(installed, linked); err != nil {
		t.Fatal(err)
	}
	// A hostile PATH must not select a different CLI.
	marker := filepath.Join(root, "untrusted-cli-ran")
	if err := os.WriteFile(filepath.Join(root, "agent-bus"), []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	certServer := httptest.NewTLSServer(http.NotFoundHandler())
	cert := certServer.Certificate()
	certServer.Close()
	tls := filepath.Join(root, "tls")
	if err := os.Mkdir(tls, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tls, "cert.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	pin := tlsdir.FingerprintDER(cert.Raw)
	var mu sync.Mutex
	var requests []string
	var minted struct {
		Name   string
		Rotate bool
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, r.URL.Path)
		if r.URL.Path != "/token" {
			http.Error(w, "forbidden test route", 403)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&minted); err != nil {
			t.Error(err)
		}
		io.WriteString(w, `{"token":"fixture-token"}`)
	}))
	defer server.Close()
	for _, tc := range []struct {
		command                 string
		ok, fingerprint, rotate bool
	}{
		{"", true, false, false}, {"token", true, false, false}, {"token --rotate", true, false, true}, {"--rotate", true, false, true},
		{"alice@realm", true, false, false}, {"token alice@realm", true, false, false},
		{"token --fingerprint", true, true, false}, {"--fingerprint", true, true, false},
		{"manage #victim@realm --descr changed", false, false, false}, {"kv set #victim@realm key value", false, false, false}, {"run #intruder@realm /bin/true", false, false, false},
		{"manage --fingerprint", false, false, false}, {"kv --fingerprint", false, false, false}, {"run --fingerprint", false, false, false},
		{"token --fingerprint bob@realm", false, false, false}, {"token bob@realm", false, false, false}, {"token bob@realm --rotate", false, false, false},
	} {
		t.Run(tc.command, func(t *testing.T) {
			mu.Lock()
			before := len(requests)
			mu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, linked, "alice@realm")
			env := []string{}
			for _, kv := range os.Environ() {
				if !strings.HasPrefix(kv, "AGENT_BUS_") && !strings.HasPrefix(kv, "SSH_ORIGINAL_COMMAND=") && !strings.HasPrefix(kv, "PATH=") && !strings.HasPrefix(kv, "XDG_STATE_HOME=") {
					env = append(env, kv)
				}
			}
			cmd.Env = append(env, "PATH="+root+":"+os.Getenv("PATH"), "XDG_STATE_HOME="+filepath.Join(root, "state"), "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN=fixture-bus-credential", "AGENT_BUS_TLS_DIR="+tls, "SSH_ORIGINAL_COMMAND="+tc.command)
			out, err := cmd.Output()
			if ctx.Err() != nil {
				t.Fatal("forced command did not terminate")
			}
			mu.Lock()
			count := len(requests) - before
			name, rotate := minted.Name, minted.Rotate
			mu.Unlock()
			if tc.ok {
				if err != nil {
					t.Fatalf("valid forced token request refused: %v", err)
				}
				if tc.fingerprint {
					if strings.TrimSpace(string(out)) != pin || count != 0 {
						t.Fatal("wrong fingerprint or daemon call")
					}
				} else {
					if strings.TrimSpace(string(out)) != "fixture-token" || count != 1 || name != "alice@realm" || rotate != tc.rotate {
						t.Fatal("token request lost its entitlement or flags")
					}
				}
			} else if err == nil || count != 0 || len(out) != 0 {
				t.Fatalf("forbidden SSH request ran: exit=%v daemon calls=%d output bytes=%d", err, count, len(out))
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("wrapper selected CLI from PATH")
			}
		})
	}
	t.Run("local address", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "--addr="+server.URL, "token", "alice@realm")
		cmd.Env = append(os.Environ(), "AGENT_BUS_ADDR=/missing-token-test.sock", "AGENT_BUS_TOKEN=fixture-bus-credential", "SSH_ORIGINAL_COMMAND=")
		out, err := cmd.Output()
		if err != nil || strings.TrimSpace(string(out)) != "fixture-token" {
			t.Fatal("token transport did not use the explicit CLI address")
		}
	})

}
