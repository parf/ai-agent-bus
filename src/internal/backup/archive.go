// Package backup is the encrypted archive format shared by record export and
// runner backup. It owns no registry policy and never persists a private key.
package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/authkeys"
)

const Format = 1
const MaxBytes int64 = 256 << 20
const MaxFiles = 4096

// Info is public: never add record content or credential values here.
type Info struct {
	Kind       string    `json:"kind"`
	Host       string    `json:"host"`
	Version    string    `json:"version"`
	When       time.Time `json:"when"`
	Who        string    `json:"who"`
	Include    []string  `json:"include"`
	Exclude    []string  `json:"exclude"`
	Names      []string  `json:"names"`
	Recipients []string  `json:"recipient_fingerprints"`
}
type Manifest struct {
	Format int               `json:"format"`
	Info   Info              `json:"backup_info"`
	Files  map[string]string `json:"files"`
}

// The ciphertext's own hash cannot be embedded inside that ciphertext.
type Sidecar struct {
	Info   Info   `json:"backup_info"`
	SHA256 string `json:"sha256"`
}

func Hash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

func Fingerprint(public string) (string, error) {
	key, err := authkeys.Ed25519(public)
	if err != nil {
		return "", errors.New("backup requires an enrolled Ed25519 key")
	}
	fields := strings.Fields(key)
	raw, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return "", errors.New("invalid backup recipient")
	}
	sum := sha256.Sum256(raw)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:]), nil
}

func validName(name string) bool {
	return name != "" && name != "manifest.json" && !strings.Contains(name, "\\") && !strings.HasPrefix(name, "/") && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../")
}

func Pack(info Info, files map[string][]byte) ([]byte, error) {
	if info.Kind != "full" && info.Kind != "partial" {
		return nil, errors.New("invalid backup kind")
	}
	if len(files) == 0 || len(files) > MaxFiles {
		return nil, errors.New("invalid backup file count")
	}
	manifest := Manifest{Format: Format, Info: info, Files: map[string]string{}}
	names := make([]string, 0, len(files))
	var size int64
	for name, body := range files {
		if !validName(name) {
			return nil, errors.New("invalid backup entry name")
		}
		size += int64(len(body))
		if size > MaxBytes {
			return nil, errors.New("backup exceeds 256 MiB")
		}
		manifest.Files[name] = Hash(body)
		names = append(names, name)
	}
	sort.Strings(names)
	meta, err := json.Marshal(manifest)
	if err != nil {
		return nil, errors.New("invalid backup manifest")
	}
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	add := func(name string, body []byte) error {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		_, err := tw.Write(body)
		return err
	}
	if err := add("manifest.json", meta); err != nil {
		return nil, err
	}
	for _, name := range names {
		if err := add(name, files[name]); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	if int64(out.Len()) > MaxBytes {
		return nil, errors.New("backup exceeds 256 MiB")
	}
	return out.Bytes(), nil
}

func Unpack(data []byte) (Manifest, map[string][]byte, error) {
	var manifest Manifest
	bad := func() (Manifest, map[string][]byte, error) {
		return Manifest{}, nil, errors.New("invalid backup archive")
	}
	if int64(len(data)) > MaxBytes {
		return bad()
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return bad()
	}
	defer gz.Close()
	// Read through gzip EOF to check its checksum and bound expanded data,
	// including tar padding and any trailing streams.
	expanded, err := io.ReadAll(io.LimitReader(gz, MaxBytes+1))
	if err != nil || int64(len(expanded)) > MaxBytes {
		return bad()
	}
	source := bytes.NewReader(expanded)
	tr := tar.NewReader(source)
	files := map[string][]byte{}
	var total int64
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return bad()
		}
		if h.Typeflag != tar.TypeReg || h.Size < 0 || h.Size > MaxBytes || len(files) > MaxFiles {
			return bad()
		}
		if h.Name != "manifest.json" && !validName(h.Name) {
			return bad()
		}
		if _, exists := files[h.Name]; exists {
			return bad()
		}
		total += h.Size
		if total > MaxBytes {
			return bad()
		}
		body, e := io.ReadAll(io.LimitReader(tr, h.Size+1))
		if e != nil || int64(len(body)) != h.Size {
			return bad()
		}
		files[h.Name] = body
	}
	trailing, err := io.ReadAll(source)
	if err != nil || len(bytes.Trim(trailing, "\x00")) != 0 {
		return bad()
	}
	meta, found := files["manifest.json"]
	if !found {
		return bad()
	}
	delete(files, "manifest.json")
	decoder := json.NewDecoder(bytes.NewReader(meta))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || decoder.Decode(new(any)) != io.EOF || manifest.Format != Format {
		return bad()
	}
	if manifest.Info.Kind != "full" && manifest.Info.Kind != "partial" || len(manifest.Files) != len(files) || len(files) == 0 {
		return bad()
	}
	for name, sum := range manifest.Files {
		body, ok := files[name]
		if !ok || sum != Hash(body) {
			return bad()
		}
	}
	return manifest, files, nil
}

type boundedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if int64(b.Len()+len(p)) > MaxBytes {
		b.overflow = true
		return 0, errors.New("backup exceeds 256 MiB")
	}
	return b.Buffer.Write(p)
}

func age(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	if int64(len(input)) > MaxBytes {
		return nil, errors.New("backup exceeds 256 MiB")
	}
	cmd := exec.CommandContext(ctx, "age", args...)
	cmd.Stdin = bytes.NewReader(input)
	var out boundedBuffer
	cmd.Stdout = &out
	// Never relay age stderr: a tool or fake provider can echo private input.
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil || out.overflow {
		return nil, errors.New("age operation failed; check the key and archive")
	}
	return out.Bytes(), nil
}
func Encrypt(ctx context.Context, archive []byte, recipients []string) ([]byte, error) {
	if len(recipients) == 0 {
		return nil, errors.New("no enrolled Ed25519 backup recipient")
	}
	args := []string{"--encrypt"}
	for _, key := range recipients {
		if _, err := Fingerprint(key); err != nil {
			return nil, err
		}
		args = append(args, "--recipient", key)
	}
	out, err := age(ctx, archive, args...)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(out, []byte("age-encryption.org/v1\n")) {
		return nil, errors.New("age did not produce an encrypted archive")
	}
	return out, nil
}
func Decrypt(ctx context.Context, cipher []byte, key string) ([]byte, error) {
	if key == "" {
		return nil, errors.New("restore key is required")
	}
	return age(ctx, cipher, "--decrypt", "--identity", key)
}
func CheckSidecar(cipher []byte, manifest Manifest, sidecar []byte) error {
	if len(sidecar) == 0 {
		return nil
	}
	var side Sidecar
	d := json.NewDecoder(bytes.NewReader(sidecar))
	d.DisallowUnknownFields()
	if d.Decode(&side) != nil || d.Decode(new(any)) != io.EOF || side.SHA256 != Hash(cipher) {
		return errors.New("backup sidecar does not match archive")
	}
	a, _ := json.Marshal(side.Info)
	b, _ := json.Marshal(manifest.Info)
	if !bytes.Equal(a, b) {
		return errors.New("backup sidecar info does not match encrypted manifest")
	}
	return nil
}
func SidecarBytes(info Info, cipher []byte) ([]byte, error) {
	return json.MarshalIndent(Sidecar{Info: info, SHA256: Hash(cipher)}, "", "  ")
}
func Filename(info Info) string {
	host := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '-'
	}, info.Host)
	if host == "" {
		host = "node"
	}
	return fmt.Sprintf("%s-%s-%s.tar.gz.age", info.Kind, host, info.When.UTC().Format("20060102T150405.000000000Z"))
}
