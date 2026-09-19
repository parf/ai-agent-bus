package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestTitleMarksAreFixedDecorativePageCategories(t *testing.T) {
	for category, visible := range map[string]string{
		"credentials": "🔑", "services": "⚙️", "agent": "👾",
		"users": "👤", "groups": "👥", "identity": "🪪",
		// Overview is a glyph rather than the bus mark: the header already
		// carries that logo, so the title repeated it instead of naming a page.
		"overview": "🏠",
	} {
		got := string(titleMark(category))
		if !strings.Contains(got, visible) || !strings.Contains(got, "aria-hidden=true") {
			t.Errorf("%s title mark = %q", category, got)
		}
	}
	for _, category := range []string{"channels", "activity", "diagnostics", "problem"} {
		got := string(titleMark(category))
		if !strings.Contains(got, "<svg") || !strings.Contains(got, "aria-hidden=") || !strings.Contains(got, `focusable="false"`) || !strings.Contains(got, "page-title-mark") {
			t.Errorf("%s title mark is not a decorative inline image: %q", category, got)
		}
	}
	if got := string(titleMark(`<script>alert(1)</script>`)); strings.Contains(got, "script") || !strings.Contains(got, "⚙️") {
		t.Fatalf("unknown category entered title markup or lost the section fallback: %q", got)
	}
}

func TestListHelpUsesVisibleAccessiblePopoverControls(t *testing.T) {
	m := meaningFixture(t)
	services := m.get("/services")
	for _, want := range []string{
		`class=help-button popovertarget=service-views-help aria-label="About service views">ⓘ</button>`,
		`<div popover id=service-views-help class=context-help>`,
		`<h2>About these records</h2><ul>`,
		"None of it is health", "does not establish that a send will be accepted",
	} {
		if !strings.Contains(services, want) {
			t.Errorf("Services help missing %q", want)
		}
	}
	if strings.Contains(services, `<p class=muted>Three separate facts`) {
		t.Error("the former Services prose wall remains in the primary flow")
	}
	channels := m.get("/channels")
	if !strings.Contains(channels, "All counts the caller-visible Channel and Inbox records") || strings.Contains(channels, "All and My omit Personal services") {
		t.Error("Channel help reused the Service category explanation")
	}

	users := m.get("/users")
	for _, want := range []string{
		`class=help-button popovertarget=identity-types-help aria-label="About identity types">ⓘ</button>`,
		`<div popover id=identity-types-help class=context-help>`,
		`<h2>About identity types</h2><ul>`,
		"credential with no registered name", "not figures the daemon reported",
	} {
		if !strings.Contains(users, want) {
			t.Errorf("Users help missing %q", want)
		}
	}
	if strings.Contains(users, `<p>Three kinds, named rather than guessed`) {
		t.Error("the former Users prose wall remains in the primary flow")
	}
}

func TestServiceExplanationsUseImmediateTooltipsAndStructuredPopovers(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "svc@h", Owner: "admin@h"})
	page := m.get("/service?name=svc@h")
	for _, want := range []string{
		`popovertarget=delivery-help aria-label="About delivery state" data-tooltip="Stored setting only.`,
		`popovertarget=policy-help aria-label="About record policy" data-tooltip="Queue values are this record's stored policy.`,
		`popovertarget=observed-help aria-label="About live counters" data-tooltip="Readers are outstanding requests`,
		`popovertarget=record-activity-help aria-label="About this activity history" data-tooltip="About 24 hours`,
		`.help-button[data-tooltip]:hover::after`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("service detail lacks immediate help %q", want)
		}
	}
	for _, id := range []string{"delivery-help", "policy-help", "observed-help", "record-activity-help"} {
		if !strings.Contains(page, `<div popover id=`+id+` class=context-help>`) {
			t.Errorf("service detail lost structured %s popover", id)
		}
	}
	if !strings.Contains(page, `<summary id=settings>Edit settings</summary>`) || !strings.Contains(page, `<summary>Classification and sharing</summary>`) {
		t.Error("infrequent service editors are not collapsed")
	}
}

func TestPageTitlesUseSectionOrDaemonStatedKind(t *testing.T) {
	m := meaningFixture(t)
	resp, err := m.client.Get(m.web.URL + "/signin")
	if err != nil {
		t.Fatal(err)
	}
	public, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	publicPage := string(public)
	if resp.StatusCode != http.StatusOK || !strings.Contains(publicPage, `🔑</span> Sign in to AgentBus</h1>`) {
		t.Fatal("public sign-in page lost its credential title")
	}
	if strings.Count(publicPage, `<a class=skip-link href=#main>Skip to main content</a>`) != 1 ||
		strings.Count(publicPage, `<a id=main tabindex=-1></a>`) != 1 {
		t.Fatal("public sign-in page lacks one keyboard skip target")
	}
	for _, record := range []protocol.Record{
		{Name: "service@h", Owner: "admin@h", Kind: "generic"},
		{Name: "agent@h", Owner: "admin@h", Kind: "agent"},
		{Name: "channel@h", Owner: "admin@h", Kind: protocol.KindTopic, Mode: protocol.ModeQueue},
	} {
		m.register(record)
	}
	pages := map[string]string{
		"/":                              `🏠</span> Overview</h1>`,
		"/diagnostics":                   `</svg> Diagnostics</h1>`,
		"/services":                      `⚙️</span> Services</h1>`,
		"/personal":                      `⚙️</span> Personal services</h1>`,
		"/channels":                      `</svg> Channels</h1>`,
		"/users":                         `👤</span> Users and other identities</h1>`,
		"/groups":                        `👥</span> Groups</h1>`,
		"/activity":                      `</svg> Activity graphs</h1>`,
		"/services/new":                  `⚙️</span> Register service</h1>`,
		"/channels/new":                  `</svg> Register channel or inbox</h1>`,
		"/users/new":                     `👤</span> Add user</h1>`,
		"/groups/new":                    `👥</span> Register group</h1>`,
		"/service?name=service@h":        `⚙️</span> service@h</h1>`,
		"/service?name=agent@h":          `👾</span> agent@h</h1>`,
		"/service?name=channel@h":        `</svg> channel@h</h1>`,
		"/service-danger?name=service@h": `</svg> Danger Zone · service@h</h1>`,
	}
	for path, want := range pages {
		body := m.get(path)
		if !strings.Contains(body, `class=page-title`) || !strings.Contains(body, want) {
			t.Errorf("%s lacks its title category %q", path, want)
		}
	}

	form := url.Values{
		"action": {"delete"}, "name": {"service@h"},
	}
	req, err := http.NewRequest(http.MethodPost, m.web.URL+"/service-confirm", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(m.session)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", m.web.URL)
	resp, err = m.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	confirmation, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(confirmation), `</svg> Confirm removal</h1>`) || !strings.Contains(string(confirmation), `class=page-title-mark`) {
		t.Fatalf("confirmation page returned %d without its marked title", resp.StatusCode)
	}

	missing, status := getAs(t, m, "/service?name=missing@h")
	if status != http.StatusNotFound || !strings.Contains(missing, `</svg> No such name</h1>`) || !strings.Contains(missing, `class=page-title-mark`) {
		t.Fatal("problem page lost its marked visible title")
	}
}
