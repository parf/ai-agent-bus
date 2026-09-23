package main

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

// What a stranger sees: the sign-in page says what this project is, shows the
// project's picture and links its repository and author. A page behind a
// session is for somebody already here and carries none of it.
func TestPublicPageDescribesTheProject(t *testing.T) {
	m := meaningFixture(t)
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	dashboard(&caller{client: m.backend.Client(), base: m.backend.URL}, false).ServeHTTP(w, r)
	page, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	out := string(page)
	if !strings.Contains(out, "<h1>One bus for agents, bots and services</h1>") {
		t.Fatalf("not the signed-out page: %.200s", out)
	}
	body := section(t, out, "<main>", "<footer")
	// What this is, said on the page rather than only in the small print.
	for _, want := range []string{
		"Connect AI and NON-AI agents, bots and services so they can find and message each other.",
		"One daemon gives you a registry, message queues, an MCP server, dashboard and much more",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the signed-out page does not say %q", want)
		}
	}
	// The project's links, once. They were in the footer as well, and one
	// short page saying the same thing twice is what that looked like.
	for _, want := range []string{
		`href="https://github.com/parf/ai-agent-bus"`,
		`href="https://parf.dev/"`,
		"Serg Parf",
	} {
		if n := strings.Count(out, want); n != 1 {
			t.Errorf("the signed-out page carries %q %d times, not once", want, n)
		}
		if !strings.Contains(body, want) {
			t.Errorf("the signed-out page does not carry %q where a visitor reads it", want)
		}
	}
	if !strings.Contains(body, "src=/agent-bus.jpg") {
		t.Errorf("the signed-out page does not show the project picture: %.400s", body)
	}
	if !strings.Contains(body, "alt=\"A red double-decker named Agents Bus") {
		t.Errorf("the picture is not described for a reader who cannot see it")
	}
}

// The picture has to be one this page's own policy allows. img-src is 'self'
// and nothing else, so a picture hosted anywhere else is named by the HTML and
// then refused by the browser: the page looks exactly as it did before the
// picture was added, which is how this was found.
func TestThePicturePassesThePagesOwnPolicy(t *testing.T) {
	m := meaningFixture(t)
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	dashboard(&caller{client: m.backend.Client(), base: m.backend.URL}, false).ServeHTTP(w, r)
	out := readBody(t, w)
	policy := w.Result().Header.Get("Content-Security-Policy")
	if !strings.Contains(policy, "img-src 'self'") {
		t.Fatalf("this check assumes the page restricts images to this node; policy is %q", policy)
	}
	for _, src := range imageSources(section(t, out, "<main>", "<footer")) {
		if strings.HasPrefix(src, "http") || strings.HasPrefix(src, "//") {
			t.Errorf("the page names a picture at %q, which %q refuses", src, policy)
		}
	}
}

// And the node answers for it, before any credential: the page that shows it
// is the one nobody has signed in to yet.
func TestTheNodeServesThePictureToAStranger(t *testing.T) {
	m := meaningFixture(t)
	w := httptest.NewRecorder()
	dashboard(&caller{client: m.backend.Client(), base: m.backend.URL}, false).
		ServeHTTP(w, httptest.NewRequest("GET", "/agent-bus.jpg", nil))
	res := w.Result()
	if res.StatusCode != 200 {
		t.Fatalf("a stranger asking for the picture got %d", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("the picture is served as %q", got)
	}
	if got := res.Header.Get("Cache-Control"); got != "max-age=86400" {
		t.Errorf("the picture is served with Cache-Control %q, so every visit re-sends it", got)
	}
	picture, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(picture) < 3 || picture[0] != 0xFF || picture[1] != 0xD8 || picture[2] != 0xFF {
		t.Fatalf("the answer is not a JPEG: %d bytes, starts %x", len(picture), picture[:min(3, len(picture))])
	}
	// The original is 2.5MB. Serving that to every visitor is what the
	// downscaled copy exists to avoid, so hold the size it was cut to.
	if len(picture) > 200_000 {
		t.Errorf("the picture is %d bytes; the page was given a downscaled copy", len(picture))
	}
}

// imageSources is every img src on the page, quoted or bare.
func imageSources(html string) []string {
	var out []string
	for rest := html; ; {
		i := strings.Index(rest, "<img ")
		if i < 0 {
			return out
		}
		tag := rest[i:]
		if end := strings.Index(tag, ">"); end >= 0 {
			tag = tag[:end]
		}
		rest = rest[i+5:]
		j := strings.Index(tag, "src=")
		if j < 0 {
			continue
		}
		src := tag[j+4:]
		if strings.HasPrefix(src, `"`) {
			src = src[1:]
			if k := strings.Index(src, `"`); k >= 0 {
				src = src[:k]
			}
		} else if k := strings.IndexAny(src, " \t"); k >= 0 {
			src = src[:k]
		}
		out = append(out, src)
	}
}

func readBody(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	page, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(page)
}

// A visitor is asked for a token, so the page says where one comes from: a
// hover tooltip on the control beside the heading, and the same two commands
// in a popover for anyone who cannot hover.
func TestTheSignInFormSaysHowToGetAToken(t *testing.T) {
	m := meaningFixture(t)
	w := httptest.NewRecorder()
	dashboard(&caller{client: m.backend.Client(), base: m.backend.URL}, false).
		ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	out := readBody(t, w)
	card := section(t, out, "<div class=signin-card>", "</main>")
	if !strings.Contains(card, `name=token`) {
		t.Fatal("the sign-in card has no token field")
	}
	control := section(t, card, "<button type=button class=help-button", ">")
	if !strings.Contains(control, "popovertarget=token-help") {
		t.Error("the help control opens nothing")
	}
	hover := section(t, control, `data-tooltip="`, `"`)
	for _, want := range []string{"agent-bus-token", "ssh agent-busd@"} {
		if !strings.Contains(hover, want) {
			t.Errorf("hovering the control does not mention %q: %q", want, hover)
		}
	}
	help := section(t, out, "<div popover id=token-help", "</ul>")
	for _, want := range []string{"agent-bus-token", "ssh agent-busd@"} {
		if !strings.Contains(help, want) {
			t.Errorf("the token help does not mention %q", want)
		}
	}
}

func TestASignedInPageKeepsTheProjectPitchOut(t *testing.T) {
	m := meaningFixture(t)
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(m.session)
	w := httptest.NewRecorder()
	dashboard(&caller{client: m.backend.Client(), base: m.backend.URL}, false).ServeHTTP(w, r)
	page, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	out := string(page)
	// Signed in is the sign-out form present and the token field absent: the
	// heading this used to look for is no longer on the landing page.
	if !strings.Contains(out, "<form method=post action=/signout") || strings.Contains(out, "name=token") {
		t.Fatalf("expected a signed-in page, got the landing page")
	}
	for _, unwanted := range []string{"Connect AI and NON-AI agents", "github.com/parf/ai-agent-bus", "Serg Parf"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("a signed-in page carries %q", unwanted)
		}
	}
}
