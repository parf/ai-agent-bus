// Package github is the directory adapter for GitHub logins: the public keys
// a login publishes are at https://github.com/<login>.keys, which is a plain
// GET and needs no token, no app and no webhook. HTTP is built in, never a
// subprocess (docs/10-modules.md#the-rule).
//
// It fetches and nothing else. What proves the caller holds one of these keys
// is a step of its own (docs/01-identity-and-roles.md#registration).
package github

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Directory struct {
	base   string
	client *http.Client
}

func New() *Directory {
	return &Directory{base: "https://github.com", client: &http.Client{Timeout: 10 * time.Second}}
}

// At points the adapter at another host, which is how it is exercised without
// reaching the internet.
func At(base string) *Directory {
	return &Directory{base: strings.TrimSuffix(base, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

func (d *Directory) Keys(login string) ([]string, error) {
	if login == "" || strings.ContainsAny(login, "/?#") {
		return nil, fmt.Errorf("not a login: %q", login)
	}
	resp, err := d.client.Get(d.base + "/" + login + ".keys")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s.keys: %s", login, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}
