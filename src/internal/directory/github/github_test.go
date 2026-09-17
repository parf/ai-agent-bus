package github

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	if strings.Join(entry.Keys, "|") != "ssh-ed25519 AAAAone|ssh-rsa AAAAtwo comment" || entry.PersonName != "Alice Example" {
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
