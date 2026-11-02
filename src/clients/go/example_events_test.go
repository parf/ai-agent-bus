package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// exampleFixture starts the private fixture daemon an example runs against
// and returns its address, tokens and a stop function. It is never the live bus.
func exampleFixture() (string, map[string]string, func()) {
	binary := os.Getenv("AB_CLIENT_FIXTURE")
	if binary == "" {
		binary = filepath.Join("..", "client-fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	cmd := exec.CommandContext(ctx, binary)
	cmd.WaitDelay = time.Second
	input, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err = cmd.Start(); err != nil {
		panic(err)
	}
	var info struct {
		Addr   string            `json:"addr"`
		Tokens map[string]string `json:"tokens"`
	}
	if err = json.NewDecoder(out).Decode(&info); err != nil {
		panic(err)
	}
	return info.Addr, info.Tokens, func() { input.Close(); cmd.Wait(); cancel() }
}

// An agent writes to its own private event log, its Owner reads and lists it,
// and nobody else may. Every call goes to the daemon's own #bus agent.
func Example_eventLog() {
	addr, tokens, stop := exampleFixture()
	defer stop()
	dial := func(name string) *Bus {
		b, err := Dial(Options{Addr: addr, Token: tokens[name], ExpectedName: name, Timeout: 5 * time.Second})
		if err != nil {
			panic(err)
		}
		return b
	}
	agent, owner, stranger := dial("#caller@fixture"), dial("owner@fixture"), dial("#outsider@fixture")
	defer agent.Close()
	defer owner.Close()
	defer stranger.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// call waits for the answer; a send without wait would be accepted and
	// never answered, so it proves only that the message was queued.
	call := func(b *Bus, body string) map[string]any {
		out, err := b.Call(ctx, "#bus", body)
		if err != nil || !out.Answered {
			panic(fmt.Sprint("no answer to ", body, ": ", err))
		}
		var reply map[string]any
		if err = json.Unmarshal([]byte(out.Message.Body), &reply); err != nil {
			panic(err)
		}
		return reply
	}
	code := func(reply map[string]any) any { return reply["error"].(map[string]any)["code"] }

	entry := call(agent, "write #caller@fixture alert disk full")["entry"].(map[string]any)
	fmt.Println("written:", entry["level"], entry["text"])

	read := call(owner, "read #caller@fixture")
	alerts := read["rings"].(map[string]any)["alert"].([]any)
	fmt.Println("owner reads:", len(alerts), "alert, newest", alerts[0].(map[string]any)["text"])

	logs := call(owner, "list")["logs"].([]any)
	fmt.Println("owner lists:", logs[0].(map[string]any)["log"])

	fmt.Println("a third party:", code(call(stranger, "read #caller@fixture")))
	fmt.Println("the writer reading its own log:", code(call(agent, "read #caller@fixture")))
	// Output:
	// written: alert disk full
	// owner reads: 1 alert, newest disk full
	// owner lists: #caller@fixture
	// a third party: forbidden
	// the writer reading its own log: forbidden
}

// A subscription is a normal #bus call, and its copies are non-waiting
// notices consumed from the authorized destination. They need no ack/done.
func Example_eventSubscription() {
	addr, tokens, stop := exampleFixture()
	defer stop()
	dial := func(name string) *Bus {
		b, err := Dial(Options{Addr: addr, Token: tokens[name], ExpectedName: name, Timeout: 5 * time.Second})
		if err != nil {
			panic(err)
		}
		return b
	}
	owner, agent := dial("owner@fixture"), dial("#caller@fixture")
	defer owner.Close()
	defer agent.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	call := func(b *Bus, body string) map[string]any {
		out, err := b.Call(ctx, "#bus", body, WithTimeout(5*time.Second))
		if err != nil || !out.Answered || out.Message == nil {
			panic(fmt.Sprint("no answer to ", body, ": ", err))
		}
		var reply map[string]any
		if err := json.Unmarshal([]byte(out.Message.Body), &reply); err != nil {
			panic(err)
		}
		if reply["error"] != nil {
			panic(fmt.Sprint("refused: ", reply))
		}
		return reply
	}
	row := call(owner, "sub add jobs@fixture #caller@fixture alert")
	id, ok := row["id"].(string)
	if !ok || id == "" {
		panic("sub add returned no id")
	}
	listed := call(owner, "sub list #caller@fixture")["subscriptions"].([]any)
	if len(listed) != 1 || listed[0].(map[string]any)["id"] != id {
		panic("subscription not listed")
	}
	fmt.Println("listed:", len(listed), "subscription")

	write := call(agent, "write #caller@fixture alert disk full")
	if write["deliveries"].(map[string]any)["sent"] != float64(1) {
		panic("expected one delivered copy")
	}
	copy, err := owner.Consume(ctx, WithInbox("jobs@fixture"), WithWait(300*time.Millisecond))
	if err != nil || copy == nil {
		panic(fmt.Sprint("no subscription copy: ", err))
	}
	if copy.From != "#bus" || copy.Topic != "#caller@fixture" || copy.Tag != "alert" {
		panic("wrong copy route")
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(copy.Body), &entry); err != nil {
		panic(err)
	}
	if entry["text"] != "disk full" {
		panic("wrong copy body")
	}
	fmt.Println("copy from", copy.From+":", entry["level"], entry["text"])
	if copy.Wait != "" || copy.Receipt != "" || copy.Re != "" {
		panic("copy is not a non-waiting notice")
	}
	// This is a notice. Nothing is acknowledged or marked done.
	fmt.Println("receipt: none for a notice")
	if extra, err := owner.Consume(ctx, WithInbox("jobs@fixture"), WithWait(300*time.Millisecond)); err != nil || extra != nil {
		panic("more than one copy")
	}

	if call(owner, "sub delete "+id)["deleted"] != true {
		panic("subscription was not deleted")
	}
	write = call(agent, "write #caller@fixture alert after cancellation")
	if write["deliveries"].(map[string]any)["matched"] != float64(0) {
		panic("cancelled subscription still matched")
	}
	if extra, err := owner.Consume(ctx, WithInbox("jobs@fixture"), WithWait(300*time.Millisecond)); err != nil || extra != nil {
		panic("copy after cancellation")
	}
	fmt.Println("cancelled: no further copies")
	// Output:
	// listed: 1 subscription
	// copy from #bus: alert disk full
	// receipt: none for a notice
	// cancelled: no further copies
}
