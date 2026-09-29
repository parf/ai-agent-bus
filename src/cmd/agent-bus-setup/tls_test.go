package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/tlsdir"
)

func pemCert(t *testing.T, cn string, notAfter time.Time, ca bool, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, []byte, []byte) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-48 * time.Hour), NotAfter: notAfter, IsCA: ca, BasicConstraintsValid: true, DNSNames: []string{cn}}
	signer, sk := tmpl, key
	if parent != nil {
		signer, sk = parent, parentKey
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, &key.PublicKey, sk)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	kder, _ := x509.MarshalECPrivateKey(key)
	return c, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kder})
}

func write(t *testing.T, dir, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// A generated certificate covers every name it was given, as a DNS name or an
// IP address, and pairs with its key.
func TestASelfSignedCertificateCoversItsNames(t *testing.T) {
	certPEM, keyPEM, err := selfSigned([]string{"localhost", "127.0.0.1", "bus.example"})
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tlsdir.Pair(certPEM, nil, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"localhost", "127.0.0.1", "bus.example"} {
		if err := cert.Leaf.VerifyHostname(name); err != nil {
			t.Errorf("the certificate does not cover %s: %v", name, err)
		}
	}
	if left := time.Until(cert.Leaf.NotAfter); left < 9*365*24*time.Hour {
		t.Errorf("a self-signed certificate lasts %v, not ten years", left)
	}
	if !strings.Contains(strings.Join(certNames([]string{"extra.example"}), " "), "extra.example") {
		t.Error("--tls-name is not among the names")
	}
}

// Provided files are checked before anything is copied: the key must be the
// certificate's, the chain is kept, and an expired certificate is refused.
func TestProvidedFilesAreChecked(t *testing.T) {
	dir := t.TempDir()
	ca, caKey, caPEM, _ := pemCert(t, "test ca", time.Now().Add(time.Hour*24*400), true, nil, nil)
	_, _, leafPEM, leafKey := pemCert(t, "bus.example", time.Now().Add(time.Hour*24*400), false, ca, caKey)
	c := &tlsChoice{mode: "files", cert: write(t, dir, "c.pem", leafPEM), key: write(t, dir, "k.pem", leafKey), chain: write(t, dir, "chain.pem", caPEM)}
	if _, chain, _, err := provided(c, io.Discard); err != nil || len(chain) == 0 {
		t.Fatalf("a valid certificate, chain and key: %v", err)
	}
	_, _, _, otherKey := pemCert(t, "other", time.Now().Add(time.Hour), false, nil, nil)
	c.key = write(t, dir, "other.pem", otherKey)
	if _, _, _, err := provided(c, io.Discard); err == nil {
		t.Fatal("a certificate was accepted with somebody else's key")
	}
	_, _, oldPEM, oldKey := pemCert(t, "old.example", time.Now().Add(-time.Hour), false, nil, nil)
	c = &tlsChoice{mode: "files", cert: write(t, dir, "old.pem", oldPEM), key: write(t, dir, "oldk.pem", oldKey)}
	if _, _, _, err := provided(c, io.Discard); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("an expired certificate: %v", err)
	}
}

func TestTheTLSFlagsGoTogether(t *testing.T) {
	for _, bad := range []tlsChoice{
		{mode: "files"},
		{mode: "files", cert: "c"},
		{mode: "self-signed", cert: "c"},
		{mode: "sometimes"},
		{mode: "off", names: list{"x"}},
	} {
		if err := bad.check(); err == nil {
			t.Errorf("%+v was accepted", bad)
		}
	}
	for _, good := range []tlsChoice{{}, {mode: "off"}, {mode: "self-signed", names: list{"x"}}, {mode: "files", cert: "c", key: "k"}} {
		if err := good.check(); err != nil {
			t.Errorf("%+v: %v", good, err)
		}
	}
}

// At a terminal setup asks: no is plain HTTP only, yes generates by default,
// f asks for the files. A node already serving TLS is not asked again.
func TestSetupAsksAboutTLS(t *testing.T) {
	cases := []struct{ in, mode, cert string }{
		{"\n", "off", ""},
		{"n\n", "off", ""},
		{"y\n\n", "self-signed", ""},
		{"yes\nG\n", "self-signed", ""},
		{"y\nf\n/c.pem\n/k.pem\n\n", "files", "/c.pem"},
	}
	for _, c := range cases {
		ch := &tlsChoice{dir: t.TempDir()}
		if err := ch.ask(strings.NewReader(c.in), io.Discard); err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if ch.mode != c.mode || ch.cert != c.cert {
			t.Errorf("%q: mode %q cert %q, want %q %q", c.in, ch.mode, ch.cert, c.mode, c.cert)
		}
	}
	dir := t.TempDir()
	write(t, dir, tlsdir.CertFile, []byte("x"))
	write(t, dir, tlsdir.KeyFile, []byte("x"))
	ch := &tlsChoice{dir: dir}
	if err := ch.ask(strings.NewReader("n\n"), io.Discard); err != nil || ch.mode != "" {
		t.Fatalf("a node already serving TLS was asked: mode %q, %v", ch.mode, err)
	}
	if !ch.enabled() {
		t.Fatal("and does not keep it")
	}
}

