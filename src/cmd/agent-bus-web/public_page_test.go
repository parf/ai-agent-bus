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
	if !strings.Contains(out, "Sign in to AgentBus") {
		t.Fatalf("not the signed-out page: %.200s", out)
	}
	footer := section(t, out, "<footer", "</footer>")
	for _, want := range []string{
		"Connect AI and NON-AI agents, bots and services so they can find and message each other.",
		"One daemon gives you a registry, message queues, an MCP server, dashboard and much more",
		`href="https://github.com/parf/ai-agent-bus"`,
		`href="https://parf.dev/"`,
		"Serg Parf",
	} {
		if !strings.Contains(footer, want) {
			t.Errorf("the signed-out footer does not carry %q", want)
		}
	}
	body := section(t, out, "<main>", "<footer")
	if !strings.Contains(body, "docs/img/agent-bus.png") {
		t.Errorf("the signed-out page does not show the project picture: %.400s", body)
	}
	if !strings.Contains(body, "alt=\"A red double-decker named Agents Bus") {
		t.Errorf("the picture is not described for a reader who cannot see it")
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
	if strings.Contains(out, "Sign in to AgentBus") {
		t.Fatalf("expected a signed-in page, got the sign-in form")
	}
	footer := section(t, out, "<footer", "</footer>")
	for _, unwanted := range []string{"Connect AI and NON-AI agents", "github.com/parf/ai-agent-bus", "Serg Parf"} {
		if strings.Contains(footer, unwanted) {
			t.Errorf("a signed-in footer carries %q", unwanted)
		}
	}
}
