package qdrant

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

type requestFailure struct{ beforeConnect bool }

func (e requestFailure) Error() string {
	return "Qdrant request failed or reached the 300 s operation deadline"
}

type httpFailure struct {
	method string
	status int
}

func (e httpFailure) Error() string {
	return fmt.Sprintf("Qdrant %s refused (HTTP %d)", e.method, e.status)
}
func definitelyNotCreated(err error) bool {
	var httpErr httpFailure
	var reqErr requestFailure
	return errors.As(err, &httpErr) && httpErr.status >= 400 && httpErr.status < 500 || errors.As(err, &reqErr) && reqErr.beforeConnect
}

func (h *Host) request(ctx context.Context, base, method, path string, body any, key bool) (*http.Response, error) {
	var input io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		input = strings.NewReader(string(b))
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, input)
	if err != nil {
		return nil, errors.New("invalid Qdrant request")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key {
		req.Header.Set("api-key", h.key)
	}
	resp, err := h.Client.Do(req)
	if err != nil {
		var op *net.OpError
		return nil, requestFailure{errors.As(err, &op) && op.Op == "dial"}
	}
	return resp, nil
}
func (h *Host) json(ctx context.Context, base, method, path string, body, result any) error {
	resp, err := h.request(ctx, base, method, path, body, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpFailure{method, resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return errors.New("Qdrant response could not be read")
	}
	if len(data) > 1<<20 {
		return errors.New("Qdrant JSON response exceeds 1 MiB")
	}
	if result == nil {
		return nil
	}
	var wire struct {
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(data, &wire) != nil || len(wire.Result) == 0 || json.Unmarshal(wire.Result, result) != nil {
		return errors.New("Qdrant returned malformed JSON")
	}
	return nil
}
func (h *Host) ready(ctx context.Context, base string) error {
	end := time.NewTimer(120 * time.Second)
	defer end.Stop()
	for {
		attempt, cancel := context.WithTimeout(ctx, time.Second)
		var got struct {
			Collections []struct {
				Name string `json:"name"`
			} `json:"collections"`
		}
		err := h.json(attempt, base, "GET", "/collections", nil, &got)
		cancel()
		if err == nil {
			resp, err := h.request(ctx, base, "GET", "/collections", nil, false)
			if err != nil {
				return err
			}
			resp.Body.Close()
			if resp.StatusCode != 401 && resp.StatusCode != 403 {
				return errors.New("Qdrant accepted a collections request without an API key")
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("Qdrant startup deadline exceeded; last attempt: %v", err)
		case <-end.C:
			return fmt.Errorf("Qdrant not ready within 120 s; last attempt: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

type snapshotInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
}

var snapshotName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,180}\.snapshot$`)
var ownedBackup = regexp.MustCompile(`^qdrant-[0-9]{8}T[0-9]{6}\.[0-9]{9}Z-[0-9a-f]{8}\.snapshot$`)

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func (h *Host) snapshot(ctx context.Context) (file string, err error) {
	ctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()
	dir := h.path(backupDir)
	if err := privateDir(dir, rootUID(h), rootGID(h)); err != nil {
		return "", err
	}
	free, err := h.DiskFree(dir)
	if err != nil {
		return "", err
	}
	if free < 30<<30 || free < uint64(h.MaxBytes)+(1<<30) {
		return "", errors.New("snapshot refused: need 30 GiB free and the snapshot budget plus 1 GiB spare")
	}
	if err := h.refuseUnverified(); err != nil {
		return "", err
	}
	if err := h.resumeSnapshotCleanup(ctx); err != nil {
		return "", err
	}
	before, err := h.beginSnapshot(ctx)
	if err != nil {
		return "", err
	}
	var snap snapshotInfo
	if err := h.json(ctx, h.Base, "POST", "/snapshots", nil, &snap); err != nil {
		if definitelyNotCreated(err) {
			if e := os.Remove(h.path(backupDir + "/" + unverifiedFile)); e != nil {
				return "", errors.Join(err, e)
			}
			if e := syncDir(dir); e != nil {
				return "", errors.Join(err, e)
			}
			return "", err
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		h.unverifiedHint(cleanup, before)
		return "", err
	}

	if !snapshotName.MatchString(snap.Name) || strings.Contains(snap.Name, "..") {
		return "", errors.New("Qdrant returned an unsafe snapshot name; unverified journal blocks retries; owner inspection required")
	}
	var pending *snapshotCleanup
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if pending != nil {
			if e := h.finishSnapshotCleanup(cleanup, *pending); e != nil {
				err = errors.Join(err, errors.New("snapshot child cleanup pending in private journal; full retained"))
			}
			return
		}
		h.unverifiedHint(cleanup, before)
		if e := h.json(cleanup, h.Base, "DELETE", "/snapshots/"+url.PathEscape(snap.Name), nil, nil); e != nil {
			err = errors.Join(err, errors.New("own Qdrant-side snapshot cleanup failed"))
		}
	}()
	if snap.Size <= 0 || snap.Size > h.MaxBytes {
		return "", fmt.Errorf("snapshot exceeds the %d-byte cap or has invalid size", h.MaxBytes)
	}
	if !hexKey.MatchString(strings.ToLower(snap.Checksum)) {
		return "", errors.New("snapshot has no valid upstream SHA-256 checksum")
	}
	resp, err := h.request(ctx, h.Base, "GET", "/snapshots/"+url.PathEscape(snap.Name), nil, true)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("snapshot download refused (HTTP %d)", resp.StatusCode)
	}
	if resp.ContentLength > h.MaxBytes {
		return "", fmt.Errorf("snapshot exceeds the %d-byte cap", h.MaxBytes)
	}
	f, err := os.CreateTemp(dir, ".qdrant-partial-")
	if err != nil {
		return "", err
	}
	partial := f.Name()
	defer func() { f.Close(); os.Remove(partial) }()
	if err := f.Chown(rootUID(h), rootGID(h)); err != nil {
		return "", err
	}
	if err := f.Chmod(0600); err != nil {
		return "", err
	}
	hash := sha256.New()
	size, e := io.Copy(io.MultiWriter(f, hash), io.LimitReader(resp.Body, h.MaxBytes+1))
	if e != nil {
		return "", errors.New("snapshot download failed or reached the 300 s deadline")
	}
	if size > h.MaxBytes {
		return "", fmt.Errorf("snapshot exceeds the %d-byte cap", h.MaxBytes)
	}
	if size != snap.Size || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), snap.Checksum) {
		return "", errors.New("snapshot size/SHA-256 did not match Qdrant")
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	state, err := h.recordSnapshotCleanup(snap.Name, partial)
	if err != nil {
		return "", err
	}
	pending = &state
	if err := os.Remove(h.path(backupDir + "/" + unverifiedFile)); err != nil {
		return "", err
	}
	if err := syncDir(dir); err != nil {
		return "", err
	}
	suffix, err := randomHex(4)
	if err != nil {
		return "", err
	}
	file = filepath.Join(dir, "qdrant-"+time.Now().UTC().Format("20060102T150405.000000000Z")+"-"+suffix+".snapshot")
	if _, err := os.Lstat(file); !os.IsNotExist(err) {
		return "", errors.New("backup name already exists")
	}
	if err := os.Rename(partial, file); err != nil {
		return "", err
	}
	if err := syncDir(dir); err != nil {
		return file, err
	}
	if err := pruneBackups(dir, rootUID(h)); err != nil {
		return file, err
	}
	return file, nil
}
func pruneBackups(dir string, uid int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !ownedBackup.MatchString(e.Name()) {
			continue
		}
		if _, err := privateFileMetadata(filepath.Join(dir, e.Name()), uid); err != nil {
			return err
		}
		names = append(names, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, n := range names[min(7, len(names)):] {
		if err := os.Remove(filepath.Join(dir, n)); err != nil {
			return err
		}
	}
	return syncDir(dir)
}
func privateFileMetadata(path string, uid int) (os.FileInfo, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || int(st.Sys().(*syscall.Stat_t).Uid) != uid {
		return nil, errors.New("refusing non-private or unowned backup")
	}
	return st, nil
}
