package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMailAccountsUseDistinctInstanceHostCredentials(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf unused")
	r.mu.Lock()
	s := r.served["#fixture@h"]
	r.mu.Unlock()
	writeFile(t, filepath.Join(r.agents, "fixture", "env.dist"), "SMTP_USERNAME= # required\nSMTP_PASSWORD= # required\n")
	// This package-wide file intentionally contains no account credentials.
	writeFile(t, filepath.Join(r.host, "fixture", "env"), "MAIL_MODE=starttls\n")
	for _, account := range []string{"work", "personal"} {
		writeFile(t, filepath.Join(r.host, "fixture", account, "env"), "SMTP_USERNAME="+account+"@example.com\nSMTP_PASSWORD=fixture-"+account+"-private\n")
	}
	for _, account := range []string{"work", "personal"} {
		in := s.in
		in.Key = "fixture/" + account
		in.Host.Name = "#fixture@h"
		env, private, why := r.environmentWithSecrets(in, s.bus)
		if why != "" {
			t.Fatal(why)
		}
		joined := strings.Join(env, "\n")
		if !strings.Contains(joined, "SMTP_USERNAME="+account+"@example.com") || !strings.Contains(joined, "SMTP_PASSWORD=fixture-"+account+"-private") {
			t.Fatal("instance credentials were not selected")
		}
		other := "personal"
		if account == other {
			other = "work"
		}
		if strings.Contains(joined, other+"@example.com") || strings.Contains(joined, "fixture-"+other+"-private") {
			t.Fatal("another account's credentials crossed the instance boundary")
		}
		if !strings.Contains(strings.Join(private, "\n"), "SMTP_PASSWORD=fixture-"+account+"-private") {
			t.Fatal("mail credentials lost their private provenance")
		}
	}
	if _, _, why := r.environmentWithSecrets(s.in, s.bus); !strings.Contains(why, "SMTP_USERNAME") {
		t.Fatal("package-wide environment unexpectedly supplied an account")
	}
}