// Installed, the key is root's and the daemon account's alone, the
// certificate anybody's, and a chain left from an earlier choice goes.
func TestInstallWritesTheDirectoryWithItsModes(t *testing.T) {
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	g, err := user.LookupGroupId(me.Gid)
	if err != nil {
		t.Fatal(err)
	}
	old := tlsOwner
	tlsOwner = os.Getuid()
	t.Cleanup(func() { tlsOwner = old })
	// A parent that is not there yet, as /etc/agent-bus is on a fresh host:
	// the daemon's account has to walk through it.
	dir := filepath.Join(t.TempDir(), "etc", "agent-bus", "tls")
	c := &tlsChoice{mode: "self-signed", dir: dir, names: list{"bus.example"}}
	if _, err := c.install(g.Name, io.Discard); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Dir(dir)); err != nil || st.Mode().Perm() != 0o755 {
		t.Fatalf("the parent setup made is %v, not 0755 for the daemon's account to walk: %v", st.Mode().Perm(), err)
	}
	write(t, dir, tlsdir.ChainFile, []byte("stale"))
	fp, err := c.install(g.Name, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(fp, "sha256:") {
		t.Fatalf("fingerprint %q", fp)
	}
	for name, want := range map[string]os.FileMode{"": 0o750, tlsdir.CertFile: 0o644, tlsdir.KeyFile: 0o640} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != want {
			t.Errorf("%q is %04o, want %04o", name, st.Mode().Perm(), want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, tlsdir.ChainFile)); !os.IsNotExist(err) {
		t.Error("a stale chain.pem was left to be appended to the new certificate")
	}
}

// --tls off sticks: the files stay, but a later run without --tls keeps the
// node plain and does not ask again; turning TLS on again clears the mark.
func TestTLSOffSticksUntilTurnedOnAgain(t *testing.T) {
	me, _ := user.Current()
	g, _ := user.LookupGroupId(me.Gid)
	old := tlsOwner
	tlsOwner = os.Getuid()
	t.Cleanup(func() { tlsOwner = old })
	dir := filepath.Join(t.TempDir(), "tls")
	on := &tlsChoice{mode: "self-signed", dir: dir}
	if _, err := on.install(g.Name, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := (&tlsChoice{mode: "off", dir: dir}).install(g.Name, io.Discard); err != nil {
		t.Fatal(err)
	}
	later := &tlsChoice{dir: dir}
	if later.enabled() {
		t.Fatal("a run after --tls off turned TLS back on")
	}
	if err := later.ask(strings.NewReader("y\n\n"), io.Discard); err != nil || later.mode != "" {
		t.Fatalf("a node turned off was asked again: %q, %v", later.mode, err)
	}
	if _, err := on.install(g.Name, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !(&tlsChoice{dir: dir}).enabled() {
		t.Fatal("turning TLS on again left it off")
	}
}

// Anything but a real directory at the TLS path is refused, a link included.
func TestTheTLSPathMustBeARealDirectory(t *testing.T) {
	base := t.TempDir()
	os.Mkdir(filepath.Join(base, "elsewhere"), 0o750)
	link := filepath.Join(base, "tls")
	os.Symlink(filepath.Join(base, "elsewhere"), link)
	if _, err := (&tlsChoice{mode: "self-signed", dir: link}).install("root", io.Discard); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("a symlinked TLS directory: %v", err)
	}
}

// The unit carries -tls-dir exactly when TLS is on.
func TestTheUnitCarriesTheTLSDirectoryWhenOn(t *testing.T) {
	on := unitFor("/program/agent-busd", "0.0.0.0:6767", "owner@example", nil, "/etc/agent-bus/tls")
	if !strings.Contains(on, " -tls-dir /etc/agent-bus/tls") {
		t.Fatalf("the TLS unit lacks -tls-dir:\n%s", on)
	}
	if off := unitFor("/program/agent-busd", "0.0.0.0:6767", "owner@example", nil, ""); strings.Contains(off, "-tls-dir") {
		t.Fatalf("a plain unit carries -tls-dir:\n%s", off)
	}
}

// The web face gets its own copy of the node's certificate, which only its
// account reads, and a drop-in that names it; off, the drop-in goes.
func TestTheWebFaceGetsItsOwnCopyAndDropIn(t *testing.T) {
	me, _ := user.Current()
	g, _ := user.LookupGroupId(me.Gid)
	oldOwner, oldDir, oldDrop, oldAcct := tlsOwner, webTLSDir, webTLSDropIn, webTLSAccount
	base := t.TempDir()
	tlsOwner, webTLSDir, webTLSDropIn, webTLSAccount = os.Getuid(), filepath.Join(base, "web-tls"), filepath.Join(base, "unit.d", "tls.conf"), g.Name
	t.Cleanup(func() { tlsOwner, webTLSDir, webTLSDropIn, webTLSAccount = oldOwner, oldDir, oldDrop, oldAcct })
	daemon := &tlsChoice{mode: "self-signed", dir: filepath.Join(base, "tls")}
	want, err := daemon.install(g.Name, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got, err := webTLS(true, daemon.dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("the web copy serves %s, the daemon %s", got, want)
	}
	if st, _ := os.Stat(filepath.Join(webTLSDir, tlsdir.KeyFile)); st == nil || st.Mode().Perm() != 0o640 {
		t.Fatalf("the web key is %v", st)
	}
	if b, err := os.ReadFile(webTLSDropIn); err != nil || !strings.Contains(string(b), "Environment=AGENT_BUS_WEB_TLS_DIR="+webTLSDir+"\n") {
		t.Fatalf("the drop-in: %q, %v", b, err)
	}
	if fp, err := webTLS(false, daemon.dir); err != nil || fp != "" {
		t.Fatalf("turning it off: %q, %v", fp, err)
	}
	if _, err := os.Stat(webTLSDropIn); !os.IsNotExist(err) {
		t.Fatal("the drop-in outlived TLS being off")
	}
	if !tlsdir.Present(webTLSDir) {
		t.Fatal("turning TLS off removed the web copy, which setup leaves in place")
	}
}
