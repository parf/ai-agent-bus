package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAlertCommitDaemonFixture(t *testing.T) {
	dir := os.Getenv("AB_ALERT_COMMIT_FIXTURE")
	if dir == "" {
		return
	}
	log.SetOutput(os.Stdout)
	_, err := os.Stat(filepath.Join(dir, "bus.db"))
	runBus(config{owner: "owner@h", db: filepath.Join(dir, "bus.db"), create: os.IsNotExist(err), logDir: filepath.Join(dir, "logs")})
}

// A real daemon process, including its request workers and graceful SIGTERM
// path, restarts against the same SQLite file. This is not an engine-only reopen.
func TestDaemonCleanStopRetainsOverBudgetAlerts(t *testing.T) {
	dir := t.TempDir()
	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	start := func() (string, func()) {
		t.Helper()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		base := "http://" + listener.Addr().String()
		fd, err := listenerFile(listener)
		listener.Close()
		if err != nil {
			t.Fatal(err)
		}
		child := exec.Command(os.Args[0], "-test.run=^TestAlertCommitDaemonFixture$")
		child.Env = append(os.Environ(), "AB_ALERT_COMMIT_FIXTURE="+dir, fdsEnv+"=owner:", accountsEnv+"={}")
		child.ExtraFiles = []*os.File{fd}
		out, err := child.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		child.Stderr = &stderr
		if err = child.Start(); err != nil {
			fd.Close()
			t.Fatal(err)
		}
		fd.Close()
		ready := make(chan bool, 1)
		go func() {
			scanner := bufio.NewScanner(out)
			announced := false
			for scanner.Scan() {
				if !announced && strings.Contains(scanner.Text(), "bus serving ") {
					announced = true
					ready <- true
				}
			}
			if !announced {
				ready <- false
			}
		}()
		done := make(chan error, 1)
		go func() { done <- child.Wait(); close(done) }()
		t.Cleanup(func() { child.Process.Kill(); <-done })
		select {
		case ok := <-ready:
			if !ok {
				t.Fatalf("daemon exited before readiness: %s", stderr.String())
			}
		case <-time.After(5 * time.Second):
			t.Fatal("daemon readiness deadline")
		}
		return base, func() {
			t.Helper()
			if err := child.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("daemon stop: %v: %s", err, stderr.String())
				}
			case <-time.After(5 * time.Second):
				t.Fatal("daemon failed to drain")
			}
		}
	}
	request := func(base, method, path string, in any) []byte {
		t.Helper()
		data, _ := json.Marshal(in)
		req, err := http.NewRequest(method, base+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 {
			t.Fatalf("%s: %d %s", path, res.StatusCode, raw)
		}
		return raw
	}
	call := func(base, body string) string {
		t.Helper()
		request(base, "POST", "/send", map[string]string{"to": "#bus", "body": body, "wait": "5s"})
		var answer struct{ Body string }
		raw := request(base, "GET", "/consume?wait=5s", nil)
		if err := json.Unmarshal(raw, &answer); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(answer.Body, "error:") {
			t.Fatal(answer.Body)
		}
		return answer.Body
	}
	base, stop := start()
	for id := 1; id <= 40; id++ {
		request(base, "POST", "/register", map[string]any{"name": fmt.Sprintf("#alert%d@h", id), "kind": "agent", "allow": []string{"*"}})
	}
	at := time.Now()
	for id := 1; id <= 40; id++ {
		call(base, fmt.Sprintf("write #alert%d@h alert retained-%d", id, id))
	}
	t.Logf("40 logs written in %s (daemon-wide burst 20)", time.Since(at))
	stop()
	base, stop = start()
	for id := 1; id <= 40; id++ {
		raw := call(base, fmt.Sprintf("read #alert%d@h alert", id))
		var got struct {
			Rings map[string][]struct{ Text string }
		}
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Rings["alert"]) != 1 || got.Rings["alert"][0].Text != fmt.Sprintf("retained-%d", id) {
			t.Fatalf("log %d after daemon restart: %s", id, raw)
		}
	}
	stop()
}
