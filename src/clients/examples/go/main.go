// The Go worker inventories a file and verifies its fingerprint with Rust.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	bus "github.com/parf/ai-agent-bus/src/clients/go"
)

type inventory struct {
	Worker           string `json:"worker"`
	Path             string `json:"path"`
	Bytes            int    `json:"bytes"`
	Lines            int    `json:"lines"`
	BlankLines       int    `json:"blank_lines"`
	LongestLineBytes int    `json:"longest_line_bytes"`
	TODOs            int    `json:"todos"`
	SHA256           string `json:"sha256"`
	VerifiedBy       string `json:"verified_by,omitempty"`
	JavaVerified     bool   `json:"java_verified,omitempty"`
}

type worker struct {
	bus      *bus.Bus
	root     *os.Root
	rootPath string
}

var errUnavailable = errors.New("path is unavailable or outside STATS_ROOT")

func answer(value any) (*string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	s := string(data)
	return &s, nil
}

func (w *worker) inspect(ctx context.Context, path string, verify bool) (inventory, error) {
	out := inventory{Worker: "go"}
	if !filepath.IsAbs(path) {
		return out, errors.New("path must be absolute")
	}
	// One answer for a missing path and an outside one: a caller learns nothing beyond the root.
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return out, errUnavailable
	}
	rel, err := filepath.Rel(w.rootPath, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return out, errUnavailable
	}
	var lease *bus.Lease
	if verify {
		key := sha256.Sum256([]byte(real))
		lease, err = w.bus.Record(os.Getenv("AGENT_BUS_NAME")).Locks.TryAcquire(ctx, "fingerprint-"+hex.EncodeToString(key[:8]), 30*time.Second)
		if err != nil {
			return out, err
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = lease.Release(cleanup)
		}()
	}
	file, err := w.root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return out, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return out, err
	}
	if !info.Mode().IsRegular() {
		return out, errors.New("path must name a regular file")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return out, err
	}
	out.Path, out.Bytes = real, len(data)
	out.Lines = bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		out.Lines++
	}
	lines := bytes.Split(data, []byte{'\n'})
	if len(data) == 0 {
		lines = nil
	} else if data[len(data)-1] == '\n' {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			out.BlankLines++
		}
		if len(line) > out.LongestLineBytes {
			out.LongestLineBytes = len(line)
		}
	}
	out.TODOs = bytes.Count(data, []byte("TODO"))
	hash := sha256.Sum256(data)
	out.SHA256 = hex.EncodeToString(hash[:])
	if verify {
		body, _ := json.Marshal(map[string]string{"path": real})
		result, err := w.bus.Call(ctx, "#stats-rust@localhost", string(body), bus.WithTopic("digest"), bus.WithTimeout(20*time.Second))
		if err != nil {
			return out, err
		}
		if !result.Answered || result.Message == nil {
			return out, errors.New("Rust finished without a digest answer")
		}
		var digest struct {
			Worker       string `json:"worker"`
			SHA256       string `json:"sha256"`
			Error        string `json:"error"`
			JavaVerified bool   `json:"java_verified"`
		}
		if err := json.Unmarshal([]byte(result.Message.Body), &digest); err != nil {
			return out, err
		}
		if digest.Error != "" {
			return out, fmt.Errorf("Rust digest: %s", digest.Error)
		}
		if digest.Worker != "rust" || digest.SHA256 != out.SHA256 || !digest.JavaVerified {
			return out, errors.New("Rust digest disagrees; the file may have changed")
		}
		out.VerifiedBy = "rust"
		out.JavaVerified = true
	}
	_, err = w.bus.Record(os.Getenv("AGENT_BUS_NAME")).KV.Increment(ctx, "files", 1)
	return out, err
}

func (w *worker) handle(ctx context.Context, msg bus.Envelope) (*string, error) {
	var fields map[string]json.RawMessage
	var result any
	err := json.Unmarshal([]byte(msg.Body), &fields)
	if err == nil && fields == nil {
		err = errors.New("body must be a JSON object")
	}
	if err == nil {
		switch msg.Topic {
		case "fingerprint", "stats":
			var path string
			err = json.Unmarshal(fields["path"], &path)
			if err == nil {
				result, err = w.inspect(ctx, path, msg.Topic == "fingerprint")
			}
		case "relay":
			var route, hops []string
			if raw := fields["route"]; raw != nil {
				err = json.Unmarshal(raw, &route)
			}
			if err == nil {
				if raw := fields["hops"]; raw != nil {
					err = json.Unmarshal(raw, &hops)
				}
			}
			if err == nil {
				hops = append(hops, "go")
				fields["hops"], _ = json.Marshal(hops)
				if len(route) > 0 {
					next := route[0]
					route = route[1:]
					fields["route"], _ = json.Marshal(route)
					data, _ := json.Marshal(fields)
					var response bus.CallOutcome
					response, err = w.bus.Call(ctx, next, string(data), bus.WithTopic("relay"), bus.WithTimeout(20*time.Second))
					if err == nil {
						if response.Answered && response.Message != nil {
							body := response.Message.Body
							return &body, nil
						}
						err = errors.New("relay finished without an answer")
					}
				} else {
					fields["route"] = json.RawMessage("[]")
					result = fields
				}
			}
		default:
			err = errors.New("topic must be fingerprint, stats or relay")
		}
	}
	if err != nil {
		result = map[string]string{"worker": "go", "error": err.Error()}
	}
	return answer(result)
}

func main() {
	demo := flag.String("demo", "", "ask the Go worker to fingerprint this absolute path")
	flag.Parse()
	b, err := bus.Dial(bus.Options{ExpectedName: os.Getenv("AGENT_BUS_NAME")})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer b.Close()
	if *demo != "" {
		body, _ := json.Marshal(map[string]string{"path": *demo})
		result, err := b.Call(context.Background(), "#stats-go@localhost", string(body), bus.WithTopic("fingerprint"), bus.WithTimeout(25*time.Second))
		if err != nil || !result.Answered || result.Message == nil {
			fmt.Fprintln(os.Stderr, "fingerprint call failed:", err)
			os.Exit(1)
		}
		fmt.Println(result.Message.Body)
		var report inventory
		if json.Unmarshal([]byte(result.Message.Body), &report) != nil || report.Worker != "go" || report.VerifiedBy != "rust" || !report.JavaVerified || len(report.SHA256) != 64 {
			os.Exit(1)
		}
		return
	}
	rootPath, err := filepath.Abs(os.Getenv("STATS_ROOT"))
	if os.Getenv("STATS_ROOT") == "" {
		err = errors.New("STATS_ROOT is required")
	}
	if err == nil {
		rootPath, err = filepath.EvalSymlinks(rootPath)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer root.Close()
	if os.Getenv("AGENT_BUS_NAME") == "" {
		fmt.Fprintln(os.Stderr, "AGENT_BUS_NAME is required")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	w := worker{b, root, rootPath}
	if err := b.Serve(ctx, w.handle, bus.WithWorkers(4), bus.WithDrainTimeout(25*time.Second)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
