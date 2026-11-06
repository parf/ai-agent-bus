//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// cliPresentation runs the real entry point with isolated identity and bounded
// waits. A pseudo-terminal tests the output fd, not this test runner's console.
func cliPresentation(t *testing.T, tty bool, address, token string, args ...string) (string, string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, args...)...)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "AGENT_BUS_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "AB_CLI_FLAG_HELPER=1", "AB_SESSION_FILE_FIXTURE=1", "GORACE=atexit_sleep_ms=0", "XDG_CACHE_HOME="+t.TempDir(), "XDG_STATE_HOME="+t.TempDir())
	if address == "" {
		cmd.Env = append(cmd.Env, "AGENT_BUS_SESSION_FILE="+filepath.Join(t.TempDir(), "missing.json"))
	} else {
		cmd.Env = append(cmd.Env, "AGENT_BUS_ADDR="+address, "AGENT_BUS_TOKEN="+token)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	if !tty {
		cmd.Stdout = &stdout
		err := cmd.Run()
		if ctx.Err() != nil {
			t.Fatal("CLI did not finish within 5 s")
		}
		return stdout.String(), stderr.String(), err
	}
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	master := os.NewFile(uintptr(fd), "pty")
	defer master.Close()
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout = slave
	if err := cmd.Start(); err != nil {
		slave.Close()
		t.Fatal(err)
	}
	slave.Close()
	out, readErr := io.ReadAll(master)
	err = cmd.Wait()
	if ctx.Err() != nil {
		t.Fatal("terminal CLI did not finish within 5 s")
	}
	if readErr != nil && !errors.Is(readErr, unix.EIO) {
		t.Fatal(readErr)
	}
	return strings.ReplaceAll(string(out), "\r\n", "\n"), stderr.String(), err
}

func TestCLIShortHelpAndTTYListing(t *testing.T) {
	t.Run("short help precedes the session file for every verb", func(t *testing.T) {
		cases := [][]string{{"-h"}, {"--help"}}
		for _, group := range commandGroups {
			for _, command := range group.commands {
				cases = append(cases, []string{command.name, "-h"})
			}
		}
		cases = append(cases, []string{"bigfile", "ls", "-h"})
		for _, args := range cases {
			out, stderr, err := cliPresentation(t, false, "", "", args...)
			if err != nil || stderr != "" || !strings.Contains(out, "USAGE") {
				t.Errorf("%q: help must precede missing session: err=%v stderr=%s out=%.150s", args, err, stderr, out)
			}
		}
	})
	out, stderr, err := cliPresentation(t, false, "", "", "status")
	if err == nil || out != "" || !strings.Contains(stderr, "AGENT_BUS_SESSION_FILE:") {
		t.Fatalf("fixture must preserve the missing session file: %v %q %q", err, out, stderr)
	}
	for _, args := range [][]string{{"--", "-h"}, {"--topic", "-h"}, {"--topic=-h"}} {
		if shortHelp("send", args) {
			t.Errorf("help consumed literal option/body %q", args)
		}
	}
	server := protocolTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ls" {
			t.Errorf("unexpected request %s", r.URL.Path)
			http.Error(w, "wrong route", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"#fixture@h","kind":"agent","owner":"owner@h","readers":1}]`)
	}))
	defer server.Close()
	for _, c := range []struct {
		name       string
		tty, table bool
		flags      []string
	}{
		{"terminal defaults to table", true, true, nil},
		{"pipe defaults to JSON", false, false, nil},
		{"terminal JSON override", true, false, []string{"--json"}},
		{"pipe table override", false, true, []string{"--table"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, stderr, err := cliPresentation(t, c.tty, server.URL, "fixture-token", append([]string{"ls", "--all"}, c.flags...)...)
			if err != nil || stderr != "" {
				t.Fatalf("listing failed: %v %s", err, stderr)
			}
			if c.table {
				if !strings.Contains(out, "NAME") || !strings.Contains(out, "OWNER") || !strings.Contains(out, "#fixture@h") || json.Valid([]byte(out)) {
					t.Fatalf("expected human table, got %q", out)
				}
			} else {
				var rows []struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 || rows[0].Name != "#fixture@h" {
					t.Fatalf("expected JSON records: %v %q", err, out)
				}
			}
		})
	}
}
