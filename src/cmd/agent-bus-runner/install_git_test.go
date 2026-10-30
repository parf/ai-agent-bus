package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployFetchChecksHeadAndIsolatesCredentials(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	dest := filepath.Join(root, "stage")
	host := filepath.Join(root, "host")
	log := filepath.Join(root, "environment")
	wrong := filepath.Join(root, "wrong")
	for _, p := range []string{bin, dest, host} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	literal := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	pin := strings.Repeat("a", 40)
	script := "#!/usr/bin/python3\nimport os,sys,json\njson.dump(dict(os.environ),open(" + literal(log) + ",'w'))\nif 'rev-parse' in sys.argv: print(('f'*40) if os.path.exists(" + literal(wrong) + ") else " + literal(pin) + ")\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, key := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", "SSH_AUTH_SOCK", "GIT_SSH_COMMAND", "HOME", "GIT_ASKPASS"} {
		t.Setenv(key, "private-sentinel")
	}
	r := &runner{host: host}
	in := installRequest{agent: "fixture", remote: "https://fixture.invalid/repo", commit: pin}
	if err := r.fetchDeploy(context.Background(), in, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]string
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "private-sentinel") || env["GIT_ALLOW_PROTOCOL"] != "https:ssh" || !strings.Contains(env["GIT_SSH_COMMAND"], filepath.Join(host, ".deploy", "id")) || !strings.Contains(env["GIT_SSH_COMMAND"], "StrictHostKeyChecking=yes") {
		t.Fatalf("ambient credentials or missing host policy: %s", b)
	}
	writeFile(t, wrong, "wrong")
	if err := r.fetchDeploy(context.Background(), in, dest); err == nil || !strings.Contains(err.Error(), "HEAD does not equal") {
		t.Fatalf("wrong fetched HEAD accepted: %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.fetchDeploy(canceled, in, dest); err == nil {
		t.Fatal("canceled deploy succeeded")
	}
}
