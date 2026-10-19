package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientSocketDiscovery(t *testing.T) {
	base := shortSocketDir(t)
	login, system := filepath.Join(base, "login"), filepath.Join(base, "system")
	for _, dir := range []string{login, system} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	dirs := []string{login, system}
	check := func(token bool, want string) {
		t.Helper()
		if got := clientSocket(dirs, "tester", token); got != want {
			t.Fatalf("socket with token=%v: got %s, want %s", token, got, want)
		}
	}
	listen := func(path string) {
		t.Helper()
		l, err := listenTestUnix(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { l.Close() })
	}
	// Nothing listening: the installed directory's path, so the call fails
	// there, never a guess in world-writable /tmp (E1).
	check(false, UserSocket(system, "tester"))
	check(true, filepath.Join(system, "bus.sock"))
	systemUser := UserSocket(system, "tester")
	if err := os.WriteFile(systemUser, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	check(false, systemUser) // A regular file is not a listener: nothing found.
	if err := os.Remove(systemUser); err != nil {
		t.Fatal(err)
	}
	listen(systemUser)
	check(false, systemUser)
	loginUser := UserSocket(login, "tester")
	listen(loginUser)
	check(false, loginUser)
	check(true, filepath.Join(system, "bus.sock")) // Never replace a token with the user's identity.
	shared := filepath.Join(system, "bus.sock")
	listen(shared)
	check(true, shared)
}

// A unix address with no daemon behind it says so, naming the path.
func TestDialingNoDaemonSaysSo(t *testing.T) {
	path := filepath.Join(shortSocketDir(t), "absent.sock")
	client, base := Dial(path)
	_, err := client.Get(base + "/status")
	if err == nil || !strings.Contains(err.Error(), "no daemon found at "+path) {
		t.Fatalf("a call with no daemon: %v", err)
	}
}
