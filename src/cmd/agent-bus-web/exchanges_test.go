package main

import (
	"bytes"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"strings"
	"testing"
	"time"
)

func exchangeFixture() (protocol.Envelope, protocol.Envelope) {
	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	r := protocol.Envelope{ID: "original", From: "alice@h", To: "worker@h", Topic: "job", Tag: "tag", At: at, Deadline: at.Add(time.Second)}
	e := protocol.Envelope{ID: "receipt", From: "worker@h", To: "alice@h", Topic: "job", Tag: "tag", Receipt: "done", Re: r.ID, At: at.Add(2 * time.Second)}
	return r, e
}

func TestExchangeReceiptsKeepTheirOriginalAndRoute(t *testing.T) {
	r, done := exchangeFixture()
	retry := r
	retry.ID = "retry"
	retry.At = r.At.Add(time.Millisecond)
	other := r
	other.ID = "other"
	other.From = "bob@h"
	other.To = "second@h"
	ack := done
	ack.ID = "ack"
	ack.Receipt = "ack"
	ack.At = r.Deadline
	done.Re = "retry"
	redirected := r
	redirected.ID = "redirected"
	redirected.ReplyTo = &protocol.ReplyTo{Service: "third@h", Topic: "result", Tag: "new-label"}
	redirectedDone := done
	redirectedDone.ID = "redirected-done"
	redirectedDone.Re = redirected.ID
	redirectedDone.To = "third@h"
	redirectedDone.Topic = "result"
	redirectedDone.Tag = "new-label"
	feed := []protocol.Envelope{redirectedDone, done, ack, redirected, other, retry, r}
	rows := exchanges(feed)
	by := map[string]exchange{}
	total := 0
	for _, x := range rows {
		by[x.ID] = x
		total += x.N()
	}
	if len(rows) != 4 || total != len(feed) {
		t.Fatalf("lost or merged unrelated envelopes: %#v", rows)
	}
	if !by[r.ID].Ack || by[r.ID].Done || by[retry.ID].Ack || !by[retry.ID].Done || by[other.ID].Ack || by[other.ID].Done {
		t.Fatal("reused labels contaminated independent requests")
	}
	if len(by[redirected.ID].Receipts) != 1 || !by[redirected.ID].Done {
		t.Fatal("redirected receipt did not follow substituted return route")
	}
	if by[r.ID].Receipts[0].Late || !by[retry.ID].Receipts[0].Late {
		t.Fatal("late must compare referenced request deadline, strictly")
	}
	if got := by[retry.ID].Receipts[0]; got.ID != done.ID || got.Re != retry.ID {
		t.Fatal("receipt ID or reference discarded")
	}
	if rows[0].ID != "redirected" || rows[1].ID != "retry" {
		t.Fatal("unstable newest evidence ordering")
	}
}

func TestUnmatchedReceiptsNeverInventCompletion(t *testing.T) {
	r, valid := exchangeFixture()
	for _, tc := range []struct {
		name   string
		change func(*protocol.Envelope)
	}{
		{"sender", func(e *protocol.Envelope) { e.From = "intruder@h" }},
		{"recipient", func(e *protocol.Envelope) { e.To = "other@h" }},
		{"topic", func(e *protocol.Envelope) { e.Topic = "another" }},
		{"tag", func(e *protocol.Envelope) { e.Tag = "another" }},
		{"no reference", func(e *protocol.Envelope) { e.Re = "" }},
		{"missing original", func(e *protocol.Envelope) { e.Re = "not-retained" }},
		{"predates original", func(e *protocol.Envelope) { e.At = r.At.Add(-time.Second) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := valid
			tc.change(&e)
			xs := exchanges([]protocol.Envelope{e, r})
			if len(xs) != 2 {
				t.Fatalf("unsupported receipt merged: %#v", xs)
			}
			for _, x := range xs {
				if x.Done || x.Ack || x.Late || len(x.Receipts) != 0 {
					t.Fatalf("unverified completion or deadline: %#v", x)
				}
				if x.ID == e.ID && (x.Notice == "" || x.Re != e.Re) {
					t.Fatal("standalone receipt lost explanation/reference")
				}
			}
		})
	}
	duplicate := r
	duplicate.To = "other-worker@h"
	ambiguous := exchanges([]protocol.Envelope{valid, duplicate, r})
	if len(ambiguous) != 3 {
		t.Fatal("ambiguous reference selected an original")
	}
	for _, x := range ambiguous {
		if x.ID == valid.ID && !strings.Contains(x.Notice, "ambiguous") {
			t.Fatal("ambiguous reference not explained")
		}
	}
	second := valid
	second.ID = "receipt-of-receipt"
	second.Re = valid.ID
	second.From = valid.To
	second.To = valid.From
	second.At = valid.At.Add(time.Second)
	xs := exchanges([]protocol.Envelope{second, valid})
	if len(xs) != 2 {
		t.Fatal("receipt about receipt folded")
	}
	for _, x := range xs {
		if x.ID == second.ID && !strings.Contains(x.Notice, "itself a receipt") {
			t.Fatal("referenced receipt is visible but reported absent")
		}
		if x.Done || x.Ack {
			t.Fatal("receipt asserted completion without an ordinary original")
		}
	}
	for _, topic := range []string{"pubsub@h", "queue@h"} {
		request := r
		request.To = topic
		if len(exchanges([]protocol.Envelope{valid, request})) != 2 {
			t.Fatal("topic worker's claim treated as verified original receiver")
		}
	}
}

