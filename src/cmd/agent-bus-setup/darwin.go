package main

import (
	"errors"
	"flag"
	"fmt"
	"html"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/tlsdir"
)

// The reduced setup on macOS (docs/07-setup.md#macos): one dedicated account,
// a launchd job for the daemon, its storage private to that account, TLS as
// on Linux. No per-role accounts, no cgroup limits, no web job, no per-user
// sockets: without CAP_CHOWN the daemon cannot hand a socket to another
// account, so people reach it with a token.
type darwinLayout struct {
	account string // the daemon's account, a hidden role account
	home    string // its home: the database and the logs, 0700
	run     string // the sockets, 0711
	plist   string // the launchd job
	label   string
}

var darwinInstall = darwinLayout{
	account: "_agentbusd",
	home:    "/usr/local/var/agent-bus/daemon",
	run:     api.SystemRuntimeDir,
	plist:   "/Library/LaunchDaemons/agent-busd.plist",
	label:   "agent-busd",
}

// darwinCommand runs one system command for the macOS setup; a test records
// them instead.
var darwinCommand = runIn

// The flags the reduced setup takes; anything else is Linux-only.
var darwinFlags = map[string]bool{"owner": true, "addr": true, "exec": true, "key": true, "print-unit": true, "dry-run": true,
	"tls": true, "tls-cert": true, "tls-key": true, "tls-chain": true, "tls-name": true}

func darwinRefusesFlags(fs *flag.FlagSet) error {
	var linux []string
	fs.Visit(func(f *flag.Flag) {
		if !darwinFlags[f.Name] {
			linux = append(linux, "--"+f.Name)
		}
	})
	if len(linux) != 0 {
		return fmt.Errorf("on macOS setup does the reduced install only (docs/07-setup.md#macos); Linux only: %s", strings.Join(linux, ", "))
	}
	return nil
}

