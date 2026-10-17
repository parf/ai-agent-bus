package core

import (
	"errors"
	"fmt"

	"github.com/parf/ai-agent-bus/internal/envfile"
)

// ErrEnv is a secret that is not an env file.
var ErrEnv = errors.New("a secret is an env file")

// validEnv uses the same grammar as runner delivery; values are stored as written.
// Parsing errors report a line and never repeat the private input.
func validEnv(secret string) error {
	_, err := envfile.Parse([]byte(secret))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrEnv, err)
	}
	return nil
}
