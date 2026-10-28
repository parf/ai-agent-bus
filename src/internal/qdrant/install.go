// Package qdrant installs only the local, separately keyed Qdrant component.
package qdrant

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/envfile"
)

const Image = "docker.io/qdrant/qdrant:v1.20.0@sha256:9bfc89f9914fc14876a88c05270685921a55b8877b254a98bcc1cc2f0a424cd2"
const name = "agent-bus-qdrant"
const configPath = "/etc/agent-bus/qdrant/config.yaml"
const backupDir = "/var/lib/agent-bus/qdrant.backup"
const unitDir = "/etc/containers/systemd"
const secretName = "agent-bus-qdrant-config"
const label = "io.agent-bus.component"
const generator = "/usr/lib/systemd/system-generators/podman-system-generator"

var hexKey = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Host's injected boundaries keep normal tests away from real host services.
type Host struct {
	Root                 string
	RunnerDir            string
	RunnerUID, RunnerGID int
	Out                  io.Writer
	Run                  func(context.Context, []string, ...string) ([]byte, error)
	Client               *http.Client
	Base                 string
	MaxBytes             int64
	Timeout              time.Duration
	Dial                 func(context.Context, string, string) (net.Conn, error)
	DiskFree             func(string) (uint64, error)
	PortFree             func(int) error
	key                  string
}

func New(runnerDir string, uid, gid int, out io.Writer) *Host {
	return &Host{RunnerDir: runnerDir, RunnerUID: uid, RunnerGID: gid, Out: out, Run: runCommand,
		Client:   &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		PortFree: portFree, Base: "http://127.0.0.1:6333", MaxBytes: 2 << 30, Timeout: 300 * time.Second, DiskFree: freeDisk, Dial: (&net.Dialer{Timeout: 400 * time.Millisecond}).DialContext}
}
func (h *Host) path(p string) string { return filepath.Join(h.Root, p) }
func (h *Host) say(s string)         { fmt.Fprintln(h.Out, s) }
func (h *Host) command(ctx context.Context, args ...string) ([]byte, error) {
	out, err := h.Run(ctx, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("qdrant: %s failed: %w", args[0], err)
	}
	return out, nil
}
func runCommand(ctx context.Context, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var out limitedOutput
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, commandError{exit.ExitCode()}
		}
		return nil, fmt.Errorf("command failed (%T)", err)
	}
	if out.full {
		return nil, errors.New("command output exceeds 256 KiB")
	}
	return out.Bytes(), nil
}

type limitedOutput struct {
	bytes.Buffer
	full bool
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	left := (256 << 10) - b.Len()
	if len(p) > left {
		b.full = true
		p = p[:max(0, left)]
	}
	b.Buffer.Write(p)
	return n, nil
}
func freeDisk(path string) (uint64, error) {
	var st syscall.Statfs_t
	err := syscall.Statfs(path, &st)
	return st.Bavail * uint64(st.Bsize), err
}

func portFree(port int) error {
	l, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("Qdrant port %d is occupied", port)
	}
	return l.Close()
}
func (h *Host) quadlet(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > 64<<10 || st.Mode().Perm() != 0644 || int(st.Sys().(*syscall.Stat_t).Uid) != rootUID(h) {
		return nil, errors.New("refusing unowned or unsafe Quadlet file")
	}
	return io.ReadAll(io.LimitReader(f, 64<<10))
}

func Config(key string) string {
	return "# Managed by agent-bus-setup; private Qdrant configuration.\ntelemetry_disabled: true\nstorage:\n  storage_path: /qdrant/storage\n  snapshots_path: /qdrant/storage/snapshots\n  temp_path: /qdrant/storage/tmp\nservice:\n  host: 127.0.0.1\n  http_port: 6333\n  grpc_port: null\n  api_key: \"" + key + "\"\n"
}
func Container() string {
	return `[Unit]
Description=Local Qdrant for agent-bus
After=network.target
[Container]
Image=` + Image + `
Pull=never
ContainerName=agent-bus-qdrant
Network=host
User=1000
Group=1000
DropCapability=ALL
NoNewPrivileges=true
ReadOnly=true
Tmpfs=/tmp:rw,nosuid,nodev,noexec,size=64m
Volume=agent-bus-qdrant.volume:/qdrant/storage
Secret=agent-bus-qdrant-config,type=mount,uid=1000,gid=1000,mode=0400
Exec=./qdrant --config-path /run/secrets/agent-bus-qdrant-config
Environment=QDRANT_INIT_FILE_PATH=/qdrant/storage/.qdrant-initialized
PidsLimit=512
Label=io.agent-bus.component=qdrant
[Service]
Restart=on-failure
RestartSec=3
TimeoutStartSec=120
TimeoutStopSec=30
MemoryMax=2G
CPUQuota=200%
[Install]
WantedBy=multi-user.target
`
}
func Volume() string {
	return "[Volume]\nVolumeName=agent-bus-qdrant\nUID=1000\nGID=1000\nLabel=io.agent-bus.component=qdrant\n"
}