// launchdJob is the daemon's job: as its account, never root, kept alive,
// with a umask that keeps what it writes its own.
func (l darwinLayout) launchdJob(exe, addr, owner, tlsDir string) string {
	args := []string{exe, "--addr", addr, "--socket", filepath.Join(l.run, "bus.sock"),
		"--db", filepath.Join(l.home, "agent-bus.db"), "--log-dir", filepath.Join(l.home, "logs"), "--owner", owner}
	if tlsDir != "" {
		args = append(args, "--tls-dir", tlsDir)
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	str := func(k, v string) {
		fmt.Fprintf(&b, "\t<key>%s</key>\n\t<string>%s</string>\n", k, html.EscapeString(v))
	}
	str("Label", l.label)
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	for _, a := range args {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", html.EscapeString(a))
	}
	b.WriteString("\t</array>\n")
	// The account, never root and never whoever ran setup.
	str("UserName", l.account)
	str("GroupName", l.account)
	str("WorkingDirectory", l.home)
	str("StandardErrorPath", filepath.Join(l.home, "logs", "stderr.log"))
	fmt.Fprintf(&b, "\t<key>EnvironmentVariables</key>\n\t<dict>\n\t\t<key>HOME</key>\n\t\t<string>%s</string>\n\t</dict>\n", html.EscapeString(l.home))
	// 077: the database, its logs and its dumps are the account's alone.
	b.WriteString("\t<key>Umask</key>\n\t<integer>63</integer>\n")
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n\t<key>KeepAlive</key>\n\t<true/>\n</dict>\n</plist>\n")
	return b.String()
}

func (l darwinLayout) steps(tlsC *tlsChoice, me protocol.Name, key string, installs bool) []string {
	var steps []string
	if installs {
		steps = append(steps, "install the complete release under "+installRoot)
	}
	steps = append(steps,
		fmt.Sprintf("create the hidden role account %s, with no login shell and no password", l.account),
		fmt.Sprintf("make %s %s's own, 0700: the database and its logs", l.home, l.account),
		fmt.Sprintf("make %s %s's own, 0711: everyone walks through to the shared socket, nobody lists it", l.run, l.account),
		fmt.Sprintf("initialize the database %s as %s", filepath.Join(l.home, "agent-bus.db"), l.account))
	if s := tlsC.step(); s != "" {
		steps = append(steps, s)
	}
	steps = append(steps,
		fmt.Sprintf("write %s and (re)load it into launchd's system domain, then wait until the daemon reports the installed release", l.plist))
	if key != "" {
		steps = append(steps, fmt.Sprintf("make %s the first user, from %s", me, key))
	}
	return steps
}

// darwinSetup is setup on macOS, from the parsed flags.
func darwinSetup(fs *flag.FlagSet, l darwinLayout, owner, addr, exe, key string, printJob, dry bool, tlsC *tlsChoice) error {
	if err := darwinRefusesFlags(fs); err != nil {
		return err
	}
	if err := tlsC.check(); err != nil {
		return err
	}
	me, err := protocol.ParseName(owner)
	if err != nil {
		return fmt.Errorf("--owner: %w", err)
	}
	explicitExe := exe != ""
	if !explicitExe {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		exe = filepath.Join(filepath.Dir(self), "agent-busd")
	}
	if tlsC.mode == "" && !printJob && !dry && terminal() {
		if err := tlsC.ask(os.Stdin, os.Stdout); err != nil {
			return err
		}
	}
	if tlsC.enabled() && tlsC.mode == "" {
		if _, err := tlsdir.Load(tlsC.dir); err != nil {
			return fmt.Errorf("the node's TLS directory %s does not load, and the daemon would refuse to start: %w; fix it, or pass --tls off", tlsC.dir, err)
		}
	}
	tlsDirArg := ""
	if tlsC.enabled() {
		tlsDirArg = tlsC.dir
	}
	if key == "" {
		key = installerKey()
	}
	steps := l.steps(tlsC, me, key, !explicitExe)
	if dry {
		for _, s := range steps {
			fmt.Println("would " + s)
		}
		return nil
	}
	if os.Geteuid() != 0 && !printJob {
		return fmt.Errorf("this needs root once, to %s and %s. Run:\n\n    %s\n\n"+
			"Nothing after this step does: the daemon runs as %s.\n"+
			"Use --dry-run to see the steps, or --print-unit for the launchd job alone",
			steps[0], l.plist, "sudo "+strings.Join(os.Args, " "), l.account)
	}
	if !explicitExe && !printJob {
		self, _ := os.Executable()
		bundle, err := checkBundle(filepath.Dir(self))
		if err != nil {
			return err
		}
		installed, err := installBundle(bundle)
		if err != nil {
			return fmt.Errorf("install package: %w", err)
		}
		exe = filepath.Join(installed, "agent-busd")
	}
	job := l.launchdJob(exe, addr, me.String(), tlsDirArg)
	if printJob {
		fmt.Print(job)
		return nil
	}
	if err := l.install(exe, me, tlsC, job); err != nil {
		return err
	}
	wantVersion, wantBuild, err := programVersion(exe)
	if err != nil {
		return err
	}
	node, err := waitNodeIdentity(addr, wantVersion, wantBuild, 20*time.Second)
	if err != nil {
		return err
	}
	if key != "" {
		if err := l.firstUser(exe, me, key); err != nil {
			return err
		}
	}
	fmt.Printf("agent-busd %s (%s) runs under launchd as %s, owned by %s, state in %s\n",
		node.Version, node.Build, l.account, node.Owner, l.home)
	// No per-user sockets (darwinLayout): a person calls with a token, on the
	// shared socket or the port.
	fmt.Printf("Your token: sudo -u %s %s token %s; then AGENT_BUS_TOKEN=<it> agent-bus status\n",
		l.account, filepath.Join(filepath.Dir(exe), "agent-bus"), me)
	if tlsC.enabled() {
		if cert, err := tlsdir.LoadCert(tlsC.dir); err == nil {
			fmt.Printf("TLS is on for %s beside plain HTTP, certificate %s. A client elsewhere sets:\n    AGENT_BUS_ADDR=https://<this host>:%s AGENT_BUS_TLS_FINGERPRINT=%s\n",
				addr, tlsdir.Fingerprint(cert), portOf(addr), tlsdir.Fingerprint(cert))
		}
	}
	return nil
}

// install makes the account, its directories, the database, TLS and the job,
// and (re)loads the job. Each step is safe to repeat.
func (l darwinLayout) install(exe string, me protocol.Name, tlsC *tlsChoice, job string) error {
	if err := l.ensureAccount(); err != nil {
		return err
	}
	for _, d := range []struct {
		path string
		mode os.FileMode
	}{{l.home, 0o700}, {filepath.Join(l.home, "logs"), 0o700}, {l.run, 0o711}} {
		if err := os.MkdirAll(d.path, d.mode); err != nil {
			return err
		}
		if err := darwinCommand(nil, "chown", l.account+":"+l.account, d.path); err != nil {
			return err
		}
		// MkdirAll honours the umask and skips one that exists: set, not asked.
		if err := os.Chmod(d.path, d.mode); err != nil {
			return err
		}
	}
	db := filepath.Join(l.home, "agent-bus.db")
	if _, err := os.Stat(db); errors.Is(err, os.ErrNotExist) {
		if err := darwinCommand(nil, "sudo", "-u", l.account, exe, "--owner", me.String(), "--db", db, "--init"); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err := tlsC.install(l.account, os.Stderr); err != nil {
		return fmt.Errorf("TLS: %w", err)
	}
	if err := os.WriteFile(l.plist, []byte(job), 0o644); err != nil {
		return err
	}
	// A loaded job keeps its old definition: take it out first. Not loaded
	// is the first install, so that refusal is not an error.
	_ = darwinCommand(nil, "launchctl", "bootout", "system/"+l.label)
	return darwinCommand(nil, "launchctl", "bootstrap", "system", l.plist)
}

// ensureAccount creates the hidden role account and its group, under the
// first id from 200 that neither a user nor a group holds.
func (l darwinLayout) ensureAccount() error {
	if _, err := user.Lookup(l.account); err == nil {
		return nil
	}
	id, err := freeRoleID()
	if err != nil {
		return err
	}
	ids := strconv.Itoa(id)
	group := "/Groups/" + l.account
	u := "/Users/" + l.account
	for _, c := range [][]string{
		{"-create", group},
		{"-create", group, "PrimaryGroupID", ids},
		{"-create", group, "Password", "*"},
		{"-create", u},
		{"-create", u, "UniqueID", ids},
		{"-create", u, "PrimaryGroupID", ids},
		{"-create", u, "UserShell", "/usr/bin/false"},
		{"-create", u, "NFSHomeDirectory", l.home},
		{"-create", u, "RealName", "agent-bus daemon"},
		{"-create", u, "Password", "*"},
		{"-create", u, "IsHidden", "1"},
	} {
		if err := darwinCommand(nil, "dscl", append([]string{"."}, c...)...); err != nil {
			return err
		}
	}
	return nil
}

// freeRoleID is the first id in macOS's role-account range that no user or
// group holds.
func freeRoleID() (int, error) {
	for id := 200; id < 400; id++ {
		s := strconv.Itoa(id)
		_, uErr := user.LookupId(s)
		_, gErr := user.LookupGroupId(s)
		if uErr != nil && gErr != nil {
			return id, nil
		}
	}
	return 0, errors.New("no free id from 200 to 399 for the daemon's account")
}

// firstUser makes the installer the first user, as on Linux: the admin
// program, as the daemon's account, on its own socket.
func (l darwinLayout) firstUser(exe string, me protocol.Name, keyFile string) error {
	sock := api.UserSocket(l.run, l.account)
	if err := waitForSocket(sock, 10*time.Second); err != nil {
		return err
	}
	key, err := os.ReadFile(keyFile)
	if err != nil {
		return err
	}
	admin := filepath.Join(filepath.Dir(exe), "agent-bus-admin")
	// As that account with no token, the admin program finds this socket.
	return darwinCommand(key, "sudo", "-u", l.account, admin, "user", "add", me.String(), "-", "--admin")
}
