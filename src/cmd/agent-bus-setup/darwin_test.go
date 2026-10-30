package main

import (
	"encoding/xml"
	"flag"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// fakeDarwin records the system commands the macOS setup would run, in a
// layout under a test directory.
func fakeDarwin(t *testing.T) (darwinLayout, *[]string) {
	t.Helper()
	root := t.TempDir()
	l := darwinLayout{
		account: "_agentbusd-test",
		home:    filepath.Join(root, "var/agent-bus/daemon"),
		run:     filepath.Join(root, "var/run/agent-bus"),
		plist:   filepath.Join(root, "agent-busd.plist"),
		label:   "agent-busd",
	}
	var ran []string
	saved := darwinCommand
	darwinCommand = func(stdin []byte, name string, args ...string) error {
		ran = append(ran, name+" "+strings.Join(args, " "))
		return nil
	}
	t.Cleanup(func() { darwinCommand = saved })
	return l, &ran
}

// The launchd job runs the daemon as its own account, never root, on the
// layout's database and socket; it is a well-formed property list.
func TestDarwinJobRunsTheDaemonAsItsAccount(t *testing.T) {
	l, _ := fakeDarwin(t)
	job := l.launchdJob("/usr/local/lib/agent-bus/current/agent-busd", "127.0.0.1:6767", "alice", "/etc/agent-bus/tls")
	if err := xml.Unmarshal([]byte(job), new(struct{})); err != nil {
		t.Fatalf("the job is not well-formed XML: %v\n%s", err, job)
	}
	for _, want := range []string{
		"<key>UserName</key>\n\t<string>" + l.account + "</string>",
		"<key>GroupName</key>\n\t<string>" + l.account + "</string>",
		"<string>" + filepath.Join(l.home, "agent-bus.db") + "</string>",
		"<string>" + filepath.Join(l.run, "bus.sock") + "</string>",
		"<string>--tls-dir</string>\n\t\t<string>/etc/agent-bus/tls</string>",
		"<key>Umask</key>\n\t<integer>63</integer>",
		"<key>KeepAlive</key>\n\t<true/>",
	} {
		if !strings.Contains(job, want) {
			t.Fatalf("the job lacks %q:\n%s", want, job)
		}
	}
	if strings.Contains(job, "<string>root</string>") {
		t.Fatalf("the job names root:\n%s", job)
	}
}

// install makes the home private and the socket directory walkable, both the
// account's; initializes the database as that account; writes the job before
// loading it into launchd.
func TestDarwinInstallKeepsStoragePrivateToTheAccount(t *testing.T) {
	l, ran := fakeDarwin(t)
	// An existing account, so no dscl is run: that is ensureAccount's test.
	u, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	cur := u.Username
	l.account = cur
	job := l.launchdJob("/x/agent-busd", "127.0.0.1:6767", "alice", "")
	// As strict a umask as a root shell may have: the modes are set, not asked.
	defer syscall.Umask(syscall.Umask(0o077))
	if err := l.install("/x/agent-busd", mustName(t, "alice"), &tlsChoice{mode: "off", dir: filepath.Join(t.TempDir(), "tls")}, job); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{l.home: 0o700, filepath.Join(l.home, "logs"): 0o700, l.run: 0o711} {
		st, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != want {
			t.Fatalf("%s is %#o, want %#o", path, st.Mode().Perm(), want)
		}
	}
	// The account is not their owner, so it reaches its home and its sockets
	// only if every directory setup made on the way lets others pass.
	top := filepath.Dir(filepath.Dir(filepath.Dir(l.home)))
	for _, p := range []string{filepath.Join(top, "var"), filepath.Dir(l.home), filepath.Dir(l.run)} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o755 {
			t.Fatalf("%s, made on the way to the account's directories, is %#o: the account cannot pass through it", p, st.Mode().Perm())
		}
	}
	got, _ := os.ReadFile(l.plist)
	if string(got) != job {
		t.Fatalf("the job written is not the job given")
	}
	want := []string{
		"chown " + cur + ":" + cur + " " + l.home,
		"chown " + cur + ":" + cur + " " + filepath.Join(l.home, "logs"),
		"chown " + cur + ":" + cur + " " + l.run,
		"sudo -u " + cur + " /x/agent-busd --owner alice --db " + filepath.Join(l.home, "agent-bus.db") + " --init",
		"launchctl bootout system/agent-busd",
		"launchctl bootstrap system " + l.plist,
	}
	if strings.Join(*ran, "\n") != strings.Join(want, "\n") {
		t.Fatalf("ran:\n%s\nwant:\n%s", strings.Join(*ran, "\n"), strings.Join(want, "\n"))
	}
}

