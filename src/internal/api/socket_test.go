package api

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestClientSocketDiscovery(t *testing.T) {
	// Keep the address short: Unix sockets cap path length, and t.TempDir
	// embeds the test name beneath a potentially long TMPDIR.
	root := filepath.Join("..", "..", "..", "tmp", "socket-tests")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	base, err := os.MkdirTemp(root, "s-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
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
		l, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { l.Close() })
	}
	check(false, DefaultSocket())
	systemUser := UserSocket(system, "tester")
	if err := os.WriteFile(systemUser, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	check(false, DefaultSocket()) // A regular file is not a listener.
	if err := os.Remove(systemUser); err != nil {
		t.Fatal(err)
	}
	listen(systemUser)
	check(false, systemUser)
	loginUser := UserSocket(login, "tester")
	listen(loginUser)
	check(false, loginUser)
	check(true, DefaultSocket()) // Never replace a token with the user's identity.
	shared := filepath.Join(system, "bus.sock")
	listen(shared)
	check(true, shared)
}
