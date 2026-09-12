// agent-bus-token hands out a credential and does nothing else. It is what an
// ordinary user runs, and the forced command behind their key — the admin
// program is neither. See docs/09-setup.md#the-programs.
//
// Three ways in, one answer: the socket says who you are, an environment pair
// says it, or a key you hold proves it. The last one needs no sshd, which not
// every host runs. See docs/02-access.md#getting-a-token.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/keyproof"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

const usage = `agent-bus-token <user@realm> [--rotate] [--key <path>]

  --rotate  ask for a new credential; without it, asking twice is a read
  --key     prove the name with a key instead of a credential you already have

As a forced command in the agent-bus account's authorized_keys, the name in
the line is the only one that key may ask for.`

func main() {
	if err := issue(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func issue() error {
	entitled, rotate, key, err := parse(os.Args[1:])
	if err != nil {
		return err
	}
	// Over SSH the request arrives in the environment, not in argv: argv is
	// what the authorized_keys line said this key may have.
	asked, sshRotate, _, err := parse(strings.Fields(strings.TrimPrefix(os.Getenv("SSH_ORIGINAL_COMMAND"), "token")))
	if err != nil {
		return err
	}
	rotate = rotate || sshRotate
	name := entitled
	switch {
	case entitled == "" && asked == "":
		return fmt.Errorf("which name?\n\n%s", usage)
	case entitled == "":
		name = asked
	case asked != "" && asked != entitled:
		return fmt.Errorf("this key may ask for %s, not %s", entitled, asked)
	}
	if _, err := protocol.ParseName(name); err != nil {
		return err
	}
	tok := ""
	if key == "" {
		tok, err = ask(name, rotate)
	} else {
		tok, err = prove(name, key)
	}
	if err != nil {
		return err
	}
	fmt.Println(tok)
	return nil
}

// parse reads a name and the two flags out of one argument list, whether it
// came from argv or from what SSH was asked for.
func parse(args []string) (name string, rotate bool, key string, err error) {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--rotate":
			rotate = true
		case a == "--key" && i+1 < len(args):
			i++
			key = args[i]
		case strings.HasPrefix(a, "--key="):
			key = strings.TrimPrefix(a, "--key=")
		case a == "--key":
			key = keyproof.Default()
		case strings.HasPrefix(a, "-"):
			return "", false, "", fmt.Errorf("no such option %q\n\n%s", a, usage)
		case name != "":
			return "", false, "", fmt.Errorf("one name, not two\n\n%s", usage)
		default:
			name = a
		}
	}
	return name, rotate, key, nil
}

// ask is the ordinary path: the socket says who you are, or a token you
// already hold does. See docs/02-access.md#what-a-call-carries.
func ask(name string, rotate bool) (string, error) {
	out, code, err := post("/token", map[string]any{"name": name, "rotate": rotate})
	if err != nil {
		return "", err
	}
	if code >= 400 {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return tokenIn(out)
}

// prove is the path that needs nothing already: the bus says what to sign and
// checks the answer against what the realm publishes, which is the same
// challenge enrolment asks. See docs/01-identity.md#proving-possession.
func prove(name, key string) (string, error) {
	out, code, err := post("/enrol", map[string]string{"name": name})
	if err != nil {
		return "", err
	}
	if code >= 400 {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	var ask struct{ Name, Nonce, Namespace string }
	if err := json.Unmarshal(out, &ask); err != nil {
		return "", err
	}
	sig, err := keyproof.Sign(key, ask.Namespace, ask.Nonce)
	if err != nil {
		return "", err
	}
	out, code, err = post("/enrol", map[string]string{"nonce": ask.Nonce, "signature": sig})
	if err != nil {
		return "", err
	}
	if code >= 400 {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return tokenIn(out)
}

func tokenIn(out []byte) (string, error) {
	var got struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(out, &got); err != nil || got.Token == "" {
		return "", fmt.Errorf("no token in the answer: %s", strings.TrimSpace(string(out)))
	}
	return got.Token, nil
}

func post(path string, body any) ([]byte, int, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	client, base := api.Dial(os.Getenv("AGENT_BUS_ADDR"))
	req, err := http.NewRequest("POST", base+path, bytes.NewReader(b))
	if err != nil {
		return nil, 0, err
	}
	// Sent when it is there. On your own socket it is not needed, and the key
	// path does not have one yet — that is what it is for.
	if v := os.Getenv("AGENT_BUS_TOKEN"); v != "" {
		req.Header.Set(api.HeaderToken, v)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	return out, resp.StatusCode, err
}
