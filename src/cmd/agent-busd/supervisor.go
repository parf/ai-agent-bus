// The supervisor: it opens the listeners, chowns the per-user ones, hands
// them down and keeps the children alive. It holds no store, no queue and no
// body — it is the process that must not die, so it is the one with the least
// to go wrong. See docs/11-processes.md#the-rule.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/store/sqlite"
)

func runSupervisor(c config) {
	// The daemon binds the address it is given, so agents on other hosts
	// can reach it. It speaks plain HTTP: off loopback, tokens and bodies
	// cross that network unencrypted, which is said once at start.
	// See docs/02-access.md#trust-boundary.
	public, err := offLoopback(c.addr)
	if err != nil {
		log.Fatal(err)
	}
	if public {
		log.Printf("listening on %s, which is not loopback: tokens and message bodies cross that network as plain HTTP", c.addr)
	}
	tcp, err := net.Listen("tcp", c.addr)
	if err != nil {
		log.Fatalf("listen %s: %v", c.addr, err)
	}
	// 0711: it holds one socket per account, so everyone must be able to
	// walk through it to their own. Nobody can list it, and each socket is
	// 0600 to its owner. See docs/02-access.md#local-socket.
	dir := filepath.Dir(c.sock)
	if err := os.MkdirAll(dir, 0o711); err != nil {
		log.Fatalf("socket dir: %v", err)
	}
	if err := os.Chmod(dir, 0o711); err != nil {
		log.Fatalf("socket dir: %v", err)
	}
	if err := clearStaleSocket(c.sock); err != nil {
		log.Fatal(err)
	}
	shared, err := net.Listen("unix", c.sock)
	if err != nil {
		log.Fatalf("listen %s: %v", c.sock, err)
	}
	// This listener authenticates every request by token. Local sessions run
	// under other accounts; only their mapped, credential-bearing sockets
	// are private. See docs/02-access.md#local-socket.
	if err := os.Chmod(c.sock, 0o666); err != nil {
		log.Fatalf("chmod %s: %v", c.sock, err)
	}
	me, err := requiredOwner(c.owner)
	if err != nil {
		log.Fatalf("owner: %v", err)
	}
	// Setup flags seed a legacy installation once. After the bus has written
	// the establishment marker, the snapshot is authoritative: otherwise an
	// operator's change would be undone by the unchanged systemd command line.
	explicit, err := supervisorAccounts(c)
	if err != nil {
		log.Fatalf("local accounts: %v", err)
	}
	c.users = explicit
	activeJSON, err := json.Marshal(c.users.mapping())
	if err != nil {
		log.Fatalf("local accounts: %v", err)
	}
	// One socket per local account, so the bus knows who is calling with
	// nothing for anyone to configure. The account running the daemon is a
	// user of it like any other, so it gets one without being asked for.
	// See docs/02-access.md#local-socket.
	c.users.add(ownerAccount(), me)
	keepUserSockets := map[string]bool{}
	for _, u := range c.users.list() {
		keepUserSockets["user-"+u.account+".sock"] = true
	}
	if err := clearRetiredUserSockets(dir, keepUserSockets); err != nil {
		log.Fatalf("retired local account socket: %v", err)
	}
	paths := []string{c.sock}
	names := []string{"tcp", "shared"}
	ls := []net.Listener{tcp, shared}
	for _, u := range c.users.list() {
		path := filepath.Join(dir, "user-"+u.account+".sock")
		l, err := u.listen(path)
		if err != nil {
			log.Fatalf("socket for %s: %v", u.account, err)
		}
		paths = append(paths, path)
		// The daemon account's own socket is the daemon Owner's, whoever
		// that is after a transfer; the bus child resolves it from the store.
		if u.account == ownerAccount() {
			names = append(names, "owner:")
		} else {
			names = append(names, "user:"+u.name.String())
		}
		ls = append(ls, l)
	}
	fds := make([]*os.File, len(ls))
	for i, l := range ls {
		f, err := listenerFile(l)
		if err != nil {
			log.Fatalf("fd for %s: %v", names[i], err)
		}
		fds[i] = f
	}
	log.Printf("agent-busd on http://%s and %s for %s, %d user sockets",
		c.addr, c.sock, me, len(c.users.list()))

	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("this binary: %v", err)
	}
	kids := []*child{{
		what: "bus",
		path: exe,
		args: os.Args[1:],
		env:  []string{roleEnv + "=" + roleBus, fdsEnv + "=" + strings.Join(names, ","), accountsEnv + "=" + string(activeJSON)},
		fds:  fds,
	}}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	var wg sync.WaitGroup
	for _, k := range kids {
		wg.Add(1)
		go func() { defer wg.Done(); k.keepAlive() }()
	}
	<-stop
	for _, k := range kids {
		k.end()
	}
	wg.Wait()
	// The sockets are the supervisor's: it made them, so it takes them away.
	for _, p := range paths {
		os.Remove(p)
	}
	log.Print("stopped")
}

