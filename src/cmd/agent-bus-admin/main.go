// agent-bus-admin is what edits the things the agent-busd account owns. It is
// the operator's program: an ordinary user runs agent-bus-token, which is a
// strict subset of this one. See docs/09-setup.md#the-programs.
//
// It runs as that account or not at all — when it is somebody else it re-runs
// itself under sudo rather than explaining how.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

// The account and its home, which this program edits and nothing else does.
// See docs/09-setup.md#the-two-accounts.
const (
	svcAccount = "agent-busd"
	// Somewhere other than the install: a second one, or a test.
	homeEnv = "AGENT_BUS_HOME"
)

const usage = `agent-bus-admin — what the agent-busd account owns

  agent-bus-admin user add <user@realm> <key.pub|-> [--admin]
  agent-bus-admin user list
  agent-bus-admin user remove <user@realm>
  agent-bus-admin token <user@realm> [--rotate]

A key added here reaches one forced command and no shell: agent-bus-token,
or this program with --admin. See docs/09-setup.md#ssh-admin.`

func main() {
	if version.Print() {
		return
	}
	if err := admin(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func admin() error {
	args := os.Args[1:]
	// Over SSH the line named the operator and the request arrived in the
	// environment. A name in argv is the entitlement, not the verb.
	if ssh := os.Getenv("SSH_ORIGINAL_COMMAND"); ssh != "" {
		if len(args) > 0 && !isVerb(args[0]) {
			args = args[1:]
		}
		args = append(args, strings.Fields(ssh)...)
	}
	if len(args) == 0 {
		return fmt.Errorf("%s", usage)
	}
	if err := beTheAccount(); err != nil {
		return err
	}
	switch args[0] {
	case "user":
		return userVerb(args[1:])
	case "token":
		return handOver("agent-bus-token", args[1:])
	default:
		return fmt.Errorf("no such verb %q\n\n%s", args[0], usage)
	}
}

func isVerb(s string) bool { return s == "user" || s == "token" }

// beTheAccount re-runs this program as the account that owns the files, when
// it is not already. Nothing here is root's: the account's own files are the
// account's to edit.
func beTheAccount() error {
	// A home stated in the environment is not the install's, so there is no
	// account to be: whoever runs it is editing files they already own.
	if os.Getenv(homeEnv) != "" {
		return nil
	}
	u, err := user.Lookup(svcAccount)
	if err != nil {
		return fmt.Errorf("no %s account: run agent-bus-setup first", svcAccount)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil || uid == os.Getuid() {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("sudo", append([]string{"-u", svcAccount, self}, os.Args[1:]...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo -u %s %s: %w", svcAccount, filepath.Base(self), err)
	}
	os.Exit(0)
	return nil
}

// handOver runs the smaller program for a verb they share, so there is one
// implementation of it and an operator's line is a superset rather than a
// second path. See docs/09-setup.md#the-programs.
func handOver(what string, args []string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(filepath.Dir(self), what), args...)
	cmd.Env = append(os.Environ(), "SSH_ORIGINAL_COMMAND=")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func userVerb(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", usage)
	}
	switch args[0] {
	case "add":
		return userAdd(args[1:])
	case "list":
		return userList()
	case "remove", "rm", "delete":
		return userRemove(args[1:])
	}
	return fmt.Errorf("no such user verb %q\n\n%s", args[0], usage)
}

func readKey(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func userAdd(args []string) error {
	var name, keyFile string
	admin := false
	for _, a := range args {
		switch {
		case a == "--admin":
			admin = true
		case a == "-" && keyFile == "":
			keyFile = a
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("no such option %q", a)
		case name == "":
			name = a
		case keyFile == "":
			keyFile = a
		default:
			return fmt.Errorf("user add wants a name and one key file")
		}
	}
	if name == "" || keyFile == "" {
		return fmt.Errorf("user add wants a name and one key file\n\n%s", usage)
	}
	n, err := protocol.ParseName(name)
	if err != nil {
		return err
	}
	// `-` is the key on stdin, because whoever holds the file is not always
	// whoever may open it: this program runs as agent-busd, and a key in a
	// person's home is exactly what that account cannot read. Same spelling
	// as `service-template <name> -`.
	raw, err := readKey(keyFile)
	if err != nil {
		return err
	}
	from := keyFile
	if keyFile == "-" {
		from = "the key on stdin"
	}
	key := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(key, "ssh-") && !strings.HasPrefix(key, "ecdsa-") && !strings.HasPrefix(key, "sk-") {
		return fmt.Errorf("%s is not a public key: it starts %.20q", from, key)
	}
	lines, err := keysFile()
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	forced := filepath.Join(filepath.Dir(self), "agent-bus-token")
	if admin {
		forced = self
	}
	// One line per name: adding somebody twice replaces what was there
	// rather than leaving two keys, one of which nobody meant.
	kept := without(lines, n.String())
	kept = append(kept, fmt.Sprintf("restrict,command=%q %s", forced+" "+n.String(), key))
	if err := writeKeys(kept); err != nil {
		return err
	}
	fmt.Printf("%s may now ask for its credential over ssh %s@<host>\n", n, svcAccount)
	return nil
}

func userList() error {
	lines, err := keysFile()
	if err != nil {
		return err
	}
	for _, l := range lines {
		name, what := whose(l)
		if name == "" {
			continue
		}
		fmt.Printf("%s\t%s\n", name, what)
	}
	return nil
}

func userRemove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("user remove wants one name")
	}
	n, err := protocol.ParseName(args[0])
	if err != nil {
		return err
	}
	lines, err := keysFile()
	if err != nil {
		return err
	}
	kept := without(lines, n.String())
	if len(kept) == len(lines) {
		return fmt.Errorf("%s has no key here", n)
	}
	if err := writeKeys(kept); err != nil {
		return err
	}
	fmt.Printf("%s can no longer reach this node over ssh\n", n)
	return nil
}

// whose reads back what userAdd wrote: the name a line is for, and which
// program it reaches.
func whose(line string) (name, program string) {
	i := strings.Index(line, `command="`)
	if i < 0 {
		return "", ""
	}
	rest := line[i+len(`command="`):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return "", ""
	}
	fields := strings.Fields(rest[:j])
	if len(fields) != 2 {
		return "", ""
	}
	return fields[1], filepath.Base(fields[0])
}

func without(lines []string, name string) []string {
	kept := lines[:0:0]
	for _, l := range lines {
		if who, _ := whose(l); who == name {
			continue
		}
		kept = append(kept, l)
	}
	return kept
}

// home is the account's, and the file is sshd's — this program does not
// invent a format of its own for something that has one.
func home() string {
	if h := os.Getenv(homeEnv); h != "" {
		return h
	}
	if u, err := user.Lookup(svcAccount); err == nil && u.HomeDir != "" {
		return u.HomeDir
	}
	return "/var/lib/agent-bus/daemon"
}

func keysPath() string { return filepath.Join(home(), ".ssh", "authorized_keys") }

func keysFile() ([]string, error) {
	f, err := os.Open(keysPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	return lines, s.Err()
}

func writeKeys(lines []string) error {
	dir := filepath.Dir(keysPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// sshd refuses to read either of these if they are looser, and says so
	// only in its own log — so they are set here rather than hoped for.
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	tmp := keysPath() + ".new"
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, keysPath())
}
