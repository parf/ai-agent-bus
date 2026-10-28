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

func TestMailOAuthAndPasswordAccountsDoNotCrossRead(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf unused")
	r.mu.Lock()
	s := r.served["#fixture@h"]
	r.mu.Unlock()
	writeFile(t, filepath.Join(r.agents, "fixture", "env.dist"), "SMTP_USERNAME= # required\nSMTP_PASSWORD=\nSMTP_CLIENT_ID=\nSMTP_CLIENT_SECRET=\nSMTP_REFRESH_TOKEN=\n")
	writeFile(t, filepath.Join(r.host, "fixture", "env"), "MAIL_MODE=starttls\n")
	writeFile(t, filepath.Join(r.host, "fixture", "work", "env"), "SMTP_USERNAME=work@example.com\nSMTP_CLIENT_ID=fixture-oauth-client\nSMTP_CLIENT_SECRET=fixture-oauth-secret\nSMTP_REFRESH_TOKEN=fixture-oauth-refresh\n")
	writeFile(t, filepath.Join(r.host, "fixture", "personal", "env"), "SMTP_USERNAME=personal@example.com\nSMTP_PASSWORD=fixture-password-only\n")
	for _, account := range []string{"work", "personal"} {
		in := s.in
		in.Key = "fixture/" + account
		in.Host.Name = "#fixture@h"
		env, _, why := r.environmentWithSecrets(in, s.bus)
		if why != "" {
			t.Fatal(why)
		}
		joined := strings.Join(env, "\n")
		if !strings.Contains(joined, "SMTP_USERNAME="+account+"@example.com") {
			t.Fatal("account user mismatch")
		}
		if account == "work" {
			if !strings.Contains(joined, "SMTP_REFRESH_TOKEN=fixture-oauth-refresh") || !strings.Contains(joined, "SMTP_CLIENT_SECRET=fixture-oauth-secret") || strings.Contains(joined, "fixture-password-only") {
				t.Fatal("OAuth account lost its credential boundary")
			}
		} else if !strings.Contains(joined, "SMTP_PASSWORD=fixture-password-only") || strings.Contains(joined, "fixture-oauth-") {
			t.Fatal("password account inherited OAuth credentials")
		}
	}
}
