package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Actual package, keeper controls, daemon/KV and TLS; no host account, unit or
// cgroup. The independent bwrap backend test proves confined outbox publication.
func TestImapPackagePollsThroughRunnerControlAndVerifiedTLS(t *testing.T) {
	scratch, err := filepath.Abs("../../tmp")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", scratch)
	cert := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer cert.Close()
	socket, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := tls.NewListener(socket, cert.TLS.Clone())
	defer listener.Close()
	var mu sync.Mutex
	var trace []string
	examines, bodies := 0, 0
	raw := "From: sender@example.test\r\nSubject: invoice\r\n\r\n<p>External fixture</p>"
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				fmt.Fprint(conn, "* OK fixture\r\n")
				scan := bufio.NewScanner(conn)
				for scan.Scan() {
					line := scan.Text()
					fields := strings.Fields(line)
					if len(fields) < 2 {
						return
					}
					tag, verb := fields[0], fields[1]
					mu.Lock()
					trace = append(trace, line)
					mu.Unlock()
					switch verb {
					case "CAPABILITY":
						fmt.Fprint(conn, "* CAPABILITY IMAP4rev1\r\n")
					case "LOGIN":
					case "EXAMINE":
						mu.Lock()
						examines++
						n := examines
						mu.Unlock()
						next := 1
						if n > 1 {
							next = 2
						}
						fmt.Fprintf(conn, "* 1 EXISTS\r\n* OK [UIDVALIDITY 7] valid\r\n* OK [UIDNEXT %d] next\r\n", next)
					case "UID":
						if len(fields) < 3 {
							return
						}
						if fields[2] == "SEARCH" {
							fmt.Fprint(conn, "* SEARCH 1\r\n")
						} else if fields[2] == "FETCH" {
							if strings.Contains(line, "RFC822.SIZE") {
								fmt.Fprintf(conn, "* 1 FETCH (UID 1 RFC822.SIZE %d)\r\n", len(raw))
							} else {
								body, section := raw, "BODY[]"
								if strings.Contains(line, "HEADER.FIELDS") {
									body = "From: sender@example.test\r\nSubject: invoice\r\n\r\n"
									section = "BODY[HEADER.FIELDS (FROM SUBJECT)]"
								} else {
									mu.Lock()
									bodies++
									mu.Unlock()
								}
								fmt.Fprintf(conn, "* 1 FETCH (UID 1 %s {%d}\r\n%s)\r\n", section, len(body), body)
							}
						} else {
							fmt.Fprintf(conn, "%s BAD unsupported\r\n", tag)
							continue
						}
					case "LOGOUT":
						fmt.Fprintf(conn, "* BYE done\r\n%s OK done\r\n", tag)
						return
					default:
						fmt.Fprintf(conn, "%s BAD unsupported\r\n", tag)
						continue
					}
					fmt.Fprintf(conn, "%s OK done\r\n", tag)
				}
			}()
		}
	}()
	r, b, script := configurationFixture(t, "itself", "sleep 30")
	b.Persistence(memory.NewState())
	if err := r.control("stop", "fixture"); err != nil {
		t.Fatal(err)
	}
	pack, err := filepath.Abs("../../agents/imap/package.sh")
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(pack, filepath.Dir(script)).CombinedOutput(); err != nil {
		t.Fatalf("package: %v %s", err, out)
	}
	ca := filepath.Join(r.host, "fixture-ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := mustJSON(map[string]any{"host": "127.0.0.1", "port": socket.Addr().(*net.TCPAddr).Port, "cafile": ca})
	if _, err := b.Configure("#fixture@h", "owner@h", cfg); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(r.host, "fixture")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "env"), []byte("IMAP_USERNAME=fixture@example.test\nIMAP_PASSWORD=fixture-private-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "mail@h", Kind: protocol.KindQueue, Owner: "owner@h", Allow: []string{"#fixture@h"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.control("start", "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "sub add mail@h /invoice/"}); err != nil {
		t.Fatal(err)
	}
	answer := configurationReply(t, b)
	if !strings.Contains(answer.Body, "from UID 1") {
		t.Fatalf("add: %q status=%+v", answer.Body, r.status())
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "status"}); err != nil {
		t.Fatal(err)
	}
	if view := configurationReply(t, b); strings.Contains(view.Body, "view_age_s=unknown") || !strings.Contains(view.Body, "subscriptions=1") {
		t.Fatalf("poll did not execute before serving status: %q", view.Body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	delivered, err := b.ConsumeAs(ctx, "owner@h", "mail@h", "", "", false, false)
	if err != nil {
		t.Fatalf("poll: %v status=%+v", err, r.status())
	}
	if delivered.From != "#fixture@h" || !strings.Contains(delivered.Body, "created by owner@h.") || !strings.HasSuffix(delivered.Body, raw) {
		t.Fatalf("forward: %+v", delivered)
	}
	// The next call is read after the forward outcome commit; do not race a
	// successful send's still-uncommitted marker with the restart.
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "sub list"}); err != nil {
		t.Fatal(err)
	}
	if answer := configurationReply(t, b); !strings.Contains(answer.Body, "sent=1") {
		t.Fatalf("confirmation: %q", answer.Body)
	}
	if err := r.control("restart", "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "status"}); err != nil {
		t.Fatal(err)
	}
	if answer := configurationReply(t, b); !strings.Contains(answer.Body, "subscriptions=1") {
		t.Fatalf("restarted poll: %q", answer.Body)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()
	if duplicate, err := b.ConsumeAs(ctx2, "owner@h", "mail@h", "", "", false, false); err == nil {
		t.Fatalf("duplicate: %+v", duplicate)
	}
	mu.Lock()
	defer mu.Unlock()
	if bodies != 1 {
		t.Fatalf("body FETCH=%d, want1", bodies)
	}
	for _, line := range trace {
		verb := strings.Fields(line)[1]
		switch verb {
		case "CAPABILITY", "LOGIN", "EXAMINE", "UID", "LOGOUT":
		default:
			t.Errorf("non-readonly: %s", line)
		}
		if strings.Contains(line, "BODY") && !strings.Contains(line, "BODY.PEEK") {
			t.Errorf("non-PEEK: %s", line)
		}
	}
}
