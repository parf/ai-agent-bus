package tlsdir

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type issued struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

func issue(t *testing.T, cn string, ca bool, parent *issued) issued {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  ca,
		BasicConstraintsValid: true,
		DNSNames:              []string{cn},
	}
	signer, signerKey := tmpl, key
	if parent != nil {
		signer, signerKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	return issued{cert: c, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

func keyPEM(t *testing.T, k *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
}

// A provided chain goes out behind the certificate: the leaf first, then
// what signed it.
func TestAChainIsAppendedToTheCertificate(t *testing.T) {
	ca := issue(t, "test ca", true, nil)
	leaf := issue(t, "bus.example", false, &ca)
	cert, err := Pair(leaf.pem, ca.pem, keyPEM(t, leaf.key))
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) != 2 {
		t.Fatalf("served %d certificates, want the leaf and its chain", len(cert.Certificate))
	}
	if cert.Leaf == nil || cert.Leaf.Subject.CommonName != "bus.example" {
		t.Fatalf("the leaf is not first: %+v", cert.Leaf)
	}
}

func TestAKeyThatIsNotTheCertificatesIsRefused(t *testing.T) {
	leaf := issue(t, "bus.example", false, nil)
	other := issue(t, "other", false, nil)
	if _, err := Pair(leaf.pem, nil, keyPEM(t, other.key)); err == nil {
		t.Fatal("a certificate was paired with somebody else's key")
	}
}

func TestLoadReadsTheDirectoryAndAMissingChainIsFine(t *testing.T) {
	dir := t.TempDir()
	leaf := issue(t, "bus.example", false, nil)
	os.WriteFile(filepath.Join(dir, CertFile), leaf.pem, 0o644)
	os.WriteFile(filepath.Join(dir, KeyFile), keyPEM(t, leaf.key), 0o600)
	if !Present(dir) {
		t.Fatal("a directory with a certificate and key is not present")
	}
	cert, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(leaf.cert.Raw)
	if got, want := Fingerprint(cert), "sha256:"+hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("fingerprint %s, want %s", got, want)
	}
	if Present(t.TempDir()) {
		t.Fatal("an empty directory is present")
	}
}

// A key everybody may read is refused: the daemon never serves it.
func TestAKeyReadableByEveryoneIsRefused(t *testing.T) {
	dir := t.TempDir()
	leaf := issue(t, "bus.example", false, nil)
	os.WriteFile(filepath.Join(dir, CertFile), leaf.pem, 0o644)
	os.WriteFile(filepath.Join(dir, KeyFile), keyPEM(t, leaf.key), 0o644)
	os.Chmod(filepath.Join(dir, KeyFile), 0o644)
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "readable by everyone") {
		t.Fatalf("a 0644 key was loaded: %v", err)
	}
	os.Chmod(filepath.Join(dir, KeyFile), 0o640)
	if _, err := Load(dir); err != nil {
		t.Fatalf("a 0640 key was refused: %v", err)
	}
}

// A chain file that is not certificates is refused, not ignored.
func TestAGarbageChainIsRefused(t *testing.T) {
	leaf := issue(t, "bus.example", false, nil)
	if _, err := Pair(leaf.pem, []byte("-----BEGIN CERTIFICATE-----\nbm90IGEgY2VydA==\n-----END CERTIFICATE-----\n"), keyPEM(t, leaf.key)); err == nil {
		t.Fatal("a chain that is not a certificate was accepted")
	}
}

func TestAPinMatchesHoweverItIsWritten(t *testing.T) {
	fp := "sha256:ab01cd"
	for _, pin := range []string{"sha256:ab01cd", "AB01CD", "ab:01:cd", " SHA256:AB:01:CD "} {
		if !SamePin(pin, fp) {
			t.Errorf("%q does not match %s", pin, fp)
		}
	}
	for _, pin := range []string{"", "sha256:", "ab01ce"} {
		if SamePin(pin, fp) {
			t.Errorf("%q matches %s", pin, fp)
		}
	}
	// A blank pin pins nothing, even against a blank fingerprint.
	if SamePin("", "") || SamePin("sha256:", "sha256:") {
		t.Error("a blank pin matched")
	}
}

// The fingerprint and description are public: they are read without the key,
// so a user who may not read key.pem still learns them.
func TestTheCertificateIsReadWithoutTheKey(t *testing.T) {
	dir := t.TempDir()
	ca := issue(t, "test ca", true, nil)
	leaf := issue(t, "bus.example", false, &ca)
	os.WriteFile(filepath.Join(dir, CertFile), leaf.pem, 0o644)
	os.WriteFile(filepath.Join(dir, ChainFile), ca.pem, 0o644)
	os.WriteFile(filepath.Join(dir, KeyFile), keyPEM(t, leaf.key), 0o000)
	cert, err := LoadCert(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) != 2 || cert.Leaf.Subject.CommonName != "bus.example" {
		t.Fatalf("read %d certificates, leaf %v", len(cert.Certificate), cert.Leaf.Subject)
	}
	full, err := Pair(leaf.pem, ca.pem, keyPEM(t, leaf.key))
	if err != nil {
		t.Fatal(err)
	}
	if Fingerprint(cert) != Fingerprint(full) {
		t.Fatal("the certificate alone gives another fingerprint than the served pair")
	}
}
