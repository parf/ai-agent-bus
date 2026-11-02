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
	cmd := exec.Command(binary)
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
	return info.Addr, info.Tokens, func() { input.Close(); cmd.Wait() }
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
