// An adversarial replacement web executable for G.1.3 acceptance. It runs
// through the real supervisor and its sandbox, never in the live installation.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var state, logDir, runtimeDir string // disposable paths, supplied at build time

// Every file the web must neither read nor modify, keyed as the gate names
// them: the database and its journal files, the daemon's logs and the SSH
// authorization. Whether each is visible at all is reported beside the
// refusal, as evidence of how it was refused.
func targets() map[string]string {
	return map[string]string{
		"agent-bus.db":         filepath.Join(state, "agent-bus.db"),
		"agent-bus.db-wal":     filepath.Join(state, "agent-bus.db-wal"),
		"agent-bus.db-shm":     filepath.Join(state, "agent-bus.db-shm"),
		".ssh/authorized_keys": filepath.Join(state, ".ssh/authorized_keys"),
		"audit.log":            filepath.Join(logDir, "audit.log"),
		"error.log":            filepath.Join(logDir, "error.log"),
		"debug.log":            filepath.Join(logDir, "debug.log"),
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		result := map[string]any{}
		for name, path := range targets() {
			_, err := os.ReadFile(path)
			result["read "+name] = err != nil
			result["visible "+name] = !errors.Is(err, os.ErrNotExist)
			// These are disposable files. Actually write if permitted: merely
			// observing an error on stat would not test modification authority.
			// No O_CREATE: an absent target must stay absent.
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
			denied := err != nil
			if err == nil {
				_, err = f.WriteString("\nUNEXPECTED WEB WRITE\n")
				denied = err != nil
				f.Close()
			}
			result["write "+name] = denied
		}
		for label, dir := range map[string]string{"create in state": state, "create in logs": logDir} {
			f, err := os.OpenFile(filepath.Join(dir, "web-created"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			result[label] = err != nil
			if err == nil {
				f.Close()
			}
		}
		conn, err := net.DialTimeout("unix", filepath.Join(runtimeDir, "user-agent-busd.sock"), time.Second)
		result["mapped socket"] = err != nil
		if conn != nil {
			conn.Close()
		}
		_, err = os.ReadFile("/proc/" + r.Header.Get("X-Probe-Host-Pid") + "/root" + state + "/agent-bus.db")
		result["host proc root"] = err != nil
		_, err = os.ReadFile("/proc/" + r.Header.Get("X-Probe-Host-Pid") + "/status")
		result["host process visibility"] = err != nil
		result["environment"] = os.Getenv("AGENT_BUS_TOKEN") == "" && os.Getenv("UNRELATED_SECRET") == ""
		for _, entry := range os.Environ() {
			key, value, _ := strings.Cut(entry, "=")
			switch key {
			case "AGENT_BUS_ADDR", "AGENT_BUS_WEB_ADDR", "AGENT_BUS_WEB_CERT", "AGENT_BUS_WEB_KEY":
			case "PWD":
				if value != "/" {
					result["environment"] = false
				}
			default:
				result["environment"] = false
			}
			if strings.Contains(value, "inherited-canary") {
				result["environment"] = false
			}
		}
		// The caller supplies a visitor token, never the fixture's environment.
		transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", os.Getenv("AGENT_BUS_ADDR"))
		}}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
		req, _ := http.NewRequest("GET", "http://bus/status", nil)
		req.Header.Set("X-Agent-Bus-Token", r.Header.Get("X-Agent-Bus-Token"))
		res, err := client.Do(req)
		if err != nil {
			result["api error"] = err.Error()
		} else {
			defer res.Body.Close()
			result["api status"] = res.StatusCode
			if res.StatusCode == 200 {
				body, _ := io.ReadAll(res.Body)
				var data struct {
					You string `json:"you"`
				}
				json.Unmarshal(body, &data)
				result["api you"] = data.You
			}
		}
		json.NewEncoder(w).Encode(result)
	})
	if err := http.ListenAndServe(os.Getenv("AGENT_BUS_WEB_ADDR"), nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
