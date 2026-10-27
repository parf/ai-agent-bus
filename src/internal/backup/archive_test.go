package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func key(t *testing.T) (string, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "identity")
	c := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", p)
	if out, e := c.CombinedOutput(); e != nil {
		t.Fatalf("fixture key: %v %s", e, out)
	}
	pub, e := os.ReadFile(p + ".pub")
	if e != nil {
		t.Fatal(e)
	}
	return p, string(pub)
}
func info() Info {
	return Info{Kind: "partial", Host: "fixture.example", Version: "fixture", When: time.Unix(1700000000, 0).UTC(), Who: "owner@fixture", Include: []string{"tag:ops"}, Names: []string{"#runner@fixture"}}
}
func TestEncryptedRoundTripAndOptionalMatchingSidecar(t *testing.T) {
	private, public := key(t)
	other, _ := key(t)
	i := info()
	fp, e := Fingerprint(public)
	if e != nil {
		t.Fatal(e)
	}
	i.Recipients = []string{fp}
	payload := []byte(`{"Secret":"fixture-private-secret","Current":"fixture-private-token"}`)
	packed, e := Pack(i, map[string][]byte{"records.json": payload})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cipher, e := Encrypt(ctx, packed, []string{public})
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"fixture-private-secret", "fixture-private-token"} {
		if bytes.Contains(cipher, []byte(secret)) {
			t.Fatal("private value is readable in ciphertext")
		}
	}
	if _, e := Decrypt(ctx, cipher, other); e == nil {
		t.Fatal("wrong key decrypted archive")
	}
	plain, e := Decrypt(ctx, cipher, private)
	if e != nil {
		t.Fatal(e)
	}
	manifest, files, e := Unpack(plain)
	if e != nil || !bytes.Equal(files["records.json"], payload) {
		t.Fatal("encrypted round trip failed")
	}
	side, e := SidecarBytes(i, cipher)
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"fixture-private-secret", "fixture-private-token"} {
		if bytes.Contains(side, []byte(secret)) {
			t.Fatal("private value reached sidecar")
		}
	}
	if CheckSidecar(cipher, manifest, nil) != nil || CheckSidecar(cipher, manifest, side) != nil {
		t.Fatal("optional matching sidecar refused")
	}
	var altered Sidecar
	if json.Unmarshal(side, &altered) != nil {
		t.Fatal("sidecar not JSON")
	}
	altered.Info.Who = "other@fixture"
	bad, _ := json.Marshal(altered)
	if CheckSidecar(cipher, manifest, bad) == nil {
		t.Fatal("mismatched sidecar info accepted")
	}
	if CheckSidecar(append(cipher, 'x'), manifest, side) == nil {
		t.Fatal("mismatched ciphertext hash accepted")
	}
	if !strings.HasPrefix(Filename(i), "partial-fixture.example-") {
		t.Fatal("partial filename lacks its kind")
	}
	i.Kind = "full"
	if !strings.HasPrefix(Filename(i), "full-fixture.example-") {
		t.Fatal("full filename lacks its kind")
	}
}
func TestArchiveRefusesUnsafeEntriesAndAlteredContents(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "a/../b", "manifest.json", "a\\b"} {
		if _, e := Pack(info(), map[string][]byte{name: []byte("data")}); e == nil {
			t.Fatalf("unsafe entry accepted %q", name)
		}
	}
	packed, e := Pack(info(), map[string][]byte{"runner/env": []byte("private")})
	if e != nil {
		t.Fatal(e)
	}
	m, files, e := Unpack(packed)
	if e != nil {
		t.Fatal(e)
	}
	files["runner/env"] = []byte("changed")
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	meta, _ := json.Marshal(m)
	for name, body := range map[string][]byte{"manifest.json": meta, "runner/env": files["runner/env"]} {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write(body)
	}
	_ = tw.Close()
	_ = gz.Close()
	if _, _, e := Unpack(out.Bytes()); e == nil {
		t.Fatal("altered payload accepted")
	}
}

func TestArchiveFilesNeverOverwriteAndFailedSidecarRemovesArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial-host-time.tar.gz.age")
	if e := WriteFiles(path, []byte("cipher"), []byte("info")); e != nil {
		t.Fatal(e)
	}
	if e := WriteFiles(path, []byte("new"), []byte("changed")); e == nil {
		t.Fatal("archive overwritten")
	}
	old, _ := os.ReadFile(path)
	if string(old) != "cipher" {
		t.Fatal("existing archive changed")
	}
	other := filepath.Join(t.TempDir(), "full-host-time.tar.gz.age")
	if e := os.WriteFile(other+".info", []byte("existing"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := WriteFiles(other, []byte("cipher"), []byte("info")); e == nil {
		t.Fatal("sidecar overwritten")
	}
	if _, e := os.Stat(other); !os.IsNotExist(e) {
		t.Fatal("failed sidecar left archive")
	}
}
func TestArchiveRejectsBadGzipChecksum(t *testing.T) {
	packed, e := Pack(info(), map[string][]byte{"records.json": []byte("fixture")})
	if e != nil {
		t.Fatal(e)
	}
	packed[len(packed)-8] ^= 1
	if _, _, e = Unpack(packed); e == nil {
		t.Fatal("gzip checksum ignored")
	}
}
