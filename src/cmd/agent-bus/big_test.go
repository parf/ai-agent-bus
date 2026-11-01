package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func bigHex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// bigDaemon points the CLI at h and counts the requests it receives.
func bigDaemon(t *testing.T, h http.HandlerFunc) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n.Add(1); h(w, r) }))
	t.Cleanup(server.Close)
	old := transport
	t.Cleanup(func() { transport = old })
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	t.Setenv("AGENT_BUS_TOKEN", "t")
	return &n
}

// captureStdout runs f with os.Stdout replaced and returns what it wrote.
func captureStdout(t *testing.T, f func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	e := f()
	os.Stdout = old
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out), e
}

func TestBigTTLReadsHoursMinutesSecondsAndDaysWithinBounds(t *testing.T) {
	for in, want := range map[string]time.Duration{"90m": 90 * time.Minute, "24h": 24 * time.Hour, "30d": 30 * 24 * time.Hour, "1d": 24 * time.Hour, "45s": 45 * time.Second} {
		if got, e := parseBigTTL(in); e != nil || got != want {
			t.Errorf("%s: %v %v", in, got, e)
		}
	}
	for _, in := range []string{"500ms", "999ms", "1ns", "", "0", "0s", "-1h", "31d", "721h", "0d", "-2d", "1.5d", "d", "abc"} {
		if _, e := parseBigTTL(in); e == nil {
			t.Errorf("%q was accepted", in)
		}
	}
}

func TestBigPutRefusesAnEmptyFileAndEmptyStdinTheSameWayBeforeAnyRequest(t *testing.T) {
	n := bigDaemon(t, func(w http.ResponseWriter, r *http.Request) {})
	empty := filepath.Join(t.TempDir(), "empty")
	os.WriteFile(empty, nil, 0o600)
	_, e1 := captureStdout(t, func() error { return bigPutVerb([]string{"#r", "k", empty}) })
	in, _ := os.CreateTemp(t.TempDir(), "stdin")
	old := os.Stdin
	os.Stdin = in
	defer func() { os.Stdin = old }()
	_, e2 := captureStdout(t, func() error { return bigPutVerb([]string{"#r", "k", "-"}) })
	_, e3 := captureStdout(t, func() error { return bigPutVerb([]string{"#r", "k", filepath.Join(t.TempDir(), "missing")}) })
	if e1 == nil || e2 == nil || e1.Error() != e2.Error() || e3 == nil || n.Load() != 0 {
		t.Fatal(e1, e2, e3, n.Load())
	}
}

func TestBigPutSendsTheHashFirstAndPrintsTheStoredValue(t *testing.T) {
	value := []byte("report bytes")
	file := filepath.Join(t.TempDir(), "report")
	os.WriteFile(file, value, 0o600)
	var query, sha string
	var body []byte
	bigDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		query, sha = r.URL.RawQuery, r.Header.Get("X-Big-Sha256")
		body, _ = io.ReadAll(r.Body)
		fmt.Fprintf(w, `{"record":"#r","name":"k","size":%d,"sha256":%q,"held":"memory","expires":"2026-11-02T00:00:00Z","replaced":true}`, len(body), bigHex(body))
	})
	out, e := captureStdout(t, func() error { return bigPutVerb([]string{"#r", "k", file, "--ttl", "2h", "--memory"}) })
	if e != nil || string(body) != string(value) || sha != bigHex(value) {
		t.Fatal(e, string(body), sha)
	}
	for _, want := range []string{"ttl=7200", "memory=true", "name=k"} {
		if !strings.Contains(query, want) {
			t.Fatalf("query %q lacks %q", query, want)
		}
	}
	for _, want := range []string{"12 bytes", bigHex(value), "held in memory", "(replaced)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("%q lacks %q", out, want)
		}
	}
}

