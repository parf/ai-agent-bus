package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// post submits a form as the fixture's signed-in caller, with the exact Origin
// the dashboard requires of every mutation.
func (m *meanings) post(t *testing.T, path string, form url.Values, want int) (string, http.Header) {
	t.Helper()
	req, err := http.NewRequest("POST", m.web.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", m.web.URL)
	req.AddCookie(m.session)
	resp, err := m.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("POST %s: %d want %d: %s", path, resp.StatusCode, want, body)
	}
	return strings.Join(strings.Fields(string(body)), " "), resp.Header
}

// Registering a service and storing its credential are two daemon verbs, so
// the one form is two calls. What matters is that the bytes arrive unchanged
// and never come back: a registration carries neither the secret nor its
// digest (docs/06-services.md#secrets), and no page shows the bytes again.
func TestRegisteringAServiceStoresTheSecretItWasGivenAndNeverShowsItAgain(t *testing.T) {
	m := meaningFixture(t)
	page := m.get("/services/new")
	if !strings.Contains(page, "<textarea name=secret rows=4 autocomplete=off") {
		t.Fatalf("service registration offers no secret field: %s", page)
	}
	// Only where a secret means anything. A kind reached by sending to its
	// name has nothing outside to authenticate to, and the daemon refuses a
	// secret on one, so a form that offered the field would be offering a
	// refusal.
	for _, path := range []string{"/agents/new", "/channels/new?kind=queue", "/channels/new?kind=pubsub"} {
		if strings.Contains(m.get(path), "name=secret") {
			t.Errorf("%s offers a secret to a kind that cannot hold one", path)
		}
	}

	// Two lines, submitted the way a browser submits a textarea. The stored
	// bytes must be what was typed, not what the wire carried.
	typed := "PGPASSWORD=one\nPGUSER=two"
	form := url.Values{
		"action": {"create"}, "name": {"vault@h"}, "kind": {protocol.KindService},
		"addr": {"db:5432"}, "protocol": {"postgresql"},
		"secret": {strings.ReplaceAll(typed, "\n", "\r\n")},
	}
	if _, header := m.post(t, "/service", form, 303); header.Get("Location") != "/service?name=vault%40h" {
		t.Fatalf("registration returned to %q", header.Get("Location"))
	}
	stored, err := m.bus.Secret("vault@h", "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	if stored != typed {
		t.Errorf("the stored secret is not the bytes that were typed: %q want %q", stored, typed)
	}

	// The record page states the digest and nothing else of it, and the digest
	// is of those bytes rather than of whatever the wire carried.
	detail := m.get("/service?name=vault@h")
	if strings.Contains(detail, "PGPASSWORD") || strings.Contains(detail, "PGUSER") {
		t.Errorf("the record page shows the secret back: %s", detail)
	}
	record, ok := m.bus.Lookup("admin@h", "vault@h")
	if !ok || record.SecretSHA == "" {
		t.Fatalf("no digest to compare the page against: %+v", record)
	}
	if !strings.Contains(detail, record.SecretSHA) {
		t.Errorf("the record page does not state the digest of what was stored: %s", detail)
	}

	// A service registered with the field left empty holds no secret, rather
	// than holding an empty one the daemon would have refused.
	empty := url.Values{
		"action": {"create"}, "name": {"keyless@h"}, "kind": {protocol.KindService},
		"addr": {"api:443"}, "protocol": {"https"}, "secret": {""},
	}
	m.post(t, "/service", empty, 303)
	if record, ok := m.bus.Lookup("admin@h", "keyless@h"); !ok || record.SecretSHA != "" {
		t.Errorf("an empty field stored something: %+v", record)
	}
}

// A refused registration returns the form with what was typed in it — except
// the credential, which is still a credential after it is refused.
func TestARefusedRegistrationKeepsItsFieldsAndNotItsSecret(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Kind: protocol.KindService, Name: "taken@h", Owner: "admin@h", Addr: "a:1", Proto: "https"})
	body, _ := m.post(t, "/service", url.Values{
		"action": {"create"}, "name": {"taken@h"}, "kind": {protocol.KindService},
		"addr": {"db:5432"}, "protocol": {"postgresql"}, "descr": {"kept"},
		"secret": {"PGPASSWORD=must-not-return"},
	}, http.StatusPreconditionFailed)
	if !strings.Contains(body, "kept") || !strings.Contains(body, "db:5432") {
		t.Errorf("the refused form lost what was typed: %s", body)
	}
	if strings.Contains(body, "must-not-return") {
		t.Errorf("the refused form handed the credential back in HTML: %s", body)
	}
}

