package api

import (
	"github.com/parf/ai-agent-bus/internal/jsonshape"
	"io"
)

func decodeRequest(r io.Reader, out any) error { return jsonshape.Decode(r, out) }
