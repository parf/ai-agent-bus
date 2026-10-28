package qdrant

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type fakeHost struct {
	h                                                          *Host
	out                                                        bytes.Buffer
	calls                                                      [][]string
	requests                                                   []string
	dials                                                      []string
	container, volume, secret, fixtureContainer, fixtureVolume bool
	secretHash, run                                            string
	points, restored                                           []any
	collections                                                []string
	data                                                       []byte
	leaked, missingRestore, wrongLabels, failedRun, changedID  bool
}

func response(code int, v any) *http.Response {
	b, _ := json.Marshal(v)
	return &http.Response{StatusCode: code, Body: io.NopCloser(bytes.NewReader(b)), Header: make(http.Header)}
}
func result(v any) *http.Response { return response(200, map[string]any{"result": v}) }
func fixtureHost(t *testing.T) *fakeHost {
	t.Helper()
	f := &fakeHost{}
	root := t.TempDir()
	runner := filepath.Join(root, "var/lib/agent-bus/parf-ab-runner")
	for _, d := range []string{filepath.Join(root, "run"), runner} {
		if e := os.MkdirAll(d, 0700); e != nil {
			t.Fatal(e)
		}
	}
	f.h = New(runner, os.Getuid(), os.Getgid(), &f.out)
	f.h.Root = root
	f.h.Run = f.command
	f.h.PortFree = func(int) error { return nil }
	f.h.DiskFree = func(string) (uint64, error) { return 100 << 30, nil }
	f.h.Dial = func(_ context.Context, _ string, addr string) (net.Conn, error) {
		f.dials = append(f.dials, addr)
		return nil, errors.New("refused")
	}
	f.h.Client = &http.Client{Transport: roundTrip(f.http), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return f
}
func (f *fakeHost) command(_ context.Context, env []string, a ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{}, a...))
	s := strings.Join(a, " ")
	switch {
	case a[0] == "podman" && len(a) > 1 && a[1] == "--version":
		return []byte("podman version 5.8.0"), nil
	case a[0] == generator:
		var dir string
		for _, v := range env {
			if strings.HasPrefix(v, "QUADLET_UNIT_DIRS=") {
				dir = strings.TrimPrefix(v, "QUADLET_UNIT_DIRS=")
			}
		}
		if _, e := os.Stat(filepath.Join(dir, name+".container")); e != nil {
			return nil, e
		}
		return []byte(name + ".service\nExecStart=/usr/bin/podman run --network=host --read-only\n"), nil
	case a[0] == "sysctl":
		return []byte("0\n"), nil
	case a[0] == "ss":
		return []byte("LISTEN 0 100 127.0.0.1:6333 0.0.0.0:* users:((\"qdrant\",pid=12345,fd=1))\nLISTEN 0 100 127.0.0.1:16333 0.0.0.0:* users:((\"qdrant\",pid=12346,fd=2))"), nil
	case a[0] == "ip":
		return []byte(`[{"ifname":"eth0","addr_info":[{"local":"127.0.0.1"},{"local":"203.0.113.2"},{"local":"fd00::1"},{"local":"fe80::1"}]}]`), nil
	case strings.HasPrefix(s, "systemctl "):
		if len(a) > 1 && a[1] == "start" {
			f.container = true
			f.volume = true
		}
		return nil, nil
	case strings.HasPrefix(s, "podman pull "):
		return nil, nil
	case len(a) > 2 && a[2] == "exists":
		exists := false
		switch a[1] {
		case "secret":
			exists = f.secret
		case "container":
			if a[3] == fixture {
				exists = f.fixtureContainer
			} else {
				exists = f.container
			}
		case "volume":
			if a[3] == fixture {
				exists = f.fixtureVolume
			} else {
				exists = f.volume
			}
		}
		if exists {
			return nil, nil
		}
		return nil, commandError{1}
	case strings.HasPrefix(s, "podman secret inspect"):
		b, _ := json.Marshal([]any{map[string]any{"Spec": map[string]any{"Labels": map[string]string{label: "qdrant", "io.agent-bus.config-sha256": f.secretHash}}}})
		return b, nil
	case strings.HasPrefix(s, "podman secret create"):
		f.secret = true
		data, e := os.ReadFile(f.h.path(configPath))
		if e != nil {
			return nil, e
		}
		sum := sha256.Sum256(data)
		f.secretHash = hex.EncodeToString(sum[:])
		return []byte("secret-id"), nil
	case strings.HasPrefix(s, "podman container inspect") || strings.HasPrefix(s, "podman volume inspect"):
		if strings.Contains(s, "--format") {
			format := a[4]
			if format == "{{.State.Pid}}" {
				if a[len(a)-1] == fixture {
					return []byte("12346"), nil
				}
				return []byte("12345"), nil
			}
			if strings.Contains(format, "agent-bus.fixture") {
				run := f.run
				if f.wrongLabels {
					run = "somebody-else"
				}
				id := fixture
				if a[1] == "container" {
					id = strings.Repeat("a", 64)
					if f.changedID {
						id = strings.Repeat("b", 64)
					}
				}
				return []byte(id + " r3 " + run), nil
			}
			return []byte("qdrant"), nil
		}
		if f.leaked {
			return []byte(f.h.key), nil
		}
		return []byte(`{"Config":{"Env":["QDRANT_INIT_FILE_PATH=/qdrant/storage/.qdrant-initialized"]}}`), nil
	case strings.HasPrefix(s, "podman volume create"):
		f.fixtureVolume = true
		for _, v := range a {
			if strings.HasPrefix(v, "agent-bus.run=") {
				f.run = strings.TrimPrefix(v, "agent-bus.run=")
			}
		}
		return []byte(fixture), nil
	case strings.HasPrefix(s, "podman volume import"):
		file, e := os.Open(a[4])
		if e != nil {
			return nil, e
		}
		defer file.Close()
		tr := tar.NewReader(file)
		hdr, e := tr.Next()
		if e != nil {
			return nil, e
		}
		if hdr.Name != "restore/r3-acceptance.snapshot" || hdr.Uid != 1000 || hdr.Gid != 1000 || hdr.Mode != 0400 {
			return nil, errors.New("unsafe imported snapshot metadata")
		}
		_, e = io.ReadAll(tr)
		return nil, e
	case strings.HasPrefix(s, "podman run "):
		f.fixtureContainer = true
		if !f.missingRestore {
			f.restored = append([]any{}, f.points...)
		}
		if f.failedRun {
			return nil, commandError{125}
		}
		return []byte(strings.Repeat("a", 64)), nil
	case strings.HasPrefix(s, "podman exec "):
		return []byte("1000:1000\n"), nil
	case strings.HasPrefix(s, "podman rm "):
		f.fixtureContainer = false
		return nil, nil
	case strings.HasPrefix(s, "podman volume rm "):
		f.fixtureVolume = false
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected fake command %q", a)
}
func (f *fakeHost) http(r *http.Request) (*http.Response, error) {
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	if r.Header.Get("api-key") != f.h.key || r.Header.Get("api-key") == "" {
		return response(403, map[string]string{"error": "no key"}), nil
	}
	fixtureRequest := r.URL.Port() == "16333"
	switch {
	case r.URL.Path == "/collections":
		var rows []any
		for _, n := range f.collections {
			rows = append(rows, map[string]string{"name": n})
		}
		return result(map[string]any{"collections": rows}), nil
	case r.URL.Path == "/collections/"+collection && r.Method == "PUT":
		f.collections = []string{collection}
		return result(true), nil
	case r.URL.Path == "/collections/"+collection && r.Method == "DELETE":
		f.collections = nil
		return result(true), nil
	case r.URL.Path == "/collections/"+collection+"/points" && r.Method == "PUT":
		var in struct {
			Points []any `json:"points"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		f.points = in.Points
		return result(true), nil
	case r.URL.Path == "/collections/"+collection+"/points":
		if fixtureRequest {
			return result(f.restored), nil
		}
		return result(f.points), nil
	case strings.HasSuffix(r.URL.Path, "/points/count"):
		points := f.points
		if fixtureRequest {
			points = f.restored
		}
		return result(map[string]int{"count": len(points)}), nil
	case r.URL.Path == "/snapshots" && r.Method == "POST":
		f.data = fullTar(tarFixtureManifest())
		sum := sha256.Sum256(f.data)
		return result(snapshotInfo{Name: "fixture.snapshot", Size: int64(len(f.data)), Checksum: hex.EncodeToString(sum[:])}), nil
	case r.URL.Path == "/collections/"+collection+"/snapshots" && r.Method == "GET":
		return result([]snapshotInfo{{Name: "child.snapshot"}}), nil
	case r.URL.Path == "/collections/"+collection+"/snapshots/child.snapshot" && r.Method == "DELETE":
		return result(true), nil
	case r.URL.Path == "/snapshots/fixture.snapshot" && r.Method == "GET":
		return &http.Response{StatusCode: 200, ContentLength: int64(len(f.data)), Body: io.NopCloser(bytes.NewReader(f.data)), Header: make(http.Header)}, nil
	case r.URL.Path == "/snapshots/fixture.snapshot" && r.Method == "DELETE":
		return result(true), nil
	}
	return response(404, nil), nil
}
func installFake(t *testing.T, f *fakeHost) {
	t.Helper()
	if e := f.h.install(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestQdrantInstallIsPrivatePinnedAndIdempotent(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	key := f.h.key
	if !hexKey.MatchString(key) {
		t.Fatal("invalid generated key")
	}
	cfg, e := privateFile(f.h.path(configPath), os.Getuid())
	if e != nil {
		t.Fatal(e)
	}
	env, e := privateFile(filepath.Join(f.h.RunnerDir, "qdrant/env"), os.Getuid())
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(cfg), key) || string(env) != "QDRANT_API_KEY="+key+"\n" {
		t.Fatal("container and runner keys differ")
	}
	for _, want := range []string{"@sha256:", "Network=host", "Pull=never", "User=1000", "Group=1000", "DropCapability=ALL", "NoNewPrivileges=true", "ReadOnly=true", "PidsLimit=512", "MemoryMax=2G", "CPUQuota=200%", "WantedBy=multi-user.target", "Volume=agent-bus-qdrant.volume:/qdrant/storage", "type=mount"} {
		if !strings.Contains(Container(), want) {
			t.Fatalf("missing critical Quadlet field %s", want)
		}
	}
	for _, want := range []string{"host: 127.0.0.1", "grpc_port: null", "telemetry_disabled: true"} {
		if !strings.Contains(string(cfg), want) {
			t.Fatal("missing config boundary", want)
		}
	}
	if strings.Contains(f.out.String(), key) {
		t.Fatal("key leaked into setup output")
	}
	for _, c := range f.calls {
		if strings.Contains(strings.Join(c, " "), key) {
			t.Fatal("key leaked into argv")
		}
	}
	if len(f.dials) != 3 || f.dials[2] != "[fe80::1%eth0]:6333" {
		t.Fatalf("IPv4/IPv6 probes skipped: %v", f.dials)
	}
	f.calls = nil
	installFake(t, f)
	if f.h.key != key {
		t.Fatal("reinstall rotated key")
	}
	for _, c := range f.calls {
		if strings.HasPrefix(strings.Join(c, " "), "podman secret create") {
			t.Fatal("reinstall replaced secret")
		}
	}
	if e := f.h.uninstall(context.Background()); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{f.h.path(configPath), filepath.Join(f.h.RunnerDir, "qdrant/env"), f.h.path(backupDir)} {
		if _, e := os.Stat(p); e != nil {
			t.Fatal("uninstall removed key/data path", p)
		}
	}
}
func TestQdrantDryRunDoesNotProvisionKeysOrServices(t *testing.T) {
	f := fixtureHost(t)
	if e := f.h.DryRun("install"); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(f.h.path(configPath)); !os.IsNotExist(e) {
		t.Fatal("dry-run created key")
	}
	for _, a := range f.calls {
		if a[0] != generator && strings.Join(a, " ") != "podman --version" {
			t.Fatal("dry-run executed mutation", a)
		}
	}
}
func TestQdrantRefusesConflictingSecretBeforeWrites(t *testing.T) {
	f := fixtureHost(t)
	f.secret = true
	f.secretHash = "wrong"
	if e := f.h.install(context.Background()); e == nil {
		t.Fatal("adopted conflicting secret")
	}
	if _, e := os.Stat(f.h.path(configPath)); !os.IsNotExist(e) {
		t.Fatal("conflict still wrote a key")
	}
}
func TestQdrantRefusesSymlinkAndPrivateFileMode(t *testing.T) {
	f := fixtureHost(t)
	p := filepath.Join(f.h.Root, "private")
	os.WriteFile(p, []byte("private"), 0644)
	if _, e := privateFile(p, os.Getuid()); e == nil {
		t.Fatal("world-readable key accepted")
	}
	link := p + "-link"
	os.Symlink(p, link)
	if _, e := privateFile(link, os.Getuid()); e == nil {
		t.Fatal("symlink key accepted")
	}
	dir := filepath.Join(f.h.Root, "symlink-dir")
	os.Symlink(f.h.Root, dir)
	if e := writeNew(filepath.Join(dir, "escaped"), []byte("x"), os.Getuid(), os.Getgid(), 0600); e == nil {
		t.Fatal("followed parent symlink on root write")
	}
}
func TestQdrantRejectsUnexpectedListenersAndInspectionLeaks(t *testing.T) {
	for _, local := range []string{"0.0.0.0:6333", "[::]:6333", "127.0.0.1:6334"} {
		if e := validateListeners([]byte("LISTEN 0 10 "+local+" *:* users:((\"qdrant\",pid=1,fd=1))"), "1", "6333"); e == nil {
			t.Fatal("accepted", local)
		}
	}
	f := fixtureHost(t)
	f.leaked = true
	if e := f.h.install(context.Background()); e == nil || !strings.Contains(e.Error(), "inspection") {
		t.Fatal("key leak not refused", e)
	}
	if strings.Contains(f.out.String(), f.h.key) {
		t.Fatal("error printed the key")
	}
}
func TestQdrantCheckRestoresPointsAndCleansEveryOwnedFixture(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	if e := f.h.check(context.Background()); e != nil {
		t.Fatal(e)
	}
	if f.fixtureVolume || f.fixtureContainer || len(f.collections) != 0 {
		t.Fatal("fixtures left behind")
	}
	if !strings.Contains(f.out.String(), "PASS full snapshot recovered") || !strings.Contains(f.out.String(), "PASS imported snapshot ownership") {
		t.Fatal("missing explicit acceptance results")
	}
	for _, entry := range mustReadDir(t, f.h.path(backupDir)) {
		if entry.Name() != journalFile {
			t.Fatal("fixture backup left behind", entry.Name())
		}
	}
	for _, c := range f.calls {
		joined := strings.Join(c, " ")
		if strings.Contains(joined, "/restore:ro") || strings.Contains(joined, ":Z") {
			t.Fatal("backup directory relabeled/bind mounted")
		}
	}
}
func mustReadDir(t *testing.T, p string) []os.DirEntry {
	t.Helper()
	v, e := os.ReadDir(p)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestQdrantCheckRefusesLiveDataAndMissingRestore(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	f.collections = []string{"live-data"}
	f.calls = nil
	if e := f.h.check(context.Background()); e == nil {
		t.Fatal("check accepted live data")
	}
	for _, c := range f.calls {
		if c[0] == "systemctl" || strings.Contains(strings.Join(c, " "), "volume create") {
			t.Fatal("mutated host before live-data refusal")
		}
	}
	f.collections = nil
	f.missingRestore = true
	if e := f.h.check(context.Background()); e == nil {
		t.Fatal("missing restored data accepted")
	}
	if f.fixtureVolume || f.fixtureContainer || len(f.collections) != 0 {
		t.Fatal("failure leaked owned fixtures")
	}
}
func TestQdrantCheckRefusesForeignCleanupLabels(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	f.missingRestore = true
	f.wrongLabels = true
	if e := f.h.check(context.Background()); e == nil || !strings.Contains(e.Error(), "cleanup") {
		t.Fatal(e)
	}
	if !f.fixtureVolume || !f.fixtureContainer {
		t.Fatal("removed resources with foreign labels")
	}
}
func TestQdrantSnapshotChecksCapsChecksumCleanupAndAtomicity(t *testing.T) {
	for _, mode := range []string{"good", "checksum", "oversize", "unsafe", "trickle"} {
		t.Run(mode, func(t *testing.T) {
			f := fixtureHost(t)
			dir := f.h.path(backupDir)
			os.MkdirAll(dir, 0700)
			f.h.key = strings.Repeat("a", 64)
			payload := fullTar(map[string][]byte{"config.json": []byte(`{"collections_mapping":{}}`)})
			deleted := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("api-key") != f.h.key {
					t.Error("missing API key")
				}
				if r.Method == "GET" && r.URL.Path == "/collections" {
					io.WriteString(w, `{"result":{"collections":[]}}`)
					return
				}
				if r.Method == "DELETE" {
					deleted++
					io.WriteString(w, `{"result":true}`)
					return
				}
				if r.Method == "POST" {
					sum := sha256.Sum256(payload)
					hash := hex.EncodeToString(sum[:])
					n := "fixture.snapshot"
					size := int64(len(payload))
					if mode == "checksum" {
						hash = strings.Repeat("0", 64)
					}
					if mode == "oversize" {
						size = 5000
					}
					if mode == "unsafe" {
						n = "../escape.snapshot"
					}
					json.NewEncoder(w).Encode(map[string]any{"result": snapshotInfo{Name: n, Size: size, Checksum: hash}})
					return
				}
				if mode == "trickle" {
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				}
				w.Write(payload)
			}))
			defer server.Close()
			f.h.Base = server.URL
			f.h.Client = New("", 0, 0, io.Discard).Client
			f.h.MaxBytes = 4096
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
			defer cancel()
			file, e := f.h.snapshot(ctx)
			if mode == "good" {
				if e != nil {
					t.Fatal(e)
				}
				data, e := privateFile(file, os.Getuid())
				if e != nil || !bytes.Equal(data, payload) {
					t.Fatal("bad stored snapshot", e)
				}
			} else if e == nil {
				t.Fatal("unsafe snapshot accepted", mode)
			}
			if mode == "oversize" && (e == nil || !strings.Contains(e.Error(), "cap")) {
				t.Fatal("size cap was not the refusal", e)
			}
			if mode != "unsafe" && deleted != 1 {
				t.Fatalf("own server snapshot not cleaned: %d", deleted)
			}
			for _, entry := range mustReadDir(t, dir) {
				if strings.HasPrefix(entry.Name(), ".qdrant-partial-") {
					t.Fatal("partial file remains")
				}
				if mode != "good" && ownedBackup.MatchString(entry.Name()) {
					t.Fatal("failed snapshot looked complete")
				}
			}
		})
	}
}
func TestQdrantSnapshotRetentionDoesNotDeleteUnownedFiles(t *testing.T) {
	f := fixtureHost(t)
	dir := f.h.path(backupDir)
	os.MkdirAll(dir, 0700)
	for i := 1; i <= 9; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("qdrant-202601%02dT000000.000000000Z-00000000.snapshot", i)), []byte("x"), 0600)
	}
	os.WriteFile(filepath.Join(dir, "owner-file"), []byte("keep"), 0600)
	if e := pruneBackups(dir, os.Getuid()); e != nil {
		t.Fatal(e)
	}
	if len(mustReadDir(t, dir)) != 8 {
		t.Fatal("retention did not keep seven plus unrelated file")
	}
	if _, e := os.Stat(filepath.Join(dir, "owner-file")); e != nil {
		t.Fatal(e)
	}
}

func TestQdrantFailedPodmanRunCleansPartialContainer(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	f.failedRun = true
	if err := f.h.check(context.Background()); err == nil {
		t.Fatal("failed run accepted")
	}
	if f.fixtureContainer || f.fixtureVolume || len(f.collections) != 0 {
		t.Fatal("partially-created fixtures left behind")
	}
}
func TestQdrantOccupiedPortAndQuadletSymlinkRefuseBeforeWrite(t *testing.T) {
	f := fixtureHost(t)
	f.h.PortFree = func(int) error { return errors.New("occupied") }
	if err := f.h.install(context.Background()); err == nil {
		t.Fatal("occupied port accepted")
	}
	if _, err := os.Stat(f.h.path(configPath)); !os.IsNotExist(err) {
		t.Fatal("key written before port refusal")
	}
	f.h.PortFree = func(int) error { return nil }
	p := f.h.path(unitDir + "/" + name + ".container")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(f.h.Root, "foreign")
	if err := os.WriteFile(target, []byte(Container()), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
	if err := f.h.install(context.Background()); err == nil {
		t.Fatal("symlink Quadlet accepted")
	}
}

func tarFixtureManifest() map[string][]byte {
	return map[string][]byte{"config.json": []byte(`{"collections_mapping":{"ab_r3_acceptance":"child.snapshot"}}`), "child.snapshot": []byte("synthetic data")}
}
func fullTar(entries map[string][]byte) []byte {
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for n, v := range entries {
		_ = tw.WriteHeader(&tar.Header{Name: n, Mode: 0600, Size: int64(len(v)), Typeflag: tar.TypeReg})
		_, _ = tw.Write(v)
	}
	_ = tw.Close()
	return b.Bytes()
}
func TestQdrantSnapshotManifestRefusesUnlistedOrUnsafeEntries(t *testing.T) {
	for _, extra := range []string{"unlisted.snapshot", "../escape", "/absolute"} {
		t.Run(extra, func(t *testing.T) {
			entries := tarFixtureManifest()
			entries[extra] = []byte("foreign")
			p := filepath.Join(t.TempDir(), "fixture")
			if err := os.WriteFile(p, fullTar(entries), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := snapshotChildren(p, 1<<20); err == nil {
				t.Fatal("unlisted/unsafe snapshot accepted for deletion")
			}
		})
	}
}

func TestQdrantSnapshotCleanupJournalRetriesBeforeNewSnapshot(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	f.h.Client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/collections/") && strings.HasSuffix(r.URL.Path, "/snapshots") {
			return response(503, nil), nil
		}
		return f.http(r)
	})
	file, err := f.h.snapshot(context.Background())
	if err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatal("cleanup failure accepted", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("verified backup lost", err)
	}
	if _, err := privateFile(f.h.path(backupDir+"/"+cleanupFile), os.Getuid()); err != nil {
		t.Fatal("cleanup journal lost", err)
	}
	f.requests = nil
	f.h.Client.Transport = roundTrip(f.http)
	if _, err := f.h.snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	deleteAt, postAt := -1, -1
	for i, r := range f.requests {
		if r == "DELETE /snapshots/fixture.snapshot" && deleteAt < 0 {
			deleteAt = i
		}
		if r == "POST /snapshots" && postAt < 0 {
			postAt = i
		}
	}
	if deleteAt < 0 || postAt < deleteAt {
		t.Fatal("new snapshot requested before old cleanup", f.requests)
	}
}
func TestQdrantCleanupRefusesReplacedContainerID(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	f.changedID = true
	if err := f.h.check(context.Background()); err == nil || !strings.Contains(err.Error(), "cleanup") {
		t.Fatal("replaced container accepted", err)
	}
	if !f.fixtureContainer {
		t.Fatal("deleted replacement container")
	}
}
func TestQdrantCheckRecoversOnlyJournalMatchedLeftover(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	run := strings.Repeat("a", 32)
	f.collections = []string{collection}
	f.points = fixturePoints(run)
	data, _ := json.Marshal(map[string]string{"run": run})
	if err := writeNew(f.h.path(backupDir+"/"+journalFile), data, os.Getuid(), os.Getgid(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.h.check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.collections) != 0 {
		t.Fatal("fixture remains")
	}
}

func TestQdrantUnverifiedSnapshotBlocksSecondCreation(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	f.h.MaxBytes = 1
	if _, err := f.h.snapshot(context.Background()); err == nil {
		t.Fatal("oversize accepted")
	}
	_, second := f.h.snapshot(context.Background())
	count := 0
	for _, r := range f.requests {
		if r == "POST /snapshots" {
			count++
		}
	}
	if count != 1 {
		t.Fatal("repeated snapshot creates leak children", count)
	}
	if second == nil {
		t.Fatal("unverified retry allowed")
	}
	if _, err := privateFile(f.h.path(backupDir+"/"+unverifiedFile), os.Getuid()); err != nil {
		t.Fatal(err)
	}
}
func TestQdrantCheckDoesNotPruneExistingBackups(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	p := f.h.path(backupDir + "/qdrant-20261028T120000.000000000Z-1234abcd.snapshot")
	if err := os.WriteFile(p, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	if err := f.h.check(context.Background()); err == nil {
		t.Fatal("commissioning accepted existing backups")
	}
	if len(f.calls) != 0 {
		t.Fatal("host modified before backup refusal")
	}
}

func TestQdrantCleanupHasBoundedSixtySecondBudget(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	original := f.h.Run
	seen := false
	f.h.Run = func(ctx context.Context, env []string, a ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(a, " "), "podman rm ") {
			seen = true
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) < 55*time.Second || time.Until(deadline) > 61*time.Second {
				t.Error("fixture cleanup budget is not bounded 60s")
			}
			if !strings.Contains(strings.Join(a, " "), "--time=2") {
				t.Error("container stop wait not bounded to 2s")
			}
		}
		return original(ctx, env, a...)
	}
	f.h.Client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/snapshots/") {
			deadline, ok := r.Context().Deadline()
			if !ok || time.Until(deadline) < 55*time.Second || time.Until(deadline) > 61*time.Second {
				t.Error("snapshot cleanup budget is not bounded 60s")
			}
		}
		return f.http(r)
	})
	if err := f.h.check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("fixture cleanup not exercised")
	}
}
func TestQdrantReadinessReportsRefusedKey(t *testing.T) {
	f := fixtureHost(t)
	f.h.Client.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return response(403, nil), nil })
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := f.h.ready(ctx, f.h.Base); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatal("refused key reported as generic slow startup", err)
	}
}
func TestQdrantUninstallAcceptsUnloadedUnit(t *testing.T) {
	f := fixtureHost(t)
	installFake(t, f)
	original := f.h.Run
	f.h.Run = func(ctx context.Context, env []string, a ...string) ([]byte, error) {
		if strings.Join(a, " ") == "systemctl stop "+name+".service" {
			return nil, commandError{5}
		}
		return original(ctx, env, a...)
	}
	if err := f.h.uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := f.h.uninstall(context.Background()); err != nil {
		t.Fatal("second uninstall", err)
	}
}
