// agent-bus-admin is what edits the things the agent-busd account owns. It is
// the operator's program: an ordinary user runs agent-bus-token, which is a
// strict subset of this one. See docs/09-setup.md#the-programs.
//
// It runs as that account or not at all — when it is somebody else it re-runs
// itself under sudo rather than explaining how.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/core"
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
	// The key is written first because it is the half that can be taken back:
	// a user is never deleted (docs/01-identity.md#user-lifecycle), so the
	// irreversible half goes last and the reversible one is undone when it
	// refuses. Either both, or neither.
	if err := provision(n, admin); err != nil {
		if undo := writeKeys(lines); undo != nil {
			return fmt.Errorf("%w\n\nthe key for %s is written and could not be taken back out (%v): remove it with `agent-bus-admin user remove %s`", err, n, undo, n)
		}
		return err
	}
	fmt.Printf("%s may now ask for its credential over ssh %s@<host>\n", n, svcAccount)
	if admin {
		fmt.Printf("%s is a administrator\n", n)
	}
	return nil
}

// provision makes the daemon know the name, which is what turns the line just
// written into a way in. Strict issuing refuses a name the daemon holds
// nothing for ([Q57](docs/decisions.md#settled)), so the `token` verb the key
// is forced into answers a fresh install with a refusal and nobody can be
// onboarded — writing `authorized_keys` and stopping was never the whole of
// adding somebody.
//
// The daemon has to be up. The alternative is to write the intent down and act
// on it at the next start, which puts onboarding in two places and needs a
// stored form nobody has asked for; a key that works before the name exists is
// the state this closes, so an unreachable daemon refuses rather than half-adds.
// See docs/09-setup.md#ssh-admin.
func provision(n protocol.Name, admin bool) error {
	// Already known is not a failure: adding a second key for somebody who is
	// already here is the same operation as adding their first.
	if _, code, err := call("POST", "/user", map[string]any{"name": n.String(), "create": true}); err != nil {
		return fmt.Errorf("%s is not added: the daemon did not answer (%w).\nStart it and run this again — a key that reaches the token command before the name exists cannot get a credential", n, err)
	} else if code >= 400 && code != http.StatusPreconditionFailed {
		return fmt.Errorf("%s is not added: the daemon refused to create it (%s)", n, http.StatusText(code))
	}
	if !admin {
		return nil
	}
	// Authority is granted only where it was asked for, and it is granted by
	// membership rather than by a field: `Administrator` is derived by the daemon
	// and never accepted as a claim. SetGroup replaces the list, so the
	// current one is read and added to.
	members, err := administrators()
	if err != nil {
		return fmt.Errorf("%s is added but is not a administrator: %w", n, err)
	}
	for _, m := range members {
		if m == n.String() {
			return nil
		}
	}
	if _, code, err := call("POST", "/group", map[string]any{"name": core.AdministratorsGroup, "members": append(members, n.String())}); err != nil {
		return fmt.Errorf("%s is added but is not a administrator: %w", n, err)
	} else if code >= 400 {
		return fmt.Errorf("%s is added but is not a administrator: the daemon refused (%s)", n, http.StatusText(code))
	}
	return nil
}

func administrators() ([]string, error) {
	out, code, err := call("GET", "/groups", nil)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, fmt.Errorf("the daemon refused the group list (%s)", http.StatusText(code))
	}
	var groups map[string][]string
	if err := json.Unmarshal(out, &groups); err != nil {
		return nil, err
	}
	return groups[core.AdministratorsGroup], nil
}

// call reaches the daemon on this account's own socket, which is the
// credential: running as the account that owns the install is what makes this
// the owner's call. See docs/02-access.md#local-socket.
func call(method, path string, body any) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		r = bytes.NewReader(b)
	}
	addr := os.Getenv("AGENT_BUS_ADDR")
	if addr == "" {
		addr = api.ClientSocket()
	}
	client, base := api.Dial(addr)
	req, err := http.NewRequest(method, base+path, r)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	return out, resp.StatusCode, err
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
