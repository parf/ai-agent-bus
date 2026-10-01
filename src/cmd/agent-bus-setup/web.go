package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/tlsdir"
)

// The web face is its own process: TypeScript run by the system bun, as its
// own account, under a locked-down unit that reaches the daemon only through
// the shared socket with each visitor's session
// (docs/08-processes.md#the-web-face). Setup installs the account, the link
// /var/lib/agent-bus/web to the release's web directory, and the unit the
// release ships beside its sources.
const (
	webAccount  = "agent-bus-web"
	webLink     = stateRoot + "/web"
	webUnitPath = "/etc/systemd/system/agent-bus-web.service"
	webUnitName = "agent-bus-web.service"
	bunPath     = "/usr/bin/bun"
)

var (
	webAddr   = regexp.MustCompile(`(?m)^Environment=AGENT_BUS_WEB_ADDR=(\S+)$`)
	execPaths = regexp.MustCompile(`(?m)^ExecPaths=.*$`)
	lddPath   = regexp.MustCompile(`(?m)(/\S+) \(0x`)
)

// hostExecPaths is bun and the libraries it links on this host, resolved:
// the unit may execute these and nothing else, and where a distribution
// keeps its libraries differs.
func hostExecPaths() (string, error) {
	out, err := exec.Command("ldd", bunPath).Output()
	if err != nil {
		return "", fmt.Errorf("ldd %s: %w", bunPath, err)
	}
	paths := []string{bunPath}
	for _, m := range lddPath.FindAllSubmatch(out, -1) {
		p, err := filepath.EvalSymlinks(string(m[1]))
		if err != nil {
			return "", err
		}
		paths = append(paths, p)
	}
	return "ExecPaths=" + strings.Join(paths, " "), nil
}

// webSteps is what installWeb does, for --dry-run.
func webSteps(web string) []string {
	return []string{
		fmt.Sprintf("create the system account %s, with no login and %s, its code, for home", webAccount, webLink),
		fmt.Sprintf("link %s to %s", webLink, web),
		fmt.Sprintf("write %s from %s and start it", webUnitPath, filepath.Join(web, webUnitName)),
	}
}

// installWeb installs or refreshes the web face from a release's web
// directory. An existing link that is not a link is an operator's and is
// left alone, as is a missing bun: the daemon serves without a face.
// The web face's own copy of the node's certificate, and the drop-in that
// names it: its account never reads the daemon's key (docs/07-setup.md#tls).
var (
	webTLSDir    = "/etc/agent-bus/web-tls"
	webTLSDropIn = "/etc/systemd/system/agent-bus-web.service.d/tls.conf"
	// webTLSAccount reads the copy; a test names its own group.
	webTLSAccount = webAccount
)

// webTLS gives the web face TLS when the daemon has it: a copy of the
// daemon's certificate, chain and key it may read, and the drop-in that
// points it there. Off, the drop-in goes and the copy stays, as the daemon's
// files do. It answers the fingerprint the face will serve.
func webTLS(on bool, daemonDir string) (string, error) {
	if !on {
		if err := os.Remove(webTLSDropIn); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return "", nil
	}
	read := func(name string, optional bool) ([]byte, error) {
		b, err := os.ReadFile(filepath.Join(daemonDir, name))
		if optional && errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return b, err
	}
	certPEM, err := read(tlsdir.CertFile, false)
	if err != nil {
		return "", err
	}
	chainPEM, err := read(tlsdir.ChainFile, true)
	if err != nil {
		return "", err
	}
	keyPEM, err := read(tlsdir.KeyFile, false)
	if err != nil {
		return "", err
	}
	if err := writeTLSDir(webTLSDir, webTLSAccount, certPEM, chainPEM, keyPEM); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(webTLSDropIn), 0o755); err != nil {
		return "", err
	}
	dropIn := "# Written by agent-bus-setup: TLS beside plain HTTP, which is redirected.\n[Service]\nEnvironment=AGENT_BUS_WEB_TLS_DIR=" + webTLSDir + "\n"
	// Written aside and renamed, as the unit beside it: systemd never
	// parses half a drop-in.
	if err := os.WriteFile(webTLSDropIn+".new", []byte(dropIn), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(webTLSDropIn+".new", webTLSDropIn); err != nil {
		return "", err
	}
	cert, err := tlsdir.Load(webTLSDir)
	if err != nil {
		return "", err
	}
	return tlsdir.Fingerprint(cert), nil
}