// A queue's form declares the policy of the queue it is about to hold, so the
// registration has to carry it. Showing the fields and dropping them would
// leave a queue on the defaults with a page that said otherwise.
func TestRegisteringAQueueCarriesTheQueuePolicyItDeclared(t *testing.T) {
	m := meaningFixture(t)
	m.post(t, "/service", url.Values{
		"action": {"create"}, "name": {"jobs@h"}, "kind": {protocol.KindQueue},
		"ttl": {"90s"}, "bound": {"7"}, "overflow": {"ring"},
	}, 303)
	record, ok := m.bus.Lookup("admin@h", "jobs@h")
	if !ok {
		t.Fatal("the queue was not registered")
	}
	// Each asserted against the value that was typed, and each different from
	// the default it would fall back to.
	for _, want := range []struct{ field, got, declared string }{
		{"TTL", record.TTL, "90s"},
		{"overflow", record.Full, protocol.OverflowRing},
	} {
		if want.got != want.declared {
			t.Errorf("the queue's %s is %q, not the %q its form declared", want.field, want.got, want.declared)
		}
	}
	if record.Bound != 7 {
		t.Errorf("the queue's capacity is %d, not the 7 its form declared", record.Bound)
	}

	// A capacity that is not a number is refused before anything is sent, and
	// the form comes back with what was typed in it.
	bad, _ := m.post(t, "/service", url.Values{
		"action": {"create"}, "name": {"nope@h"}, "kind": {protocol.KindQueue},
		"bound": {"lots"}, "descr": {"kept through the refusal"},
	}, 400)
	if !strings.Contains(bad, "Queue capacity must be a whole number.") || !strings.Contains(bad, "kept through the refusal") {
		t.Errorf("an unparseable capacity did not return the form with its values: %s", bad)
	}
	if _, registered := m.bus.Lookup("admin@h", "nope@h"); registered {
		t.Error("a refused capacity registered the queue anyway")
	}

	// A pub/sub topic has no queue, so its form declares none and the
	// registration carries none — the other half of the same rule.
	m.post(t, "/service", url.Values{
		"action": {"create"}, "name": {"shout@h"}, "kind": {protocol.KindPubSub},
	}, 303)
	if topic, ok := m.bus.Lookup("admin@h", "shout@h"); !ok || topic.TTL != "" || topic.Bound != 0 {
		t.Errorf("a topic came out carrying queue policy: %+v", topic)
	}
}

// The settings form offers the credential too, because it is the same form:
// a service registered without one, or with the wrong one, is changed where
// every other field of it is changed. Empty leaves the stored bytes alone,
// which is the only thing a field that is never filled in can mean.
// See docs/06-services.md#secrets.
func TestTheSettingsFormStoresASecretAndAnEmptyFieldKeepsTheStoredOne(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "vault@h", Kind: protocol.KindService, Owner: "admin@h", Addr: "host:1", Proto: "https", Allow: []string{"*"}})
	settings := url.Values{
		"action": {"save"}, "name": {"vault@h"}, "descr": {"Vault"},
		"addr": {"host:1"}, "protocol": {"https"},
		"edit_allow": {"1"}, "allow": {"*"},
	}
	first := url.Values{}
	for k, v := range settings {
		first[k] = v
	}
	first.Set("secret", "TOKEN=first\r\nSECOND=two")
	m.post(t, "/service", first, 303)
	stored, err := m.bus.Secret("vault@h", "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "TOKEN=first\nSECOND=two" {
		t.Fatalf("the stored bytes are %q", stored)
	}

	// A save that leaves the field empty is every other field changing.
	kept := url.Values{}
	for k, v := range settings {
		kept[k] = v
	}
	kept.Set("descr", "Vault, renamed")
	kept.Set("secret", "")
	m.post(t, "/service", kept, 303)
	if r, _ := m.bus.Lookup("admin@h", "vault@h"); r.Descr != "Vault, renamed" {
		t.Fatalf("the description did not change: %q", r.Descr)
	}
	if stored, err = m.bus.Secret("vault@h", "admin@h"); err != nil || stored != "TOKEN=first\nSECOND=two" {
		t.Fatalf("an empty field changed the stored credential to %q (%v)", stored, err)
	}

	// And the page never shows it again, before or after.
	editor := m.get("/service/edit?name=vault@h")
	if strings.Contains(editor, "TOKEN=first") || strings.Contains(editor, "SECOND=two") {
		t.Fatal("the settings form filled the secret field back in")
	}
	if !strings.Contains(editor, "<textarea name=secret rows=4 autocomplete=off") {
		t.Fatal("the settings form does not offer a secret field at all")
	}
}
