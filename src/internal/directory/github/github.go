// Package github is the directory adapter for GitHub logins: the public keys
// a login publishes are at https://github.com/<login>.keys, which is a plain
// GET and needs no token, no app and no webhook. HTTP is built in, never a
// subprocess (docs/10-modules.md#the-rule).
//
// It fetches directory facts and nothing else. What proves the caller holds
// one of these keys is a step of its own
// (docs/01-identity-and-roles.md#registration).
package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

type Directory struct {
	keysBase    string
	profileBase string
	client      *http.Client
}

func New() *Directory {
	return &Directory{
		keysBase:    "https://github.com",
		profileBase: "https://api.github.com/users",
		client:      &http.Client{Timeout: 10 * time.Second},
	}
}

// At points the adapter at another host, which is how it is exercised without
// reaching the internet.
func At(base string) *Directory {
	base = strings.TrimSuffix(base, "/")
	return &Directory{keysBase: base, profileBase: base + "/users", client: &http.Client{Timeout: 10 * time.Second}}
}

func (d *Directory) Lookup(login string) (ports.DirectoryEntry, error) {
	if login == "" || strings.ContainsAny(login, "/?#") {
		return ports.DirectoryEntry{}, fmt.Errorf("not a login: %q", login)
	}
	keys, err := d.get(d.keysBase + "/" + login + ".keys")
	if err != nil {
		return ports.DirectoryEntry{}, err
	}
	profile, err := d.get(d.profileBase + "/" + login)
	if err != nil {
		return ports.DirectoryEntry{}, err
	}
	var out []string
	for _, line := range strings.Split(string(keys), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(profile, &p); err != nil {
		return ports.DirectoryEntry{}, fmt.Errorf("%s profile: %w", login, err)
	}
	return ports.DirectoryEntry{Keys: out, PersonName: strings.TrimSpace(p.Name)}, nil
}

func (d *Directory) get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "agent-bus")
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", req.URL.Path, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<16))
}
