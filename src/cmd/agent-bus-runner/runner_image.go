package main

import (
	_ "embed"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
)

// runnerImage is every runner's own picture, answered to the image ask
// (Plans/R1.1/agent-images.md#how-the-web-gets-it).
//
//go:embed runner.webp
var runnerImage []byte

var runnerPicture = func() agentmeta.Image {
	img, err := agentmeta.CheckImage(runnerImage)
	if err != nil {
		panic("runner.webp: " + err.Error())
	}
	return img
}()
