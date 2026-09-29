package duallisten

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// selfSigned is a certificate for 127.0.0.1 and the pool that trusts it.
func selfSigned(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "duallisten test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

// serve starts an HTTP server on a dual listener that says which way each
// request came, and returns its address, the pool to trust and the listener.
func serve(t *testing.T) (string, *x509.CertPool, net.Listener) {
	t.Helper()
	cert, pool := selfSigned(t)
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"h2", "http/1.1"}}
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	l := New(inner, cfg)
	srv := &http.Server{
		TLSConfig: cfg,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS != nil {
				io.WriteString(w, "tls "+r.Proto)
			} else {
				io.WriteString(w, "plain "+r.Proto)
			}
		}),
	}
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })
	return inner.Addr().String(), pool, l
}

func get(t *testing.T, c *http.Client, url string) string {
	t.Helper()
	r, err := c.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

func tlsClient(pool *x509.CertPool, h2 bool) *http.Client {
	tr := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}, ForceAttemptHTTP2: h2}
	return &http.Client{Transport: tr, Timeout: 5 * time.Second}
}

func TestOnePortAnswersTLSAndPlain(t *testing.T) {
	addr, pool, _ := serve(t)
	if got := get(t, tlsClient(pool, false), "https://"+addr+"/"); got != "tls HTTP/1.1" {
		t.Fatalf("https answered %q", got)
	}
	if got := get(t, &http.Client{Timeout: 5 * time.Second}, "http://"+addr+"/"); got != "plain HTTP/1.1" {
		t.Fatalf("http answered %q", got)
	}
}

func TestHTTP2IsNegotiatedOverTLS(t *testing.T) {
	addr, pool, _ := serve(t)
	if got := get(t, tlsClient(pool, true), "https://"+addr+"/"); got != "tls HTTP/2.0" {
		t.Fatalf("an h2 client got %q", got)
	}
}

// A client that connects and says nothing holds up nobody: the sniff runs on
// its own goroutine, and the deadline closes it.
func TestASilentClientBlocksNobodyAndIsClosed(t *testing.T) {
	old := SniffTimeout
	SniffTimeout = 300 * time.Millisecond
	t.Cleanup(func() { SniffTimeout = old })
	addr, pool, _ := serve(t)
	silent, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer silent.Close()
	start := time.Now()
	get(t, &http.Client{Timeout: 5 * time.Second}, "http://"+addr+"/")
	get(t, tlsClient(pool, false), "https://"+addr+"/")
	if d := time.Since(start); d > SniffTimeout {
		t.Fatalf("two clients waited %v behind a silent one", d)
	}
	silent.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := silent.Read(make([]byte, 1)); err == nil {
		t.Fatal("the silent client was sent something instead of being closed")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("the silent client was never closed")
	}
}

// Closing the listener while connections are still being sniffed hands them
// to nobody and closes them, and panics nowhere.
func TestCloseWithSniffsInFlight(t *testing.T) {
	addr, _, l := serve(t)
	var pending []net.Conn
	for i := 0; i < 8; i++ {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		pending = append(pending, c)
	}
	time.Sleep(50 * time.Millisecond)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	for _, c := range pending {
		c.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	}
	// Each stranded connection is closed by the listener, not answered.
	for i, c := range pending {
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := c.Read(make([]byte, 64))
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			t.Fatalf("connection %d was left open after Close", i)
		}
		if n > 0 {
			t.Fatalf("connection %d was answered after Close", i)
		}
		c.Close()
	}
	if _, err := l.Accept(); err == nil {
		t.Fatal("Accept after Close returned a connection")
	}
}

// Two requests in one write both arrive: the replayed first byte and the rest
// of the buffer are the same stream.
func TestPipelinedRequestsAfterTheSniff(t *testing.T) {
	addr, _, _ := serve(t)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write([]byte("GET /a HTTP/1.1\r\nHost: x\r\n\r\nGET /b HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"))
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	b, _ := io.ReadAll(c)
	if got := strings.Count(string(b), "plain HTTP/1.1"); got != 2 {
		t.Fatalf("answered %d of 2 pipelined requests: %q", got, b)
	}
}

// flaky fails its first Accepts the way an exhausted fd table does.
type flaky struct {
	net.Listener
	fails int
}

type tempErr struct{}

func (tempErr) Error() string   { return "too many open files" }
func (tempErr) Timeout() bool   { return false }
func (tempErr) Temporary() bool { return true }

func (f *flaky) Accept() (net.Conn, error) {
	if f.fails > 0 {
		f.fails--
		return nil, tempErr{}
	}
	return f.Listener.Accept()
}

// A failed Accept that is not the listener closing does not kill the port.
func TestATemporaryAcceptErrorKeepsThePort(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := selfSigned(t)
	l := New(&flaky{Listener: inner, fails: 3}, &tls.Config{Certificates: []tls.Certificate{cert}})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "up") })}
	go srv.Serve(l)
	defer srv.Close()
	if got := get(t, &http.Client{Timeout: 5 * time.Second}, "http://"+inner.Addr().String()+"/"); got != "up" {
		t.Fatalf("after temporary Accept errors the port answered %q", got)
	}
}