func (h *Host) Do(verb string, dry bool) (err error) {
	if !strings.Contains("|install|snapshot|check|uninstall|", "|"+verb+"|") {
		return errors.New("--qdrant takes install, snapshot, check or uninstall")
	}
	if dry {
		return h.DryRun(verb)
	}
	lockPath := h.path("/run/agent-bus-qdrant.lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another Qdrant setup operation is running")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	budget := h.Timeout
	if verb == "install" || verb == "check" {
		budget = 3 * h.Timeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	switch verb {
	case "install":
		return h.install(ctx)
	case "uninstall":
		return h.uninstall(ctx)
	}
	if err := h.loadKey(); err != nil {
		return err
	}
	if verb == "check" {
		return h.check(ctx)
	}
	file, err := h.snapshot(ctx)
	if err == nil {
		h.say("snapshot stored: " + file)
	}
	return err
}

// DryRun renders only public units and commands; never reads or creates a key.
func (h *Host) DryRun(verb string) error {
	h.say("Qdrant " + verb + " dry-run: no persistent host changes; no key bytes read or printed")
	h.say("image: " + Image)
	if verb == "install" {
		h.say(Container())
		h.say(Volume())
		h.say("would preserve/create private " + configPath + " and " + filepath.Join(h.RunnerDir, "qdrant/env"))
		h.say("would pull pinned image; create mounted config secret; write owned Quadlets; daemon-reload; start only agent-bus-qdrant.service")
		return h.validateGenerator(context.Background())
	}
	if verb == "check" {
		h.say("would refuse non-fixture collections; restart Qdrant briefly; snapshot synthetic points; import and restore only a fresh labeled fixture volume on 127.0.0.1:16333; clean all own fixtures")
	}
	if verb == "snapshot" {
		h.say(fmt.Sprintf("would checksum and atomically store a snapshot (cap %d bytes, %s); retain 7 owned backups; delete own server snapshot", h.MaxBytes, h.Timeout))
	}
	if verb == "uninstall" {
		h.say("would stop owned Qdrant, remove owned Quadlets, daemon-reload; retain volume, secret, config, runner env and backups")
	}
	return nil
}
func (h *Host) validateGenerator(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	version, err := h.command(ctx, "podman", "--version")
	if err != nil {
		return err
	}
	h.say(strings.TrimSpace(string(version)))
	parentDir := h.path("/run")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(parentDir, "agent-bus-qdrant-validate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	for file, body := range map[string]string{name + ".container": Container(), name + ".volume": Volume()} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0644); err != nil {
			return err
		}
	}
	out, err := h.Run(ctx, []string{"QUADLET_UNIT_DIRS=" + dir}, generator, "--dryrun")
	if err != nil {
		return fmt.Errorf("Quadlet validation failed (no units installed): %w", err)
	}
	lower := strings.ToLower(string(out))
	if strings.Contains(lower, "unsupported") || strings.Contains(lower, "unknown key") || strings.Contains(lower, "failed to") {
		return errors.New("generator rejected a Quadlet key; no units installed")
	}
	if !bytes.Contains(out, []byte(name+".service")) || !bytes.Contains(out, []byte("--network=host")) && !bytes.Contains(out, []byte("--network host")) {
		return errors.New("generator did not produce the expected Qdrant service")
	}
	h.say(string(out))
	return nil
}

