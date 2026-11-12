package api

import (
	_ "embed"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
)

// busImage is #bus's own picture, answered to the image ask
// (Plans/R1.1/agent-images.md#how-the-web-gets-it).
//
//go:embed bus.webp
var busImage []byte

// busPicture is the checked picture; a broken embed fails at start, not per ask.
var busPicture = func() agentmeta.Image {
	img, err := agentmeta.CheckImage(busImage)
	if err != nil {
		panic("bus.webp: " + err.Error())
	}
	return img
}()