// A missing account is made a hidden role account with no shell or password,
// under one id free as both user and group.
func TestDarwinAccountIsAHiddenRoleAccount(t *testing.T) {
	l, ran := fakeDarwin(t)
	if err := l.ensureAccount(); err != nil {
		t.Fatal(err)
	}
	all := strings.Join(*ran, "\n")
	id := ""
	for _, c := range *ran {
		if f := strings.Fields(c); len(f) == 6 && f[4] == "UniqueID" {
			id = f[5]
		}
	}
	n, err := strconv.Atoi(id)
	if err != nil || n < 200 || n >= 400 {
		t.Fatalf("UniqueID %q is not a role-account id:\n%s", id, all)
	}
	u := "dscl . -create /Users/" + l.account
	for _, want := range []string{
		"dscl . -create /Groups/" + l.account + " PrimaryGroupID " + id,
		u + " PrimaryGroupID " + id,
		u + " UserShell /usr/bin/false",
		u + " Password *",
		u + " IsHidden 1",
		u + " NFSHomeDirectory " + l.home,
	} {
		if !strings.Contains(all, want+"\n") && !strings.HasSuffix(all, want) {
			t.Fatalf("no %q in:\n%s", want, all)
		}
	}
}

// Linux-only options are refused on macOS rather than ignored.
func TestDarwinRefusesLinuxOnlyOptions(t *testing.T) {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.String("owner", "", "")
	fs.String("runner", "", "")
	fs.Bool("upgrade", false, "")
	if err := fs.Parse([]string{"--owner", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := darwinRefusesFlags(fs); err != nil {
		t.Fatalf("--owner refused: %v", err)
	}
	if err := fs.Parse([]string{"--runner", "install", "--upgrade"}); err != nil {
		t.Fatal(err)
	}
	if err := darwinRefusesFlags(fs); err == nil || !strings.Contains(err.Error(), "Linux only: --runner, --upgrade") {
		t.Fatalf("Linux-only options: %v", err)
	}
}

func mustName(t *testing.T, s string) protocol.Name {
	t.Helper()
	n, err := protocol.ParseName(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// The admin helper's Linux account default must not redirect the reduced
// install: its explicit home and socket belong to the new macOS role account.
func TestDarwinFirstUserUsesTheRoleAccountsHomeAndSocket(t *testing.T) {
	l, ran := fakeDarwin(t)
	short, err := os.MkdirTemp("../../../tmp", "m2-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(short)
	l.run, err = filepath.Abs(short)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", api.UserSocket(l.run, l.account))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	key := filepath.Join(t.TempDir(), "key.pub")
	if err := os.WriteFile(key, []byte("ssh-ed25519 fixture-public-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := l.firstUser("/package/agent-busd", mustName(t, "alice"), key); err != nil {
		t.Fatal(err)
	}
	want := "sudo -u " + l.account + " env AGENT_BUS_HOME=" + l.home + " AGENT_BUS_ADDR=" + api.UserSocket(l.run, l.account) + " /package/agent-bus-admin user add alice - --admin"
	if len(*ran) != 1 || (*ran)[0] != want {
		t.Fatalf("first-user admin must stay in the macOS role account: got %v, want %s", *ran, want)
	}
}