func privateFile(path string, uid int) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || int(st.Sys().(*syscall.Stat_t).Uid) != uid {
		return nil, errors.New("private Qdrant file must be regular, owned correctly and mode0600")
	}
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if len(data) > 65536 {
		return nil, errors.New("private Qdrant file exceeds 64 KiB")
	}
	return data, err
}
func privateDir(path string, uid, gid int) error {
	parent, err := os.OpenFile(filepath.Dir(path), os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer parent.Close()
	base := filepath.Base(path)
	created := false
	if err := syscall.Mkdirat(int(parent.Fd()), base, 0700); err == nil {
		created = true
	} else if err != syscall.EEXIST {
		return err
	}
	fd, err := syscall.Openat(int(parent.Fd()), base, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	dir := os.NewFile(uintptr(fd), base)
	defer dir.Close()
	if created {
		if err := dir.Chown(uid, gid); err != nil {
			return err
		}
	}
	st, err := dir.Stat()
	if err != nil {
		return err
	}
	if st.Mode().Perm() != 0700 || int(st.Sys().(*syscall.Stat_t).Uid) != uid {
		return errors.New("private Qdrant directory has unexpected ownership or mode")
	}
	return nil
}
func writeNew(path string, data []byte, uid, gid int, mode os.FileMode) error {
	parent, err := os.OpenFile(filepath.Dir(path), os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer parent.Close()
	base := filepath.Base(path)
	fd, err := syscall.Openat(int(parent.Fd()), base, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, uint32(mode))
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), base)
	ok := false
	defer func() {
		f.Close()
		if !ok {
			syscall.Unlinkat(int(parent.Fd()), base)
		}
	}()
	if err := f.Chown(uid, gid); err != nil {
		return err
	}
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := parent.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}
func rootUID(h *Host) int {
	if h.Root != "" {
		return os.Getuid()
	}
	return 0
}
func rootGID(h *Host) int {
	if h.Root != "" {
		return os.Getgid()
	}
	return 0
}
func (h *Host) loadKey() error {
	data, err := privateFile(h.path(configPath), rootUID(h))
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`(?m)^  api_key: "([0-9a-f]{64})"$`)
	m := re.FindSubmatch(data)
	if len(m) != 2 || string(data) != Config(string(m[1])) {
		return errors.New("Qdrant config conflicts with the managed loopback/gRPC-off config")
	}
	h.key = string(m[1])
	return nil
}
func (h *Host) exists(ctx context.Context, kind, n string) (bool, error) {
	_, err := h.Run(ctx, nil, "podman", kind, "exists", n)
	if err == nil {
		return true, nil
	}
	var code interface{ ExitCode() int }
	if errors.As(err, &code) && code.ExitCode() == 1 {
		return false, nil
	}
	// runCommand intentionally hides stderr but preserves exit codes through a safe wrapper.
	return false, err
}

type commandError struct{ code int }

