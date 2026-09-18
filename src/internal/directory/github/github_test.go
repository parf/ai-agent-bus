package github

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			im.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 100, A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, im); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestLookupReturnsKeysAndTrustedProfileName(t *testing.T) {
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		if r.Header.Get("User-Agent") != "agent-bus" {
			t.Fatalf("missing user agent: %q", r.Header.Get("User-Agent"))
		}
		switch r.URL.Path {
		case "/alice.keys":
			w.Write([]byte("ssh-ed25519 AAAAone\nssh-rsa AAAAtwo comment\n"))
		case "/users/alice":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":" Alice Example "}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	entry, err := At(srv.URL).Lookup("alice")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(entry.Keys, "|") != "ssh-ed25519 AAAAone|ssh-rsa AAAAtwo comment" || entry.Profile.PersonName != "Alice Example" {
		t.Fatalf("wrong directory entry: %+v", entry)
	}
	if strings.Join(requests, "|") != "/alice.keys|/users/alice" {
		t.Fatalf("wrong provider requests: %v", requests)
	}
}

func TestLookupRequiresBothProviderAnswers(t *testing.T) {
	for _, failed := range []string{"/alice.keys", "/users/alice"} {
		t.Run(failed, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == failed {
					http.Error(w, "gone", http.StatusBadGateway)
					return
				}
				if strings.HasSuffix(r.URL.Path, ".keys") {
					w.Write([]byte("ssh-ed25519 AAAAone\n"))
					return
				}
				w.Write([]byte(`{"name":"Alice"}`))
			}))
			defer srv.Close()
			if _, err := At(srv.URL).Lookup("alice"); err == nil {
				t.Fatal("partial provider answer was accepted")
			}
		})
	}
}

func TestProfileReturnsFieldsAndNormalizedLocalPhoto(t *testing.T) {
	photo := testPNG(t, 4, 2)
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/alice":
			w.Write([]byte(`{"login":"Alice","name":" Alice ","email":"PUBLIC@example.com","company":" Example ","location":" NYC ","twitter_username":"alice_x","avatar_url":"` + base + `/photo","gravatar_id":"legacy"}`))
		case "/photo":
			w.Header().Set("Content-Type", "image/png")
			w.Write(photo)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base = srv.URL

	p, err := At(base).Profile("alice")
	if err != nil {
		t.Fatal(err)
	}
	if p.Login != "alice" || p.PersonName != "Alice" || p.Email != "PUBLIC@example.com" || p.Company != "Example" || p.Location != "NYC" || p.TwitterUsername != "alice_x" || p.GravatarID != "legacy" || p.PhotoSource != "github" || p.FetchedAt.IsZero() || p.PhotoFetchedAt.IsZero() {
		t.Fatalf("wrong profile: %+v", p)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(p.PhotoPNG))
	if err != nil || format != "png" || cfg.Width != thumbnailSide || cfg.Height != thumbnailSide {
		t.Fatalf("photo was not normalized to local PNG: %s %dx%d %v", format, cfg.Width, cfg.Height, err)
	}
}

func TestProfilePhotoFailureFallsBackAndNeverBroadensRedirectHost(t *testing.T) {
	photo := testPNG(t, 2, 2)
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/alice":
			w.Write([]byte(`{"login":"alice","avatar_url":"` + base + `/bad","gravatar_id":"legacy"}`))
		case "/bad":
			http.Redirect(w, r, strings.Replace(base, "127.0.0.1", "localhost", 1)+"/private", http.StatusFound)
		case "/private":
			w.Write(testPNG(t, 3, 3))
		case "/avatar/legacy":
			w.Write(photo)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base = srv.URL
	p, err := At(base).Profile("alice")
	if err != nil {
		t.Fatal(err)
	}
	if p.PhotoSource != "gravatar" || len(p.PhotoPNG) == 0 || p.ClearPhoto {
		t.Fatalf("safe fallback was not used: %+v", p)
	}
}

func TestPhotoByteAndDimensionBounds(t *testing.T) {
	tooWide := testPNG(t, maxPhotoSide+1, 1)
	atWidth := testPNG(t, maxPhotoSide, 1)
	basePhoto := testPNG(t, 2, 2)
	exactBytes := append(append([]byte(nil), basePhoto...), make([]byte, maxPhotoBytes-len(basePhoto))...)
	overBytes := append(append([]byte(nil), basePhoto...), make([]byte, maxPhotoBytes+1-len(basePhoto))...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/large":
			w.Write(overBytes)
		case "/exact-bytes":
			w.Write(exactBytes)
		case "/at-width":
			w.Write(atWidth)
		case "/not-image":
			w.Write([]byte("this is not an image"))
		default:
			w.Write(tooWide)
		}
	}))
	defer srv.Close()
	d := At(srv.URL)
	if _, err := d.photo(srv.URL + "/large"); err == nil {
		t.Fatal("oversized photo body was accepted")
	}
	if _, err := d.photo(srv.URL + "/wide"); err == nil {
		t.Fatal("oversized photo dimensions were accepted")
	}
	if _, err := d.photo(srv.URL + "/exact-bytes"); err != nil {
		t.Fatalf("exact byte bound was refused: %v", err)
	}
	if _, err := d.photo(srv.URL + "/at-width"); err != nil {
		t.Fatalf("exact dimension bound was refused: %v", err)
	}
	if _, err := d.photo(srv.URL + "/not-image"); err == nil {
		t.Fatal("non-image body was accepted")
	}
}