func TestBigGetWritesNothingUntilTheValueIsVerified(t *testing.T) {
	value := []byte("the value")
	serve := func(sum string) {
		bigDaemon(t, func(w http.ResponseWriter, r *http.Request) { w.Header().Set("X-Big-Sha256", sum); w.Write(value) })
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "copy")
	serve(bigHex(value))
	if _, e := captureStdout(t, func() error { return bigGetVerb([]string{"#r", "k", "-o", file}) }); e != nil {
		t.Fatal(e)
	}
	if got, _ := os.ReadFile(file); string(got) != string(value) {
		t.Fatalf("%q", got)
	}
	if st, _ := os.Stat(file); st.Mode().Perm() != 0o600 {
		t.Fatal(st.Mode())
	}
	out, e := captureStdout(t, func() error { return bigGetVerb([]string{"#r", "k"}) })
	if e != nil || out != string(value) {
		t.Fatal(e, out)
	}
	serve(strings.Repeat("0", 64))
	bad := filepath.Join(dir, "bad")
	if _, e := captureStdout(t, func() error { return bigGetVerb([]string{"#r", "k", "-o", bad}) }); e == nil || !strings.Contains(e.Error(), "nothing was written") {
		t.Fatal(e)
	}
	out, e = captureStdout(t, func() error { return bigGetVerb([]string{"#r", "k"}) })
	if e == nil || out != "" {
		t.Fatalf("a wrong digest reached stdout: %q %v", out, e)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(left) != 1 || left[0] != file {
		t.Fatalf("stray files after a refused get: %v", left)
	}
}

func TestBigLsAndRmPrintTheDaemonsAnswer(t *testing.T) {
	bigDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/big/list":
			fmt.Fprintf(w, `{"keys":[{"name":"a","size":1,"sha256":"aa","held":"file","expires":%q}]}`, time.Now().Add(time.Hour).Format(time.RFC3339))
		default:
			io.WriteString(w, `{"deleted":false}`)
		}
	})
	out, e := captureStdout(t, func() error { return bigLsVerb([]string{"#r"}) })
	if e != nil || !strings.HasPrefix(out, "a\t1\taa\tfile\t") || !strings.HasSuffix(strings.TrimSpace(out), "s") || strings.Contains(out, "T") {
		t.Fatal(e, out)
	}
	out, e = captureStdout(t, func() error { return bigRmVerb([]string{"#r", "a"}) })
	if e != nil || !strings.Contains(out, "held no big key") {
		t.Fatal(e, out)
	}
}

func TestBigRefusalIsPrintedAsTheDaemonsText(t *testing.T) {
	bigDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		io.WriteString(w, `acl: not a Maintainer`)
	})
	_, e := captureStdout(t, func() error { return bigGetVerb([]string{"#r", "k"}) })
	if e == nil || !strings.Contains(e.Error(), "acl: not a Maintainer") {
		t.Fatal(e)
	}
}

func TestBigPutRefusesOversizedStdinWithoutReadingItAll(t *testing.T) {
	n := bigDaemon(t, func(w http.ResponseWriter, r *http.Request) {})
	r, w, _ := os.Pipe()
	go func() { w.Write(make([]byte, bigMaxBytes+1)); w.Close() }()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	_, e := captureStdout(t, func() error { return bigPutVerb([]string{"#r", "k", "-"}) })
	if e == nil || !strings.Contains(e.Error(), "at most") || n.Load() != 0 {
		t.Fatal(e, n.Load())
	}
	// A reader that never ends is stopped at the cap, not drained.
	p := bigPutPlan{Record: "#r", Name: "k"}
	if _, _, e := bigSource(p, &endless{}); e == nil || !strings.Contains(e.Error(), "at most") {
		t.Fatal(e)
	}
}

type endless struct{ read int }

func (e *endless) Read(p []byte) (int, error) {
	e.read += len(p)
	if e.read > 4*bigMaxBytes {
		return 0, io.ErrUnexpectedEOF
	}
	return len(p), nil
}

func TestBigUnavailableStoreShowsTheDaemonsCauseOnEveryCall(t *testing.T) {
	const cause = "big-key store unavailable: the directory cannot be opened"
	bigDaemon(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503); io.WriteString(w, cause) })
	file := filepath.Join(t.TempDir(), "v")
	os.WriteFile(file, []byte("x"), 0o600)
	for name, run := range map[string]func() error{
		"put": func() error { return bigPutVerb([]string{"#r", "k", file}) },
		"get": func() error { return bigGetVerb([]string{"#r", "k"}) },
		"ls":  func() error { return bigLsVerb([]string{"#r"}) },
		"rm":  func() error { return bigRmVerb([]string{"#r", "k"}) },
	} {
		_, e := captureStdout(t, run)
		if e == nil || !strings.Contains(e.Error(), cause) {
			t.Errorf("%s: %v", name, e)
		}
	}
}

func TestBigPutOfALargeFileReportsAnEarlyRefusalNotAResetConnection(t *testing.T) {
	file := filepath.Join(t.TempDir(), "large")
	f, _ := os.Create(file)
	f.Truncate(24 << 20)
	f.Close()
	for _, status := range []int{403, 413, 503} {
		bigDaemon(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			fmt.Fprintf(w, "refused early: %d", status)
		})
		for try := 0; try < 3; try++ {
			_, e := captureStdout(t, func() error { return bigPutVerb([]string{"#r", "k", file}) })
			if e == nil || !strings.Contains(e.Error(), fmt.Sprintf("refused early: %d", status)) {
				t.Fatalf("%d try %d: %v", status, try, e)
			}
		}
	}
}
