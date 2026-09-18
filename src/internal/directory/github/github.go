// Package github is the directory adapter for GitHub logins: the public keys
// a login publishes are at https://github.com/<login>.keys and the public
// profile is at https://api.github.com/users/<login>. Fetching these facts is
// not authentication; key possession is proved separately.
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

const (
	maxProfileBytes = 1 << 16
	maxPhotoBytes   = 2 << 20
	maxPhotoSide    = 4096
	maxPhotoPixels  = 16 << 20
	thumbnailSide   = 96
)

type Directory struct {
	keysBase     string
	profileBase  string
	gravatarBase string
	client       *http.Client
	photoHosts   map[string]bool
	photoSchemes map[string]bool
	now          func() time.Time
}

func New() *Directory {
	return &Directory{
		keysBase:     "https://github.com",
		profileBase:  "https://api.github.com/users",
		gravatarBase: "https://secure.gravatar.com/avatar",
		client:       &http.Client{Timeout: 10 * time.Second},
		photoHosts: map[string]bool{
			"avatars.githubusercontent.com": true,
			"secure.gravatar.com":           true,
			"www.gravatar.com":              true,
			"gravatar.com":                  true,
		},
		photoSchemes: map[string]bool{"https": true},
		now:          time.Now,
	}
}

// At points every provider endpoint at one fixture host. Production never
// broadens its host allow-list; this constructor is how tests avoid the net.
func At(base string) *Directory {
	base = strings.TrimSuffix(base, "/")
	u, _ := url.Parse(base)
	return &Directory{
		keysBase:     base,
		profileBase:  base + "/users",
		gravatarBase: base + "/avatar",
		client:       &http.Client{Timeout: 10 * time.Second},
		photoHosts:   map[string]bool{strings.ToLower(u.Hostname()): true},
		photoSchemes: map[string]bool{u.Scheme: true},
		now:          time.Now,
	}
}

func validLogin(login string) error {
	if login == "" || strings.ContainsAny(login, "/?#") {
		return fmt.Errorf("not a login: %q", login)
	}
	return nil
}

func (d *Directory) Lookup(login string) (ports.DirectoryEntry, error) {
	if err := validLogin(login); err != nil {
		return ports.DirectoryEntry{}, err
	}
	keys, err := d.get(d.keysBase + "/" + login + ".keys")
	if err != nil {
		return ports.DirectoryEntry{}, err
	}
	profile, err := d.Profile(login)
	if err != nil {
		return ports.DirectoryEntry{}, err
	}
	var out []string
	for _, line := range strings.Split(string(keys), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return ports.DirectoryEntry{Keys: out, Profile: profile}, nil
}

// Profile returns a complete provider-profile answer. Importing the photo is
// deliberately optional: profile facts remain usable during an avatar outage.
func (d *Directory) Profile(login string) (ports.DirectoryProfile, error) {
	if err := validLogin(login); err != nil {
		return ports.DirectoryProfile{}, err
	}
	body, err := d.get(d.profileBase + "/" + login)
	if err != nil {
		return ports.DirectoryProfile{}, err
	}
	var raw struct {
		Login           string `json:"login"`
		Name            string `json:"name"`
		Email           string `json:"email"`
		Company         string `json:"company"`
		Location        string `json:"location"`
		TwitterUsername string `json:"twitter_username"`
		AvatarURL       string `json:"avatar_url"`
		GravatarID      string `json:"gravatar_id"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ports.DirectoryProfile{}, fmt.Errorf("%s profile: %w", login, err)
	}
	if raw.Login != "" && !strings.EqualFold(raw.Login, login) {
		return ports.DirectoryProfile{}, fmt.Errorf("%s profile answered for %q", login, raw.Login)
	}
	p := ports.DirectoryProfile{
		Login:           strings.ToLower(login),
		PersonName:      strings.TrimSpace(raw.Name),
		Email:           strings.TrimSpace(raw.Email),
		Company:         strings.TrimSpace(raw.Company),
		Location:        strings.TrimSpace(raw.Location),
		TwitterUsername: strings.TrimSpace(raw.TwitterUsername),
		AvatarURL:       strings.TrimSpace(raw.AvatarURL),
		GravatarID:      strings.TrimSpace(raw.GravatarID),
		FetchedAt:       d.now().UTC(),
	}
	for _, candidate := range []struct{ url, source string }{
		{p.AvatarURL, "github"},
		{d.gravatarURL(p.GravatarID), "gravatar"},
	} {
		if candidate.url == "" {
			continue
		}
		if photo, err := d.photo(candidate.url); err == nil {
			p.PhotoPNG, p.PhotoSource, p.PhotoFetchedAt = photo, candidate.source, d.now().UTC()
			return p, nil
		}
	}
	p.ClearPhoto = p.AvatarURL == "" && p.GravatarID == ""
	return p, nil
}

func (d *Directory) gravatarURL(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return d.gravatarBase + "/" + url.PathEscape(id) + "?s=192&d=404"
}

func (d *Directory) get(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProfileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxProfileBytes {
		return nil, fmt.Errorf("%s: response exceeds %d bytes", req.URL.Path, maxProfileBytes)
	}
	return body, nil
}

func (d *Directory) allowedPhotoURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.User != nil || !d.photoSchemes[u.Scheme] || !d.photoHosts[strings.ToLower(u.Hostname())] {
		return fmt.Errorf("profile photo location is not an approved provider host")
	}
	return nil
}

func (d *Directory) photo(rawURL string) ([]byte, error) {
	if err := d.allowedPhotoURL(rawURL); err != nil {
		return nil, err
	}
	client := *d.client
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many profile photo redirects")
		}
		return d.allowedPhotoURL(req.URL.String())
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/*")
	req.Header.Set("User-Agent", "agent-bus")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", req.URL.Path, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPhotoBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPhotoBytes {
		return nil, fmt.Errorf("profile photo exceeds %d bytes", maxPhotoBytes)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxPhotoSide || cfg.Height > maxPhotoSide || int64(cfg.Width)*int64(cfg.Height) > maxPhotoPixels {
		return nil, fmt.Errorf("profile photo dimensions are invalid or exceed %dx%d", maxPhotoSide, maxPhotoSide)
	}
	src, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("decode profile photo: %w", err)
	}
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	x0, y0 := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
	dst := image.NewNRGBA(image.Rect(0, 0, thumbnailSide, thumbnailSide))
	for y := 0; y < thumbnailSide; y++ {
		for x := 0; x < thumbnailSide; x++ {
			sx := x0 + x*side/thumbnailSide
			sy := y0 + y*side/thumbnailSide
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
