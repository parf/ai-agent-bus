package agentmeta

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const imageVectors = "../../testdata/agent-images"

type imageCase struct {
	Type    string `json:"type"`
	Size    int    `json:"size"`
	Refused string `json:"refused"`
}

func imageCases(t *testing.T) map[string]imageCase {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(imageVectors, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]imageCase{}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func TestImageVectors(t *testing.T) {
	for name, want := range imageCases(t) {
		data, err := os.ReadFile(filepath.Join(imageVectors, name))
		if err != nil {
			t.Fatal(err)
		}
		img, err := CheckImage(data)
		if want.Refused != "" {
			if err == nil || !strings.Contains(err.Error(), want.Refused) {
				t.Errorf("%s: want refusal %q, got %v", name, want.Refused, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		_, w, h, _ := imageSize(data)
		if img.Type != want.Type || w != want.Size || h != want.Size || len(img.SHA256) != 64 {
			t.Errorf("%s: got %s %d×%d sha %q, want %s %d", name, img.Type, w, h, img.SHA256, want.Type, want.Size)
		}
	}
}

func TestImageByteCap(t *testing.T) {
	data, _ := os.ReadFile(filepath.Join(imageVectors, "a.png"))
	at := append(append([]byte(nil), data...), make([]byte, ImageMaxBytes-len(data))...)
	if _, err := CheckImage(at); err != nil {
		t.Fatalf("an image of exactly 256 KiB is accepted: %v", err)
	}
	if _, err := CheckImage(append(at, 0)); err == nil || !strings.Contains(err.Error(), "over 256 KiB") {
		t.Fatalf("one byte over 256 KiB: want over 256 KiB, got %v", err)
	}
}

// pkg writes an agent package whose agent.json names image.
func pkg(t *testing.T, image string) string {
	t.Helper()
	dir := t.TempDir()
	m := `{"description":"Pictured agent.","image":` + strconvQuote(image) + `}`
	if err := os.WriteFile(filepath.Join(dir, "agent.json"), []byte(m), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

func copyVector(t *testing.T, name, to string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(imageVectors, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImagePathRefusals(t *testing.T) {
	outside := t.TempDir()
	copyVector(t, "a.png", filepath.Join(outside, "a.png"))
	for _, c := range []struct{ name, image, want string }{
		{"absolute", filepath.Join(outside, "a.png"), "absolute path"},
		{"dotdot", "../a.png", ". or .. parts"},
		{"final symlink", "link.png", "symlink"},
		{"directory symlink", "linked/a.png", "symlink"},
		{"directory", "dir", "not a regular file"},
		{"missing", "none.png", "no such file"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := pkg(t, c.image)
			_ = os.Symlink(filepath.Join(outside, "a.png"), filepath.Join(dir, "link.png"))
			_ = os.Symlink(outside, filepath.Join(dir, "linked"))
			_ = os.Mkdir(filepath.Join(dir, "dir"), 0o755)
			_, err := Load(dir)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("image %q: want refusal containing %q, got %v", c.image, c.want, err)
			}
		})
	}
}

func TestImageHelpAndAsk(t *testing.T) {
	dir := pkg(t, "art/agent.webp")
	copyVector(t, "lossless.webp", filepath.Join(dir, "art", "agent.webp"))
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := LoadImage(dir, "art/agent.webp")
	raw, err := m.Answer(dir, "help-json", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Image       string   `json:"image"`
		ImageSHA256 string   `json:"image_sha256"`
		Asks        []string `json:"asks"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got.Image != "art/agent.webp" || got.ImageSHA256 != want.SHA256 || strings.Join(got.Asks, ",") != "help,help-config,help-secret,help-json,image" {
		t.Fatalf("help-json image fields: %+v", got)
	}
	for _, native := range []string{"", "native"} {
		m.Help = native
		answer, handled, err := DispatchAsk("image", m.AskHandlers(dir, nil, "#x"))
		if err != nil || !handled {
			t.Fatalf("help=%q: image ask: handled=%v err=%v", native, handled, err)
		}
		bytes, err := base64.StdEncoding.DecodeString(answer)
		if err != nil || string(bytes) != string(want.Data) {
			t.Fatalf("help=%q: image ask answers other bytes (%d, %v)", native, len(bytes), err)
		}
	}
}

func TestNoImageNoFields(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "agent.json"), []byte(`{"description":"Plain agent."}`), 0o644)
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := m.Answer(dir, "help-json", nil, true)
	if strings.Contains(raw, "image") {
		t.Fatalf("an agent without an image names none: %s", raw)
	}
	if answer, _, _ := DispatchAsk("image", m.AskHandlers(dir, nil, "#x")); answer != "unsupported: image" {
		t.Fatalf("image ask without an image: %q", answer)
	}
}