func (e commandError) Error() string { return "exit status " + strconv.Itoa(e.code) }
func (e commandError) ExitCode() int { return e.code }
func (h *Host) owned(ctx context.Context, kind, n string) error {
	has, err := h.exists(ctx, kind, n)
	if err != nil {
		return err
	}
	if !has {
		return nil
	}
	format := `{{index .Config.Labels "io.agent-bus.component"}}`
	if kind == "volume" {
		format = `{{index .Labels "io.agent-bus.component"}}`
	}
	out, err := h.command(ctx, "podman", kind, "inspect", "--format", format, n)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) != "qdrant" {
		return fmt.Errorf("refusing existing unowned %s %s", kind, n)
	}
	return nil
}
func (h *Host) install(ctx context.Context) error {
	if h.RunnerDir == "" {
		return errors.New("--for must name an existing runner")
	}
	if st, err := os.Lstat(h.RunnerDir); err != nil || !st.IsDir() || int(st.Sys().(*syscall.Stat_t).Uid) != h.RunnerUID {
		return errors.New("runner private home is missing or not runner-owned")
	}
	for _, kind := range []string{"container", "volume"} {
		if err := h.owned(ctx, kind, name); err != nil {
			return err
		}
	}
	has, err := h.exists(ctx, "container", name)
	if err != nil {
		return err
	}
	if !has {
		if err := h.PortFree(6333); err != nil {
			return err
		}
	}
	for suffix, body := range map[string]string{".container": Container(), ".volume": Volume()} {
		data, err := h.quadlet(h.path(unitDir + "/" + name + suffix))
		if err == nil && string(data) != body {
			return errors.New("refusing conflicting Qdrant Quadlet")
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	configExists := true
	if err := h.loadKey(); os.IsNotExist(err) {
		configExists = false
	} else if err != nil {
		return err
	}
	envPath := filepath.Join(h.RunnerDir, "qdrant/env")
	envData, err := privateFile(envPath, h.RunnerUID)
	envExists := !os.IsNotExist(err)
	if err != nil && envExists {
		return err
	}
	if envExists {
		vars, err := envfile.Parse(envData)
		if err != nil {
			return errors.New("Qdrant runner env is invalid")
		}
		found := 0
		for _, v := range vars {
			if v.Key == "QDRANT_API_KEY" {
				found++
				if !hexKey.MatchString(v.Value) || h.key != "" && h.key != v.Value {
					return errors.New("Qdrant key files disagree")
				}
				h.key = v.Value
			}
		}
		if found != 1 {
			return errors.New("Qdrant runner env needs exactly one QDRANT_API_KEY")
		}
	}
	if h.key == "" {
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return err
		}
		h.key = hex.EncodeToString(raw[:])
	}
	sum := sha256.Sum256([]byte(Config(h.key)))
	hash := hex.EncodeToString(sum[:])
	secretExists, err := h.exists(ctx, "secret", secretName)
	if err != nil {
		return err
	}
	if secretExists {
		out, err := h.command(ctx, "podman", "secret", "inspect", secretName)
		if err != nil {
			return err
		}
		var items []struct {
			Spec struct{ Labels map[string]string }
		}
		if json.Unmarshal(out, &items) != nil || len(items) != 1 || items[0].Spec.Labels[label] != "qdrant" || items[0].Spec.Labels["io.agent-bus.config-sha256"] != hash {
			return errors.New("refusing conflicting Qdrant Podman secret")
		}
	}
	if err := h.validateGenerator(ctx); err != nil {
		return err
	}
	if _, err := h.command(ctx, "podman", "pull", Image); err != nil {
		return err
	}
	for _, base := range []string{h.path("/etc/agent-bus"), h.path("/var/lib/agent-bus"), h.path(unitDir)} {
		if err := os.MkdirAll(base, 0755); err != nil {
			return err
		}
	}
	if err := privateDir(h.path("/etc/agent-bus/qdrant"), rootUID(h), rootGID(h)); err != nil {
		return err
	}
	if err := privateDir(h.path(backupDir), rootUID(h), rootGID(h)); err != nil {
		return err
	}
	if err := privateDir(filepath.Dir(envPath), h.RunnerUID, h.RunnerGID); err != nil {
		return err
	}
	if !configExists {
		if err := writeNew(h.path(configPath), []byte(Config(h.key)), rootUID(h), rootGID(h), 0600); err != nil {
			return err
		}
	}
	if !envExists {
		if err := writeNew(envPath, []byte("QDRANT_API_KEY="+h.key+"\n"), h.RunnerUID, h.RunnerGID, 0600); err != nil {
			return err
		}
	}
	if !secretExists {
		if _, err := h.command(ctx, "podman", "secret", "create", "--label", label+"=qdrant", "--label", "io.agent-bus.config-sha256="+hash, secretName, h.path(configPath)); err != nil {
			return err
		}
	}
	for suffix, body := range map[string]string{".container": Container(), ".volume": Volume()} {
		p := h.path(unitDir + "/" + name + suffix)
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			if err := writeNew(p, []byte(body), rootUID(h), rootGID(h), 0644); err != nil {
				return err
			}
		}
	}
	for _, args := range [][]string{{"systemctl", "daemon-reload"}, {"systemctl", "start", name + ".service"}, {"systemctl", "is-active", name + ".service"}} {
		if _, err := h.command(ctx, args...); err != nil {
			return err
		}
	}
	if err := h.ready(ctx, h.Base); err != nil {
		return err
	}
	if err := h.exposure(ctx, name, 6333); err != nil {
		return err
	}
	h.say("Qdrant installed; API key kept private; reboot acceptance still pending")
	return nil
}
func (h *Host) uninstall(ctx context.Context) error {
	for _, kind := range []string{"container", "volume"} {
		if err := h.owned(ctx, kind, name); err != nil {
			return err
		}
	}
	for suffix, body := range map[string]string{".container": Container(), ".volume": Volume()} {
		data, err := h.quadlet(h.path(unitDir + "/" + name + suffix))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if string(data) != body {
			return errors.New("refusing to remove conflicting Quadlet")
		}
	}
	if _, err := h.command(ctx, "systemctl", "stop", name+".service"); err != nil {
		var code interface{ ExitCode() int }
		if !errors.As(err, &code) || code.ExitCode() != 5 {
			return err
		}
	}
	for _, suffix := range []string{".container", ".volume"} {
		if err := os.Remove(h.path(unitDir + "/" + name + suffix)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if _, err := h.command(ctx, "systemctl", "daemon-reload"); err != nil {
		return err
	}
	h.say("Qdrant stopped; retained volume, secret, config, runner key and backups")
	return nil
}