func supervisorAccounts(c config) (accounts, error) {
	if c.create {
		if err := os.MkdirAll(filepath.Dir(c.db), 0o700); err != nil {
			return accounts{}, err
		}
	}
	st, err := sqlite.Open(c.db, c.create)
	if err != nil {
		return accounts{}, err
	}
	snapshot, err := st.Load()
	st.Close() // the bus child takes the database next, exclusively
	if err != nil {
		return accounts{}, err
	}
	if !snapshot.AccountsEstablished && len(snapshot.Accounts) == 0 {
		return editableAccounts(c.users)
	}
	if !snapshot.AccountsEstablished {
		return accounts{}, fmt.Errorf("snapshot has local account mappings without establishment marker")
	}
	var out accounts
	for _, mapping := range snapshot.Accounts {
		if out.seen[mapping.Account] {
			return accounts{}, fmt.Errorf("snapshot repeats local account %q", mapping.Account)
		}
		if err := out.Set(mapping.Account + "=" + mapping.Principal); err != nil {
			return accounts{}, err
		}
	}
	return editableAccounts(out)
}

func editableAccounts(in accounts) (accounts, error) {
	if principal, supplied := in.mapping()[ownerAccount()]; supplied {
		return accounts{}, fmt.Errorf("%s is the daemon account and is mapped implicitly, not to %s",
			ownerAccount(), principal)
	}
	return in, nil
}

// A removed mapping must remove its discoverable socket too. RuntimeDirectory
// normally clears these across a systemd restart, but direct runs and an
// unclean supervisor death do not get to lean on that. Only our exact socket
// namespace is touched; a live or non-socket collision fails closed.
func clearRetiredUserSockets(dir string, keep map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if keep[name] || !strings.HasPrefix(name, "user-") || !strings.HasSuffix(name, ".sock") {
			continue
		}
		if err := clearStaleSocket(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// environ is the child's environment: the supervisor's, and its own on top.
func (k *child) environ(around []string) []string {
	return append(append([]string{}, around...), k.env...)
}

// child is one supervised process. A child dying is normal: it is contained,
// restarted with backoff, and only the supervisor surviving matters.
type child struct {
	what string
	path string
	args []string
	env  []string
	fds  []*os.File

	mu      sync.Mutex
	cmd     *exec.Cmd
	stopped bool
}

const (
	backoffMin  = 100 * time.Millisecond
	backoffMax  = 5 * time.Second
	longEnough  = 10 * time.Second // alive this long and the last death is forgotten
	stopPatient = 6 * time.Second  // the bus dumps on the way out; wait for it
)

// keepAlive starts the child and starts it again when it dies, until end.
func (k *child) keepAlive() {
	// Capabilities are per-thread and inherited across exec through the
	// ambient set. Clearing it on the one thread that forks is what keeps
	// CAP_CHOWN the supervisor's alone.
	// See docs/11-processes.md#why-the-supervisor-holds-cap_chown.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := clearAmbient(); err != nil {
		log.Printf("%s: ambient capabilities stay as they are: %v", k.what, err)
	}
	wait := backoffMin
	for {
		at := time.Now()
		err := k.run()
		if k.done() {
			return
		}
		if time.Since(at) > longEnough {
			wait = backoffMin
		}
		log.Printf("%s died (%v); restarting in %v", k.what, err, wait)
		time.Sleep(wait)
		if k.done() {
			return
		}
		if wait *= 2; wait > backoffMax {
			wait = backoffMax
		}
	}
}

func (k *child) run() error {
	cmd := exec.Command(k.path, k.args...)
	cmd.Env = k.environ(os.Environ())
	cmd.ExtraFiles = k.fds
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	// A supervisor that is killed outright still takes its children with it:
	// otherwise an orphan keeps the listeners and the next start finds the
	// address in use. The forking thread is locked for the child's whole
	// life, which is what makes this reliable in a Go process.
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
	if err := cmd.Start(); err != nil {
		return err
	}
	k.mu.Lock()
	k.cmd = cmd
	stopped := k.stopped
	k.mu.Unlock()
	if stopped { // end() ran while this one was starting
		cmd.Process.Signal(syscall.SIGTERM)
	}
	return cmd.Wait()
}

// end asks the child to stop. Waiting for it is keepAlive's job — two waiters
// on one process race to reap it — so this only sets the deadline after which
// asking becomes telling.
func (k *child) end() {
	k.mu.Lock()
	k.stopped = true
	cmd := k.cmd
	k.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	cmd.Process.Signal(syscall.SIGTERM)
	// Kill after it has been reaped is refused by os.Process itself, so this
	// needs no second look at whether it is still running — which would be a
	// race with the goroutine that waits for it.
	time.AfterFunc(stopPatient, func() {
		if err := cmd.Process.Kill(); err == nil {
			log.Printf("%s did not stop, so it was killed", k.what)
		}
	})
}

func (k *child) done() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.stopped
}

func listenerFile(l net.Listener) (*os.File, error) {
	type filer interface{ File() (*os.File, error) }
	f, ok := l.(filer)
	if !ok {
		return nil, os.ErrInvalid
	}
	return f.File()
}

// Clearing the ambient set costs the supervisor nothing — its own permitted
// set is untouched, so it can still chown a socket on reload — and leaves a
// child with no capability at all.
const (
	prCapAmbient         = 47
	prCapAmbientClearAll = 4
)

func clearAmbient() error {
	if _, _, e := syscall.Syscall6(syscall.SYS_PRCTL, prCapAmbient, prCapAmbientClearAll, 0, 0, 0, 0); e != 0 {
		return e
	}
	return nil
}
