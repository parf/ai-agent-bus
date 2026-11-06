package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Exercise the fixture's actual startup and shutdown, not a second store wiring.
func TestFixtureUsesFileAndMemoryBigfilesAndCleansUp(t *testing.T) {
	if os.Getenv("C4_FIXTURE_CHILD") == "1" {
		os.Args = os.Args[:1]
		main()
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestFixtureUsesFileAndMemoryBigfilesAndCleansUp$")
	cmd.Env = append(os.Environ(), "C4_FIXTURE_CHILD=1", "TMPDIR="+work)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		in.Close()
		if !waited {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	var ready struct {
		Addr   string
		Tokens map[string]string
	}
	if err := json.NewDecoder(out).Decode(&ready); err != nil {
		t.Fatalf("fixture startup: %v", err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	for _, mode := range []string{"default", "file", "memory"} {
		held := mode
		if mode == "default" {
			held = "file"
		}
		q := url.Values{"record": {"jobs@fixture"}, "name": {mode}, "memory": {"false"}}
		if mode == "default" {
			q.Del("memory")
		}
		if held == "memory" {
			q.Set("memory", "true")
		}
		req, err := http.NewRequestWithContext(ctx, "PUT", ready.Addr+"/bigfile?"+q.Encode(), strings.NewReader("fixture bytes"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Agent-Bus-Token", ready.Tokens["owner@fixture"])
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var info struct {
			Held string
			Size int64
		}
		err = json.NewDecoder(resp.Body).Decode(&info)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK || info.Held != held || info.Size != 13 {
			t.Fatalf("%s put: status %d, held %q, size %d, decode %v", held, resp.StatusCode, info.Held, info.Size, err)
		}
		q.Del("memory")
		req, err = http.NewRequestWithContext(ctx, "GET", ready.Addr+"/bigfile?"+q.Encode(), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Agent-Bus-Token", ready.Tokens["owner@fixture"])
		resp, err = client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK || string(data) != "fixture bytes" || resp.Header.Get("X-Bigfile-Held") != held {
			t.Fatalf("%s get: status %d, data %q, held %q, read %v", held, resp.StatusCode, data, resp.Header.Get("X-Bigfile-Held"), err)
		}
	}
	in.Close()
	err = cmd.Wait()
	waited = true
	if err != nil {
		t.Fatalf("fixture exit: %v", err)
	}
	entries, err := os.ReadDir(work)
	if err != nil || len(entries) != 0 {
		t.Fatalf("fixture left temporary bigfiles: %v, %v", entries, err)
	}
}

// A startup error must unwind run's temporary store before main exits.
func TestFixtureFailedStartupCleansUpBigfiles(t *testing.T) {
	if os.Getenv("C4_FIXTURE_FAILED_CHILD") == "1" {
		os.Args = []string{os.Args[0], os.Getenv("C4_FIXTURE_BAD_SOCKET")}
		main()
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	// A regular file cannot be the parent of the requested Unix socket.
	blocker := work + "/not-a-directory"
	if err := os.WriteFile(blocker, []byte("block"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestFixtureFailedStartupCleansUpBigfiles$")
	cmd.Env = append(os.Environ(), "C4_FIXTURE_FAILED_CHILD=1", "C4_FIXTURE_BAD_SOCKET="+blocker+"/fixture.sock", "TMPDIR="+work)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("fixture startup failure did not return before its deadline")
	}
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("unusable socket must fail startup with exit 1: %v", err)
	}
	if !bytes.Contains(out, []byte(blocker+"/fixture.sock")) || !bytes.Contains(out, []byte("not a directory")) {
		t.Fatalf("startup refusal must name the unusable socket: %s", out)
	}
	entries, err := os.ReadDir(work)
	if err != nil || len(entries) != 1 || entries[0].Name() != "not-a-directory" {
		t.Fatalf("failed startup left temporary bigfiles: %v, %v", entries, err)
	}
}
