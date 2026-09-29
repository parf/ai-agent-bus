package main

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/tlsdir"
)

// tlsOwner owns the TLS directory and its files: root, in every real run; a
// test sets its own uid, being unable to chown to root.
var tlsOwner = 0

// tlsChoice is what setup was asked to do with TLS on the daemon's port
// (docs/09-setup.md#tls). An empty mode keeps what the node has.
type tlsChoice struct {
	mode             string // "", "off", "self-signed" or "files"
	cert, key, chain string
	names            list
	dir              string
}

func tlsFlags(fs *flag.FlagSet) *tlsChoice {
	c := &tlsChoice{dir: tlsdir.Installed()}
	fs.StringVar(&c.mode, "tls", "", "TLS on the daemon's port beside plain HTTP: `off`, self-signed or files; without it setup keeps what the node has, and asks at a terminal")
	fs.StringVar(&c.cert, "tls-cert", "", "with --tls files: the certificate `file`")
	fs.StringVar(&c.key, "tls-key", "", "with --tls files: its private key `file`")
	fs.StringVar(&c.chain, "tls-chain", "", "with --tls files: the intermediate certificates `file`, optional")
	fs.Var(&c.names, "tls-name", "with --tls self-signed: another host `name` or IP the certificate covers; repeatable")
	return c
}

func (c *tlsChoice) check() error {
	switch c.mode {
	case "", "off", "self-signed":
		if c.cert != "" || c.key != "" || c.chain != "" {
			return errors.New("--tls-cert, --tls-key and --tls-chain go with --tls files")
		}
	case "files":
		if c.cert == "" || c.key == "" {
			return errors.New("--tls files needs --tls-cert and --tls-key")
		}
	default:
		return fmt.Errorf("--tls is off, self-signed or files, not %q", c.mode)
	}
	if len(c.names) > 0 && c.mode != "self-signed" {
		return errors.New("--tls-name goes with --tls self-signed")
	}
	return nil
}

// offMarker in the TLS directory records that --tls off was asked for, so a
// later run without --tls keeps the node plain rather than reading the files
// still there as on.
const offMarker = "disabled"

func (c *tlsChoice) turnedOff() bool {
	_, err := os.Stat(filepath.Join(c.dir, offMarker))
	return err == nil
}

// terminal says whether a person is at stdin to answer questions.
func terminal() bool {
	st, err := os.Stdin.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// ask settles an empty mode with a person at the terminal. A node that
// already serves TLS keeps it without a question.
func (c *tlsChoice) ask(in io.Reader, out io.Writer) error {
	if c.mode != "" || tlsdir.Present(c.dir) || c.turnedOff() {
		return nil
	}
	r := bufio.NewReader(in)
	line := func(q string) string {
		fmt.Fprint(out, q)
		s, _ := r.ReadString('\n')
		return strings.TrimSpace(s)
	}
	if a := strings.ToLower(line("Enable SSL on the daemon's port? [y/N] ")); a != "y" && a != "yes" {
		c.mode = "off"
		return nil
	}
	if a := strings.ToLower(line("Generate a self-signed certificate, or use your own files? [G/f] ")); a == "f" || a == "files" {
		c.mode = "files"
		c.cert = line("  certificate file: ")
		c.key = line("  private key file: ")
		c.chain = line("  chain file (Enter for none): ")
		return c.check()
	}
	c.mode = "self-signed"
	return nil
}

// enabled says whether the unit is to carry -tls-dir after this run.
func (c *tlsChoice) enabled() bool {
	switch c.mode {
	case "self-signed", "files":
		return true
	case "off":
		return false
	}
	return tlsdir.Present(c.dir) && !c.turnedOff()
}

func (c *tlsChoice) step() string {
	switch c.mode {
	case "self-signed":
		return fmt.Sprintf("generate a self-signed TLS certificate for %s into %s, the key the daemon account's and root's alone", strings.Join(certNames(c.names), ", "), c.dir)
	case "files":
		return fmt.Sprintf("check %s and its key (and chain) and copy them into %s", c.cert, c.dir)
	case "off":
		return "serve plain HTTP only on the daemon's port, and mark " + c.dir + " off so later runs keep it so; its files are left as they are"
	}
	if c.turnedOff() {
		return "keep TLS off, as an earlier --tls off asked"
	}
	if tlsdir.Present(c.dir) {
		return "keep TLS from " + c.dir
	}
	return ""
}

// certNames is what a generated certificate covers: this host by every name
// and address a client might use, and whatever --tls-name adds.
func certNames(extra []string) []string {
	names := []string{"localhost", "127.0.0.1", "::1"}
	if h, err := os.Hostname(); err == nil && h != "" {
		names = append(names, h)
		if short, _, ok := strings.Cut(h, "."); ok {
			names = append(names, short)
		}
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() && !ipn.IP.IsLinkLocalUnicast() {
				names = append(names, ipn.IP.String())
			}
		}
	}
	names = append(names, extra...)
	seen := map[string]bool{}
	out := names[:0]
	for _, n := range names {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// selfSigned is a certificate and key for names: ECDSA P-256, ten years, since
// clients pin it by fingerprint rather than trusting it for its dates.
func selfSigned(names []string) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, nil, err
	}
	host, _ := os.Hostname()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "agent-busd " + host},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	kder, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kder}), nil
}