func installWeb(web string, tlsOn bool, daemonTLSDir string) error {
	unit, err := os.ReadFile(filepath.Join(web, webUnitName))
	if err != nil {
		return fmt.Errorf("the release has no web face: %w", err)
	}
	if _, err := os.Stat(bunPath); err != nil {
		return fmt.Errorf("%s is missing; install bun there and run setup again", bunPath)
	}
	paths, err := hostExecPaths()
	if err != nil {
		return err
	}
	unit = execPaths.ReplaceAll(unit, []byte(paths))
	// Its home is its code, the link: it owns nothing and writes nowhere.
	if u, err := user.Lookup(webAccount); err != nil {
		if err := run("useradd", "--system", "--no-create-home", "--home-dir", webLink,
			"--shell", "/usr/sbin/nologin", webAccount); err != nil {
			return err
		}
	} else if u.HomeDir != webLink {
		// An account in use cannot be changed: the unit stops first, and is
		// restarted below.
		_ = exec.Command("systemctl", "stop", webUnitName).Run()
		if err := run("usermod", "--home", webLink, webAccount); err != nil {
			return err
		}
	}
	if info, err := os.Lstat(webLink); err == nil && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists and is not a link; move it and run setup again", webLink)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(stateRoot, 0o755); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.new-%d", webLink, os.Getpid())
	_ = os.Remove(tmp)
	if err := os.Symlink(web, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, webLink); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	fingerprint, err := webTLS(tlsOn, daemonTLSDir)
	if err != nil {
		return fmt.Errorf("web TLS: %w", err)
	}
	if err := os.WriteFile(webUnitPath, unit, 0o644); err != nil {
		return err
	}
	if err := run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := run("systemctl", "enable", webUnitName); err != nil {
		return err
	}
	if err := run("systemctl", "restart", webUnitName); err != nil {
		return err
	}
	addr := "127.0.0.1:6780"
	if m := webAddr.FindSubmatch(unit); m != nil {
		addr = string(m[1])
	}
	// With TLS on, plain HTTP is only a redirect: health is asked over
	// https://, trusting exactly the certificate just written.
	if fingerprint != "" {
		return waitHealthz("https://"+addr+"/healthz", 10*time.Second, fingerprint)
	}
	return waitHealthz("http://"+addr+"/healthz", 10*time.Second, "")
}

func waitHealthz(url string, timeout time.Duration, pin string) error {
	deadline := time.Now().Add(timeout)
	client := http.Client{Timeout: time.Second}
	if pin != "" {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // the pin replaces the chain check
			VerifyConnection: func(cs tls.ConnectionState) error {
				if len(cs.PeerCertificates) == 0 || !tlsdir.SamePin(pin, tlsdir.FingerprintDER(cs.PeerCertificates[0].Raw)) {
					return errors.New("the web face serves another certificate than the one setup wrote")
				}
				return nil
			},
		}}
	}
	for time.Now().Before(deadline) {
		if r, err := client.Get(url); err == nil {
			r.Body.Close()
			if r.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	out, _ := exec.Command("systemctl", "status", "--no-pager", webUnitName).CombinedOutput()
	return fmt.Errorf("agent-bus-web did not answer %s:\n%s", url, out)
}

// webDirFor is the web directory beside the daemon being installed: the
// current release's for a package, so a release switch carries the face with
// it, or the one next to an explicit --exec, such as a checkout's.
func webDirFor(exe string) string {
	if dir := filepath.Dir(exe); filepath.Dir(dir) == filepath.Join(installRoot, "releases") || dir == filepath.Join(installRoot, "current") {
		return filepath.Join(installRoot, "current", "web")
	}
	return filepath.Join(filepath.Dir(exe), "web")
}
