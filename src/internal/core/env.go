package core

import (
	"errors"
	"fmt"
	"strings"
)

// ErrEnv is a secret that is not an env file.
var ErrEnv = errors.New("a secret is an env file")

// validEnv checks a secret for basic env-file syntax and nothing
// application-specific (docs/constitution.md#-private-values): every line is
// blank, a # comment, or KEY=value with an optional `export `, KEY a shell
// identifier, and a quoted value closed on its own line. The refusal names
// the line, never its content: a secret is not repeated in an error.
func validEnv(secret string) error {
	if strings.ContainsRune(secret, 0) {
		return fmt.Errorf("%w, and this one holds a NUL byte", ErrEnv)
	}
	for i, line := range strings.Split(secret, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok || !envKey(strings.TrimSpace(key)) {
			return fmt.Errorf("%w: line %d is not KEY=value", ErrEnv, i+1)
		}
		value = strings.TrimSpace(value)
		if q := value; q != "" && (q[0] == '"' || q[0] == '\'') {
			if len(q) < 2 || !strings.Contains(q[1:], string(q[0])) {
				return fmt.Errorf("%w: line %d opens a quote it does not close", ErrEnv, i+1)
			}
		}
	}
	return nil
}

func envKey(k string) bool {
	if k == "" || k[0] >= '0' && k[0] <= '9' {
		return false
	}
	for _, c := range k {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