func TestOrdinaryResponsesRemainMessagesWithQualifiedRouteMatches(t *testing.T) {
	r, answer := exchangeFixture()
	answer.Receipt = ""
	answer.Re = ""
	answer.ID = "answer"
	r.ReplyTo = &protocol.ReplyTo{Service: "third@h", Topic: "output", Tag: "redirect"}
	answer.To = "third@h"
	answer.Topic = "output"
	answer.Tag = "redirect"
	check := func(feed []protocol.Envelope, want int, late bool) {
		t.Helper()
		xs := exchanges(feed)
		if len(xs) != len(feed) {
			t.Fatal("ordinary messages merged without explicit reference")
		}
		for _, x := range xs {
			if x.Done || x.Ack {
				t.Fatal("route match asserted completion/receipt")
			}
			if x.ID == answer.ID && (len(x.Matches) != want || x.Late != late) {
				t.Fatalf("wrong route/deadline evidence: %#v", x)
			}
		}
	}
	check([]protocol.Envelope{answer, r}, 1, true)
	retry := r
	retry.ID = "retry"
	retry.At = r.At.Add(time.Millisecond)
	retry.Deadline = r.At.Add(5 * time.Second)
	check([]protocol.Envelope{answer, retry, r}, 2, false)
	check([]protocol.Envelope{answer}, 0, false)
	other := r
	other.To = "different@h"
	check([]protocol.Envelope{answer, other}, 0, false)
	untagged := r
	untagged.ReplyTo = &protocol.ReplyTo{Service: "third@h", Topic: "output"}
	answer.Tag = ""
	check([]protocol.Envelope{answer, untagged}, 0, false)
}

func TestExchangeRenderingPreservesEvidenceAndMissingHistory(t *testing.T) {
	request, receipt := exchangeFixture()
	request.Topic = "<script>topic</script>"
	request.Body = "body-must-not-render"
	receipt.Topic = request.Topic
	orphan := receipt
	orphan.ID = "orphan-id"
	orphan.Re = "missing-id"
	renderView := func(v view) string {
		t.Helper()
		var out bytes.Buffer
		if err := page.Execute(&out, v); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	html := renderView(view{You: "owner@h", Exchanges: exchanges([]protocol.Envelope{orphan, receipt, request})})
	for _, text := range []string{request.ID, receipt.ID, "missing-id", "Original message not in this visible history", "Completion receipt observed", "about <code>original</code>", "2026-09-15 12:00:00Z", "History is bounded", "scope=col", "&lt;script&gt;topic&lt;/script&gt;"} {
		if !strings.Contains(html, text) {
			t.Errorf("rendered history lost %q", text)
		}
	}
	for _, secret := range []string{"body-must-not-render", "<script>topic</script>"} {
		if strings.Contains(html, secret) {
			t.Errorf("rendered unsafe content: %s", secret)
		}
	}
	answer := receipt
	answer.Receipt, answer.Re, answer.ID = "", "", "ordinary-answer"
	matched := renderView(view{You: "owner@h", Exchanges: exchanges([]protocol.Envelope{answer, request})})
	for _, text := range []string{"Possible response", `href="#message-original"`, "No completion receipt observed in retained history"} {
		if !strings.Contains(matched, text) {
			t.Errorf("ordinary response lost qualification %q", text)
		}
	}
	if strings.Contains(matched, "<p>Completion receipt observed.") {
		t.Fatal("route match rendered as completion")
	}
	if !strings.Contains(renderView(view{You: "owner@h"}), "No envelopes in your retained history") {
		t.Fatal("empty window not explained")
	}
	unavailable := renderView(view{You: "owner@h", NoFeed: "fixture unavailable"})
	if !strings.Contains(unavailable, "Envelope history unavailable") || strings.Contains(unavailable, "No envelopes in your retained history") {
		t.Fatal("failed load claimed empty history")
	}
}

func TestRedirectedExchangeEvidenceIsScopedToTheViewer(t *testing.T) {
	b := core.New()
	b.Administrator("owner@h")
	b.Masters([]string{"owner@h"})
	for _, name := range []string{"alice@h", "worker@h", "third@h"} {
		if _, err := b.Register(protocol.Record{Name: name, Owner: "owner@h", Allow: []string{"*"}}); err != nil {
			t.Fatal(err)
		}
	}
	request, err := b.Send(protocol.Envelope{From: "alice@h", To: "worker@h", Topic: "input", Tag: "one", ReplyTo: &protocol.ReplyTo{Service: "third@h", Topic: "output", Tag: "two"}})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := b.Send(protocol.Envelope{From: "worker@h", To: "third@h", Topic: "output", Tag: "two", Receipt: "done", Re: request.ID})
	if err != nil {
		t.Fatal(err)
	}
	master := exchanges(b.Recent("owner@h"))
	if len(master) != 1 || !master[0].Done {
		t.Fatal("master with both envelopes lost correlation")
	}
	requester := exchanges(b.Recent("alice@h"))
	if len(requester) != 1 || requester[0].ID != request.ID || requester[0].Done {
		t.Fatal("requester's partial history invented completion")
	}
	third := exchanges(b.Recent("third@h"))
	if len(third) != 1 || third[0].ID != receipt.ID || third[0].Done || third[0].Re != request.ID || third[0].Notice == "" {
		t.Fatal("third-party partial history invented original or lost reference")
	}
	if len(exchanges(b.Recent("outsider@h"))) != 0 {
		t.Fatal("unrelated visitor got exchange")
	}
}
