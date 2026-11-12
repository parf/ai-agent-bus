package api

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"testing"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// #bus answers the image ask with its own picture, and help names its hash.
func TestBusAnswersItsImage(t *testing.T) {
	_, s, _ := jsonFixture(t)
	got, err := s.daemonAnswer(protocol.Envelope{From: "alice@h", Ask: protocol.AskImage})
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatal(err)
	}
	img, err := agentmeta.CheckImage(data)
	if err != nil || img.Type != "image/webp" {
		t.Fatalf("the image ask answers no valid picture: %v %s", err, img.Type)
	}
	var help struct {
		Image string   `json:"image_sha256"`
		Asks  []string `json:"asks"`
	}
	if err := json.Unmarshal([]byte(busAnswer2(t, s)), &help); err != nil {
		t.Fatal(err)
	}
	if help.Image != img.SHA256 || !slices.Contains(help.Asks, "image") {
		t.Fatalf("help names hash %q and asks %v, the picture is %s", help.Image, help.Asks, img.SHA256)
	}
}
