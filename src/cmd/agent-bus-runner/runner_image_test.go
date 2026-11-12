package main

import (
	"testing"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
)

// Every runner carries one picture that keeps the image rules.
func TestRunnerPictureKeepsTheRules(t *testing.T) {
	img, err := agentmeta.CheckImage(runnerImage)
	if err != nil || img.SHA256 != runnerPicture.SHA256 || img.Type != "image/webp" {
		t.Fatalf("runner.webp: %v %s", err, img.Type)
	}
}
