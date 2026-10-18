package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTLSSetupPermissionFixture(t *testing.T) {
	if os.Getenv("AB_TLS_PERMISSION_FIXTURE") != "1" {
		return
	}
	os.Args = []string{"agent-bus-setup", "--print-unit", "--owner", "fixture@h", "--exec", os.Args[0]}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	if err := setup(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestSetupRefusesUnreadableTLSInsteadOfPrintingTLSOff(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission refusal needs an unprivileged account")
	}
	for _, mode := range []os.FileMode{0000, 0600} {
		t.Run(fmt.Sprintf("%04o", mode), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "tls")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"cert.pem", "key.pem"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(dir, mode); err != nil {
				t.Fatal(err)
			}
			defer os.Chmod(dir, 0700)
			cmd := exec.Command(os.Args[0], "-test.run=^TestTLSSetupPermissionFixture$")
			cmd.Env = append(os.Environ(), "AB_TLS_PERMISSION_FIXTURE=1", "AGENT_BUS_TLS_DIR="+dir)
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "permission denied") || strings.Contains(string(out), "[Unit]") {
				t.Fatalf("unreadable TLS became an off unit: err=%v out=%s", err, out)
			}
		})
	}
	// A genuinely absent directory still permits an explicitly plain unit.
	cmd := exec.Command(os.Args[0], "-test.run=^TestTLSSetupPermissionFixture$")
	cmd.Env = append(os.Environ(), "AB_TLS_PERMISSION_FIXTURE=1", "AGENT_BUS_TLS_DIR="+filepath.Join(t.TempDir(), "absent"))
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "[Unit]") || strings.Contains(string(out), "-tls-dir") {
		t.Fatalf("absent TLS control: err=%v out=%s", err, out)
	}
}
