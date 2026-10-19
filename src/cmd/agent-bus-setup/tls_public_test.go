package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A root installer creates the real files; the parent test stays an ordinary
// account, outside the TLS key's group, when it runs the public token command.
func TestPublicTLSInstallFixture(t *testing.T) {
	dir := os.Getenv("AB_PUBLIC_TLS_FIXTURE")
	if dir == "" {
		return
	}
	c := tlsChoice{dir: dir, mode: "self-signed", names: list{"fixture"}}
	if os.Getenv("AB_PUBLIC_TLS_REPAIR") == "1" {
		c.mode = ""
	}
	if _, err := c.install("root", io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestOrdinaryUserCanReadInstalledFingerprint(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("ordinary-user permission check")
	}
	if err := exec.Command("sudo", "-n", "true").Run(); err != nil {
		t.Skip("root installation fixture needs passwordless sudo")
	}
	base := t.TempDir()
	dir := filepath.Join(base, "tls")
	t.Cleanup(func() {
		if out, err := exec.Command("sudo", "-n", "rm", "-rf", "--", dir).CombinedOutput(); err != nil {
			t.Errorf("clean TLS fixture: %v %s", err, out)
		}
	})
	token := filepath.Join(base, "agent-bus-token")
	if out, err := exec.Command("go", "build", "-o", token, "../agent-bus-token").CombinedOutput(); err != nil {
		t.Fatalf("build token: %v %s", err, out)
	}
	install := func(repair bool) {
		flag := "0"
		if repair {
			flag = "1"
		}
		out, err := exec.Command("sudo", "-n", "env", "AB_PUBLIC_TLS_FIXTURE="+dir, "AB_PUBLIC_TLS_REPAIR="+flag, os.Args[0], "-test.run=^TestPublicTLSInstallFixture$").CombinedOutput()
		if err != nil {
			t.Fatalf("root setup: %v %s", err, out)
		}
	}
	pin := func() string {
		c := exec.Command(token, "--fingerprint")
		c.Env = append(os.Environ(), "AGENT_BUS_TLS_DIR="+dir)
		out, err := c.CombinedOutput()
		if err != nil || !strings.HasPrefix(string(out), "sha256:") {
			t.Fatalf("ordinary user fingerprint: %v %s", err, out)
		}
		if _, err := os.ReadFile(filepath.Join(dir, "key.pem")); !os.IsPermission(err) {
			t.Fatalf("private key is not protected: %v", err)
		}
		st, err := os.Stat(filepath.Join(dir, "key.pem"))
		if err != nil || st.Mode().Perm() != 0640 {
			t.Fatalf("key mode changed: %v %v", st, err)
		}
		return strings.TrimSpace(string(out))
	}
	install(false)
	before := pin()
	if out, err := exec.Command("sudo", "-n", "chmod", "0750", dir).CombinedOutput(); err != nil {
		t.Fatalf("old-directory fixture: %v %s", err, out)
	}
	install(true)
	if after := pin(); after != before {
		t.Fatal("repair rotated the certificate")
	}
}
