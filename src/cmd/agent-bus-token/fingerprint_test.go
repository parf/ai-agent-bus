package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFingerprintReportsReadFailureTruthfully(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission denial requires non-root")
	}
	old := os.Args
	os.Args = []string{"agent-bus-token", "--fingerprint"}
	defer func() { os.Args = old }()
	dir := t.TempDir()
	t.Setenv("AGENT_BUS_TLS_DIR", dir)
	cert := filepath.Join(dir, "cert.pem")
	if err := os.WriteFile(cert, []byte("fixture"), 0000); err != nil {
		t.Fatal(err)
	}
	err := issue()
	if err == nil || !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "cannot read") || strings.Contains(err.Error(), "no TLS") {
		t.Fatalf("wrong unreadable-certificate error: %v", err)
	}
	os.Remove(cert)
	err = issue()
	if err == nil || !strings.Contains(err.Error(), "no TLS certificate") {
		t.Fatalf("wrong absent-certificate error: %v", err)
	}
}
