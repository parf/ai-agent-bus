// agent-bus token hands out a credential and does nothing else. It is what an
// ordinary user runs, and the forced command behind their key — the admin
// program is neither. See docs/07-setup.md#the-programs.
//
// Three ways in, one answer: the socket says who you are, an environment pair
// says it, or a key you hold proves it. The last one needs no sshd, which not
// every host runs. See docs/02-access.md#getting-a-token.
package tokencmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/keyproof"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/tlsdir"
	"github.com/parf/ai-agent-bus/internal/version"
)

// usage opens with the program and its version, like every agent-bus program.
var usage = "agent-bus token " + version.String + "\n\n" + display.ManText(usageText)

const usageText = `USAGE
  agent-bus token [<name>]

DESCRIPTION
  <name> is an optional User or #agent, with an optional realm. Without it,
  use the login socket principal. Asking again reads the existing credential;
  --rotate creates a new one. --key proves possession with a private SSH key.
  --fingerprint prints the public TLS certificate pin instead of a credential.

OPTIONS
  -h, --help  Show this command help
  --rotate  Rotate the credential
  --key=file  SSH private key (default $HOME/.ssh/id_ed25519)
  --fingerprint  Print the TLS certificate pin

NAMES
  With no name, use the login socket principal. A forced SSH command
  may request only its entitled principal's credential.

EXAMPLES
  agent-bus token > private.token
  agent-bus token --fingerprint

SEE ALSO
  ` + display.SourceURL + `/docs/01-identity-and-authority.md#names`

// Run is the token command only: SSH input is never a CLI verb.
func Run(args []string) error {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		fmt.Printf("%s\nbuild_info: %s\n", version.String, version.Build)
		return nil
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Println(usage)
		return nil
	}
	return issue(args)
}

func issue(args []string) error {
	request := strings.Fields(os.Getenv("SSH_ORIGINAL_COMMAND"))
	if len(request) > 0 && request[0] == "token" {
		request = request[1:]
	}
	fingerprint := len(args) == 1 && args[0] == "--fingerprint"
	sshFingerprint := len(request) == 1 && request[0] == "--fingerprint"
	var entitled, asked, key string
	var rotate, sshRotate bool
	var err error
	if !fingerprint {
		entitled, rotate, key, err = parse(protocol.ExpandAgentFlags(args))
		if err != nil {
			return err
		}
	}
	// Validate the entire SSH request before the public fingerprint branch too.
	if !sshFingerprint {
		asked, sshRotate, _, err = parse(request)
		if err != nil {
			return err
		}
	}
	if fingerprint && len(request) > 0 && !sshFingerprint {
		return fmt.Errorf("--fingerprint takes no other arguments")
	}
	if fingerprint || sshFingerprint {
		return printFingerprint()
	}
	rotate = rotate || sshRotate
	name := entitled
	switch {
	case entitled == "" && asked == "":
		name, err = loginPrincipal()
		if err != nil {
			return err
		}
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

// The fingerprint is public, but reached only after validating all arguments.
func printFingerprint() error {
	cert, err := tlsdir.LoadCert(tlsdir.Installed())
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("this node has no TLS certificate: %w", err)
		}
		return fmt.Errorf("cannot read this node's TLS certificate: %w", err)
	}
	fmt.Println(tlsdir.Fingerprint(cert))
	return nil
}

// Resolve only a login socket; environment names never select a principal.
func loginPrincipal() (string, error) {
	const instruction = "no name given and no login socket: pass <name>"
	addr := os.Getenv("AGENT_BUS_ADDR")
	if addr == "" {
		addr = api.ClientSocket()
	}
	if api.IsTCP(addr) || !api.IsUserSocket(addr) {
		return "", fmt.Errorf("%s", instruction)
	}
	client, base := api.Dial(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/status", nil)
	if err != nil {
		return "", fmt.Errorf("%s", instruction)
	}
	if token := os.Getenv("AGENT_BUS_TOKEN"); token != "" {
		req.Header.Set(api.HeaderToken, token)
	}
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s", instruction)
	}
	defer response.Body.Close()
	var status struct {
		You string `json:"you"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&status) != nil {
		return "", fmt.Errorf("%s", instruction)
	}
	if _, err := protocol.ParseName(status.You); err != nil {
		return "", fmt.Errorf("%s", instruction)
	}
	return status.You, nil
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
// challenge enrolment asks. See docs/02-access.md#proving-possession.
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
		return "", fmt.Errorf("no token in the answer")
	}
	return got.Token, nil
}

func post(path string, body any) ([]byte, int, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	addr := os.Getenv("AGENT_BUS_ADDR")
	if addr == "" {
		addr = api.ClientSocket()
	}
	client, base := api.Dial(addr)
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
