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
// sha256:<hex>, as agent-bus-token --fingerprint prints it
// (docs/14-remote-access.md#over-https).
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
func tlsClient(pin string) *http.Transport {
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
	return &http.Transport{TLSClientConfig: cfg, ForceAttemptHTTP2: true, IdleConnTimeout: 90 * time.Second}
}

func dialTLS(addr string) (*http.Client, string) {
	return &http.Client{Timeout: 2 * time.Minute, Transport: tlsClient(os.Getenv(PinEnv))}, strings.TrimSuffix(addr, "/")
}