// provided reads and checks the operator's files: the key must be the
// certificate's, the chain must be certificates, and the leaf must not have
// expired. A leaf close to expiry is only warned about.
func provided(c *tlsChoice, warn io.Writer) (certPEM, chainPEM, keyPEM []byte, err error) {
	if certPEM, err = os.ReadFile(c.cert); err != nil {
		return nil, nil, nil, err
	}
	if keyPEM, err = os.ReadFile(c.key); err != nil {
		return nil, nil, nil, err
	}
	if c.chain != "" {
		if chainPEM, err = os.ReadFile(c.chain); err != nil {
			return nil, nil, nil, err
		}
	}
	cert, err := tlsdir.Pair(certPEM, chainPEM, keyPEM)
	if err != nil {
		return nil, nil, nil, err
	}
	if left := time.Until(cert.Leaf.NotAfter); left <= 0 {
		return nil, nil, nil, fmt.Errorf("%s expired %s", c.cert, cert.Leaf.NotAfter.Format(time.DateOnly))
	} else if left < 30*24*time.Hour {
		fmt.Fprintf(warn, "warning: %s expires %s\n", c.cert, cert.Leaf.NotAfter.Format(time.DateOnly))
	}
	return certPEM, chainPEM, keyPEM, nil
}

// install writes the chosen certificate into the TLS directory, owned by root
// and readable by the daemon's account: the certificate and chain by anyone,
// the key by root and that account alone. It answers the fingerprint served.
func (c *tlsChoice) install(account string, warn io.Writer) (string, error) {
	var certPEM, chainPEM, keyPEM []byte
	var err error
	// Only root can put anything at this path, but a link there would have
	// every write below go through it: a real directory or nothing.
	if st, err := os.Lstat(c.dir); err == nil && !st.IsDir() {
		return "", fmt.Errorf("%s is not a directory (%s); setup writes TLS files only into a real one", c.dir, st.Mode().Type())
	}
	switch c.mode {
	case "self-signed":
		certPEM, keyPEM, err = selfSigned(certNames(c.names))
	case "files":
		certPEM, chainPEM, keyPEM, err = provided(c, warn)
	case "off":
		if tlsdir.Present(c.dir) {
			return "", os.WriteFile(filepath.Join(c.dir, offMarker), []byte("--tls off\n"), 0o644)
		}
		return "", nil
	default:
		return "", nil
	}
	if err != nil {
		return "", err
	}
	gid := -1
	if g, err := user.LookupGroup(account); err == nil {
		gid, _ = strconv.Atoi(g.Gid)
	} else {
		return "", fmt.Errorf("the daemon account's group %s: %w", account, err)
	}
	// The parents are walked through by the daemon's account, so a parent
	// setup creates is 0755; one that already exists is left as it is.
	parent := filepath.Dir(c.dir)
	_, statErr := os.Stat(parent)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	// MkdirAll honours the umask; a parent made here is set, not asked for.
	if errors.Is(statErr, os.ErrNotExist) {
		if err := os.Chmod(parent, 0o755); err != nil {
			return "", err
		}
	}
	if err := os.Mkdir(c.dir, 0o750); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	if err := os.Chown(c.dir, tlsOwner, gid); err != nil {
		return "", err
	}
	if err := os.Chmod(c.dir, 0o750); err != nil {
		return "", err
	}
	files := []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{tlsdir.CertFile, certPEM, 0o644},
		{tlsdir.KeyFile, keyPEM, 0o640},
		{tlsdir.ChainFile, chainPEM, 0o644},
	}
	for _, f := range files {
		path := filepath.Join(c.dir, f.name)
		if f.name == tlsdir.ChainFile && len(f.data) == 0 {
			// A chain from an earlier choice would be appended to this
			// certificate, so it goes when this one has none.
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
			continue
		}
		// Written aside and renamed, so the daemon never reads half a key.
		tmp := path + ".new"
		if err := os.WriteFile(tmp, f.data, f.mode); err != nil {
			return "", err
		}
		if err := os.Chown(tmp, tlsOwner, gid); err != nil {
			return "", err
		}
		if err := os.Chmod(tmp, f.mode); err != nil {
			return "", err
		}
		if err := os.Rename(tmp, path); err != nil {
			return "", err
		}
	}
	if err := os.Remove(filepath.Join(c.dir, offMarker)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	cert, err := tlsdir.Load(c.dir)
	if err != nil {
		return "", err
	}
	return tlsdir.Fingerprint(cert), nil
}
