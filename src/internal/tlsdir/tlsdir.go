// Package tlsdir is the daemon's TLS directory: a certificate, its key and an
// optional chain, and the fingerprint clients pin. Setup writes it, the daemon
// serves from it, and the token helper and admin read its fingerprint.
// See docs/07-setup.md#tls.
package tlsdir

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Dir is where setup keeps it on an installed node.
const Dir = "/etc/agent-bus/tls"

const (
	CertFile  = "cert.pem"
	KeyFile   = "key.pem"
	ChainFile = "chain.pem"
)

// Installed is the directory this node's programs read: Dir, or
// AGENT_BUS_TLS_DIR when a test or a development daemon keeps it elsewhere.
func Installed() string {
	if v := os.Getenv("AGENT_BUS_TLS_DIR"); v != "" {
		return v
	}
	return Dir
}

// Describe is what an operator is shown about the served certificate.
func Describe(cert tls.Certificate) string {
	l := cert.Leaf
	names := append([]string{}, l.DNSNames...)
	for _, ip := range l.IPAddresses {
		names = append(names, ip.String())
	}
	self := "self-signed"
	if len(cert.Certificate) > 1 || l.Issuer.String() != l.Subject.String() {
		self = fmt.Sprintf("issued by %s, %d certificates in the chain", l.Issuer.CommonName, len(cert.Certificate))
	}
	return fmt.Sprintf("fingerprint %s\nnames       %s\nexpires     %s\nissuer      %s\n",
		Fingerprint(cert), strings.Join(names, ", "), l.NotAfter.Format("2006-01-02"), self)
}

// Present says whether dir holds a certificate and a key. It does not say
// they are valid; Load does.
func Present(dir string) bool {
	for _, f := range []string{CertFile, KeyFile} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return false
		}
	}
	return true
}

// Load reads the certificate, appends the chain when there is one, and pairs
// them with the key: the key must be the certificate's.
func Load(dir string) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(filepath.Join(dir, CertFile))
	if err != nil {
		return tls.Certificate{}, err
	}
	// A key anybody on the host can read is no longer the daemon's alone,
	// and serving it would be a secret in the open: refused, not warned.
	keyPath := filepath.Join(dir, KeyFile)
	info, err := os.Stat(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	if info.Mode().Perm()&0o007 != 0 {
		return tls.Certificate{}, fmt.Errorf("%s is readable by everyone (mode %04o); it must be the daemon account's and root's alone, 0640 or tighter", keyPath, info.Mode().Perm())
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	chainPEM, err := os.ReadFile(filepath.Join(dir, ChainFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return tls.Certificate{}, err
	}
	return Pair(certPEM, chainPEM, keyPEM)
}

// LoadCert reads the certificate and its chain, never the key: what the
// fingerprint and a description of the certificate need, which any user may
// learn.
func LoadCert(dir string) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(filepath.Join(dir, CertFile))
	if err != nil {
		return tls.Certificate{}, err
	}
	chainPEM, err := os.ReadFile(filepath.Join(dir, ChainFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return tls.Certificate{}, err
	}
	var cert tls.Certificate
	rest := append(append(append([]byte{}, certPEM...), '\n'), chainPEM...)
	for {
		var block *pem.Block
		if block, rest = pem.Decode(rest); block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("certificate %d of the chain: %w", len(cert.Certificate)+1, err)
		}
		if cert.Leaf == nil {
			cert.Leaf = c
		}
		cert.Certificate = append(cert.Certificate, block.Bytes)
	}
	if cert.Leaf == nil {
		return tls.Certificate{}, fmt.Errorf("%s holds no certificate", filepath.Join(dir, CertFile))
	}
	return cert, nil
}

// Pair is Load on bytes: the certificate, then the chain, then the key.
func Pair(certPEM, chainPEM, keyPEM []byte) (tls.Certificate, error) {
	full := certPEM
	if len(bytes.TrimSpace(chainPEM)) > 0 {
		full = append(append(append([]byte{}, certPEM...), '\n'), chainPEM...)
	}
	cert, err := tls.X509KeyPair(full, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certificate and key: %w", err)
	}
	for i, der := range cert.Certificate {
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("certificate %d of the chain: %w", i+1, err)
		}
		if i == 0 {
			cert.Leaf = c
		}
	}
	return cert, nil
}

// Fingerprint is what a client pins: the SHA-256 of the leaf certificate,
// written sha256:<hex>.
func Fingerprint(cert tls.Certificate) string {
	return FingerprintDER(cert.Certificate[0])
}

func FingerprintDER(der []byte) string {
	sum := sha256.Sum256(der)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// SamePin compares a pin as a person may write it — upper or lower case, with
// or without the sha256: prefix and colons — with a fingerprint.
func SamePin(pin, fingerprint string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.TrimPrefix(s, "sha256:")
		return strings.ReplaceAll(s, ":", "")
	}
	return norm(pin) != "" && norm(pin) == norm(fingerprint)
}

// ServerConfig serves cert, over HTTP/2 or HTTP/1.1.
func ServerConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2", "http/1.1"},
		MinVersion:   tls.VersionTLS12,
	}
}
