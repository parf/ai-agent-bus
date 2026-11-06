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
		// ls and bigfile ls read -h as the human form (Unix ls -h); --help is help everywhere.
		cases := [][]string{{"-h"}, {"--help"}, {"ls", "--help"}, {"ls", "--all", "--help"}, {"bigfile", "--help"}, {"bigfile", "-h"}, {"bigfile", "ls", "--help"}, {"bigfile", "ls", "rec@h", "--help"}, {"call", "-h"}}
		// --help after a subcommand is help too, for every subcommand a verb's usage names.
		for _, group := range commandGroups {
			for _, command := range group.commands {
				for _, line := range strings.Split(command.synopsis, "\n") {
					words := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "agent-bus "))
					if len(words) > 1 && words[0] == command.name && !strings.HasPrefix(words[1], "<") && !strings.HasPrefix(words[1], "[") && !strings.HasPrefix(words[1], "-") && command.name != "send" && command.name != "call" {
						cases = append(cases, []string{command.name, words[1], "--help"})
					}
				}
			}
		}
		for _, group := range commandGroups {
			for _, command := range group.commands {
				if command.name != "ls" {
					cases = append(cases, []string{command.name, "-h"})
				}
			}
		}
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
	// A listing's -h needs the bus like any listing: a missing session file refuses it.
	for _, args := range [][]string{{"ls", "-h"}, {"bigfile", "ls", "rec@h", "-h"}} {
		out, stderr, err := cliPresentation(t, false, "", "", args...)
		if err == nil || out != "" || strings.Contains(out, "USAGE") || !strings.Contains(stderr, "AGENT_BUS_SESSION_FILE:") {
			t.Errorf("%q must be the listing, refused for the missing session file: %v %q %q", args, err, out, stderr)
		}
	}
	if shortHelp("ls", []string{"-h"}) || shortHelp("bigfile", []string{"ls", "-h"}) || !shortHelp("bigfile", []string{"-h"}) {
		t.Error("only the listings read -h as the human form")
	}
	if shortHelp("send", []string{"#a@h", "--help"}) || shortHelp("call", []string{"#a@h", "--help"}) || !shortHelp("call", []string{"#a@h", "-h"}) {
		t.Error("--help on send and call asks the destination; -h stays local help")
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
		{"pipe -h is the human form", false, true, []string{"-h"}},
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

func TestBigfileLsHumanForm(t *testing.T) {
	server := protocolTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bigfile/list" {
			http.Error(w, "wrong route", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"keys":[{"name":"photo","size":1536,"sha256":"ab12","held":"disk","expires":"2999-01-01T00:00:00Z"}]}`)
	}))
	defer server.Close()
	plain, stderr, err := cliPresentation(t, false, server.URL, "fixture-token", "bigfile", "ls", "rec@h")
	if err != nil || stderr != "" || !strings.HasPrefix(plain, "photo\t1536\tab12\tdisk\t") {
		t.Fatalf("default bigfile ls must stay tab-separated bytes: %v %q %q", err, plain, stderr)
	}
	human, stderr, err := cliPresentation(t, false, server.URL, "fixture-token", "bigfile", "ls", "rec@h", "-h")
	if err != nil || stderr != "" || !strings.HasPrefix(human, "NAME") || !strings.Contains(human, "1.5K") || strings.Contains(human, "\t") || strings.Contains(human, "USAGE") {
		t.Fatalf("bigfile ls -h, piped, must be the human table: %v %q %q", err, human, stderr)
	}
	for size, want := range map[int64]string{0: "0", 1023: "1023", 1024: "1.0K", 10188: "9.9K", 10189: "10K", 12 * 1024: "12K", 1047552: "1023K", 1048064: "1.0M", 1048575: "1.0M", 1536 * 1024: "1.5M", 1<<30 - 1: "1.0G", 5 << 30: "5.0G", 1<<40 - 1: "1.0T", 1<<60 - 1: "1.0E"} {
		if got := humanSize(size); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", size, got, want)
		}
	}
}
