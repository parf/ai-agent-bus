package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A fake daemon, because what is under test is what agent-bus-admin does with
// the answer rather than what core decides. It records the calls so a check
// can say the user was created, not merely that nothing failed.
type daemon struct {
	users    []string
	members  []string
	refuse   map[string]int // path -> status
	exists   map[string]bool
	requests []string
	profiles map[string]protocol.User
}

func newDaemon() *daemon {
	return &daemon{members: []string{"owner@h"}, refuse: map[string]int{}, exists: map[string]bool{}, profiles: map[string]protocol.User{}}
}

func (d *daemon) serve(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /user", func(w http.ResponseWriter, r *http.Request) {
		var in protocol.User
		json.NewDecoder(r.Body).Decode(&in)
		d.requests = append(d.requests, "POST /user "+in.Name)
		if code, bad := d.refuse["/user"]; bad {
			w.WriteHeader(code)
			return
		}
		if d.exists[in.Name] {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		d.users = append(d.users, in.Name)
		d.profiles[in.Name] = in
		w.Write([]byte(`{}`))
	})
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		d.requests = append(d.requests, "GET /users")
		users := make([]protocol.User, 0, len(d.profiles))
		for _, profile := range d.profiles {
			users = append(users, profile)
		}
		json.NewEncoder(w).Encode(users)
	})
	mux.HandleFunc("GET /groups", func(w http.ResponseWriter, r *http.Request) {
		d.requests = append(d.requests, "GET /groups")
		if code, bad := d.refuse["/groups"]; bad {
			w.WriteHeader(code)
			return
		}
		json.NewEncoder(w).Encode(map[string][]string{core.AdministratorsGroup: d.members})
	})
	mux.HandleFunc("POST /group", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name    string   `json:"name"`
			Members []string `json:"members"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		d.requests = append(d.requests, "POST /group "+in.Name)
		if code, bad := d.refuse["/group"]; bad {
			w.WriteHeader(code)
			return
		}
		d.members = in.Members
		w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func (d *daemon) knows(name string) bool {
	for _, u := range d.users {
		if u == name {
			return true
		}
	}
	return false
}

func (d *daemon) maintains(name string) bool {
	for _, m := range d.members {
		if m == name {
			return true
		}
	}
	return false
}

// home puts the keys file somewhere this test owns, and a key file to add.
func testHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(homeEnv, dir)
	key := filepath.Join(dir, "k.pub")
	if err := os.WriteFile(key, []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIexample somebody@example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return key
}

func keyed(t *testing.T, name string) bool {
	t.Helper()
	lines, err := keysFile()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		if who, _ := whose(l); who == name {
			return true
		}
	}
	return false
}

// The acceptance itself: on a daemon where the name does not exist, `user add`
// leaves it able to ask for a credential — which means the daemon knows it and
// a forced-command line reaches the token program.
func TestUserAddCreatesTheUserItAdds(t *testing.T) {
	key := testHome(t)
	d := newDaemon()
	t.Setenv("AGENT_BUS_ADDR", d.serve(t))

	if err := userAdd([]string{"newcomer@h", key}); err != nil {
		t.Fatalf("user add refused: %v", err)
	}
	if !d.knows("newcomer@h") {
		t.Fatal("the key was added without the user: the daemon was never asked to create newcomer@h, so its forced command would be refused a credential")
	}
	if !keyed(t, "newcomer@h") {
		t.Fatal("the user was created without the key: nothing in authorized_keys reaches the token command")
	}
	if d.maintains("newcomer@h") {
		t.Fatal("authority nobody granted: newcomer@h became a administrator without --admin")
	}
}

// --admin is the only thing that grants authority, and it grants it by
// membership rather than by asking for a field the daemon would ignore.
func TestAdminFlagGrantsAdministratorAndNothingElseDoes(t *testing.T) {
	key := testHome(t)
	d := newDaemon()
	t.Setenv("AGENT_BUS_ADDR", d.serve(t))

	if err := userAdd([]string{"boss@h", key, "--admin"}); err != nil {
		t.Fatalf("user add --admin refused: %v", err)
	}
	if !d.knows("boss@h") {
		t.Fatal("the key was added without the user")
	}
	if !d.maintains("boss@h") {
		t.Fatal("--admin did not grant administrator authority")
	}
	// The owner must survive being added to, or the daemon would refuse the
	// whole group write and the authority would silently not be granted.
	if !d.maintains("owner@h") {
		t.Fatal("the existing administrators were replaced rather than added to")
	}
}

// The half-add this task closes: a key that works before the name exists. With
// no daemon there is no way to create the name, so nothing is written.
func TestAnUnreachableDaemonAddsNobody(t *testing.T) {
	key := testHome(t)
	t.Setenv("AGENT_BUS_ADDR", "http://127.0.0.1:1")

	err := userAdd([]string{"nobody@h", key})
	if err == nil {
		t.Fatal("user add succeeded with no daemon: it cannot have created the user")
	}
	if keyed(t, "nobody@h") {
		t.Fatal("a key that works before the name exists: the line was left behind after the daemon could not be reached")
	}
}

// A refusal from the daemon is the same shape: the key does not survive it.
func TestARefusedCreationLeavesNoKey(t *testing.T) {
	key := testHome(t)
	d := newDaemon()
	d.refuse["/user"] = http.StatusForbidden
	t.Setenv("AGENT_BUS_ADDR", d.serve(t))

	err := userAdd([]string{"denied@h", key})
	if err == nil {
		t.Fatal("user add succeeded although the daemon refused to create the user")
	}
	if keyed(t, "denied@h") {
		t.Fatal("the key outlived a refused creation")
	}
}

// Adding a second key for somebody already here is the same operation, so an
// already-known name is not an error — only a genuinely refused creation is.
func TestAnAlreadyKnownNameStillGetsItsKey(t *testing.T) {
	key := testHome(t)
	d := newDaemon()
	d.exists["known@h"] = true
	t.Setenv("AGENT_BUS_ADDR", d.serve(t))

	if err := userAdd([]string{"known@h", key}); err != nil {
		t.Fatalf("user add refused for a name the daemon already holds: %v", err)
	}
	if !keyed(t, "known@h") {
		t.Fatal("an existing user did not get the key that was being added")
	}
}

// Existing behaviour that must survive: one line per name.
func TestAddingTwiceReplacesTheLine(t *testing.T) {
	key := testHome(t)
	d := newDaemon()
	t.Setenv("AGENT_BUS_ADDR", d.serve(t))

	if err := userAdd([]string{"twice@h", key}); err != nil {
		t.Fatal(err)
	}
	d.exists["twice@h"] = true
	if err := userAdd([]string{"twice@h", key}); err != nil {
		t.Fatal(err)
	}
	lines, err := keysFile()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range lines {
		if who, _ := whose(l); who == "twice@h" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("twice@h has %d lines, want 1", n)
	}
}

// The forced command is what the key reaches, and --admin is what changes it.
func TestForcedCommandFollowsTheFlag(t *testing.T) {
	key := testHome(t)
	d := newDaemon()
	t.Setenv("AGENT_BUS_ADDR", d.serve(t))

	if err := userAdd([]string{"plain@h", key}); err != nil {
		t.Fatal(err)
	}
	if err := userAdd([]string{"boss@h", key, "--admin"}); err != nil {
		t.Fatal(err)
	}
	lines, err := keysFile()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		who, what := whose(l)
		switch who {
		case "plain@h":
			if !strings.Contains(what, "agent-bus-token") {
				t.Fatalf("plain@h is forced into %q, want the token program", what)
			}
		case "boss@h":
			if !strings.Contains(what, "agent-bus-admin") {
				t.Fatalf("boss@h is forced into %q, want the admin program", what)
			}
		}
	}
}

func TestLocalPersonNameComesFromAccountLookup(t *testing.T) {
	d := newDaemon()
	d.profiles["owner@h"] = protocol.User{Name: "owner@h", Status: "active", Email: "owner@example.com"}
	t.Setenv("AGENT_BUS_ADDR", d.serve(t))
	lookedUp := ""
	lookup := func(account string) (*user.User, error) {
		lookedUp = account
		return &user.User{Username: account, Name: " Alice From Passwd "}, nil
	}
	if err := userImportLocal([]string{"owner@h", "alice"}, lookup); err != nil {
		t.Fatal(err)
	}
	if lookedUp != "alice" {
		t.Fatalf("looked up %q, want alice", lookedUp)
	}
	got := d.profiles["owner@h"]
	if got.PersonName != "Alice From Passwd" || got.Email != "owner@example.com" || got.Status != "active" {
		t.Fatalf("local import changed the wrong profile fields: %+v", got)
	}
}

func TestLocalImportPreservesExplicitPersonName(t *testing.T) {
	d := newDaemon()
	d.profiles["owner@h"] = protocol.User{Name: "owner@h", PersonName: "Administrator Choice", Status: "active"}
	t.Setenv("AGENT_BUS_ADDR", d.serve(t))
	if err := userImportLocal([]string{"owner@h", "alice"}, func(string) (*user.User, error) {
		return &user.User{Name: "Passwd Choice"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := d.profiles["owner@h"].PersonName; got != "Administrator Choice" {
		t.Fatalf("local import overwrote %q", got)
	}
	for _, request := range d.requests {
		if request == "POST /user owner@h" {
			t.Fatal("preserved profile was posted back unnecessarily")
		}
	}
}

func TestLocalImportAcceptsNoCallerPersonName(t *testing.T) {
	called := false
	lookup := func(string) (*user.User, error) {
		called = true
		return nil, errors.New("must not run")
	}
	if err := userImportLocal([]string{"owner@h", "alice", "Forged Name"}, lookup); err == nil {
		t.Fatal("free-form person name was accepted")
	}
	if called {
		t.Fatal("invalid free-form input reached the trusted account lookup")
	}
}

// A caller aimed at another daemon is refused rather than switched to the
// installed account, where sudo would drop its address and edit the live node.
func TestAnotherDaemonsAddressIsNotTradedForTheInstalledOne(t *testing.T) {
	if err := switchRefusal(""); err != nil {
		t.Errorf("no address: %v", err)
	}
	if err := switchRefusal(installedSocket); err != nil {
		t.Errorf("the installed socket: %v", err)
	}
	if err := switchRefusal("/tmp/f12/bus.sock"); err == nil || !strings.Contains(err.Error(), "AGENT_BUS_HOME") {
		t.Errorf("another daemon's socket was not refused: %v", err)
	}
}

// The forced command follows the path the program was run as, so a key added
// through /usr/local/bin keeps working after a deploy prunes the release.
func TestTheForcedCommandUsesTheInvokedPath(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "agent-bus-admin")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, link); err != nil {
		t.Fatal(err)
	}
	was := os.Args[0]
	defer func() { os.Args[0] = was }()
	os.Args[0] = link
	if got, err := invoked(); err != nil || got != link {
		t.Fatalf("invoked() = %q, %v; want the symlink %q, not the file it resolves to", got, err, link)
	}
}
