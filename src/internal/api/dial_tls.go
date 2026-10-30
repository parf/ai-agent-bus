package api

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/tlsdir"
)

// PinEnv names the fingerprint a client pins the daemon's certificate to:
// sha256:<hex>, as agent-bus token --fingerprint prints it
// (docs/02-access-remote.md#over-https).
const PinEnv = "AGENT_BUS_TLS_FINGERPRINT"

// IsTCP says whether addr is the daemon's TCP port, plain or TLS, rather
// than a unix socket path.
func IsTCP(addr string) bool {
	return strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://")
}

// tlsClient reaches an https:// daemon. With a pin, it trusts exactly the
// certificate whose fingerprint that is — a self-signed one included — and no
// other; without one, the system's roots decide. A refusal is an error, never
// a quiet retry over plain HTTP.
func tlsClient(pin string, h2 bool) *http.Transport {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if pin = strings.TrimSpace(pin); pin != "" {
		// The chain is not checked because the pin replaces it: the one
		// certificate that may answer is named, so neither a CA nor a
		// hostname adds anything.
		cfg.InsecureSkipVerify = true
		cfg.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("the daemon presented no certificate")
			}
			got := tlsdir.FingerprintDER(cs.PeerCertificates[0].Raw)
			if !tlsdir.SamePin(pin, got) {
				return fmt.Errorf("the daemon's certificate is %s, not the pinned %s (%s)", got, pin, PinEnv)
			}
			return nil
		}
	}
	t := &http.Transport{TLSClientConfig: cfg, ForceAttemptHTTP2: h2, IdleConnTimeout: 90 * time.Second}
	if !h2 {
		// A non-nil, empty table is what keeps the transport from offering
		// h2 at all, whatever else it is configured with.
		t.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	}
	return t
}

// splitTransport sends a long poll (consume) over HTTP/1.1 and every other
// call over whatever the daemon and the client negotiate, HTTP/2 included. An
// HTTP/2 poll that is cancelled is a stream reset the server learns of only
// when it reads the frame, and a message sent in between goes to the gone
// reader (J41); a cancelled HTTP/1.1 poll is a closed connection the daemon
// can see at once (J39).
type splitTransport struct{ poll, rest http.RoundTripper }

// CloseIdleConnections reaches both transports, so http.Client's own call
// closes the idle connections of the poll pool and the rest alike.
func (s splitTransport) CloseIdleConnections() {
	for _, t := range []http.RoundTripper{s.poll, s.rest} {
		if c, ok := t.(interface{ CloseIdleConnections() }); ok {
			c.CloseIdleConnections()
		}
	}
}

func (s splitTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Path == "/consume" {
		return s.poll.RoundTrip(r)
	}
	return s.rest.RoundTrip(r)
}

func dialTLS(addr string) (*http.Client, string) {
	return &http.Client{Timeout: 2 * time.Minute, Transport: splitTransport{poll: tlsClient(os.Getenv(PinEnv), false), rest: tlsClient(os.Getenv(PinEnv), true)}}, strings.TrimSuffix(addr, "/")
}
