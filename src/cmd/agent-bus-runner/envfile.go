package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/parf/ai-agent-bus/internal/envfile"
)

// envVar keeps layering independent of the shared syntax parser.
type envVar = envfile.Var

// parseEnv adds the source file to the shared parser's safe line diagnostic.
func parseEnv(name string, src []byte) ([]envVar, error) {
	vars, err := envfile.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return vars, nil
}

// readEnv is parseEnv of a file that may be absent.
func readEnv(path string) ([]envVar, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseEnv(filepath.Base(path), b)
}

// layered is the environment an agent's process gets: the layers lowest
// first, each later one winning, and the variables env.dist declares with no
// value that no layer set, which refuse the instance. The order of the result
// is the order the variables were first declared.
func layered(dist []envVar, layers ...[]envVar) (env []string, missing []string) {
	value, order := map[string]string{}, []string{}
	set := func(v envVar) {
		if _, seen := value[v.Key]; !seen {
			order = append(order, v.Key)
		}
		value[v.Key] = v.Value
	}
	required := map[string]bool{}
	for _, v := range dist {
		set(v)
		if v.Value == "" {
			required[v.Key] = true
		}
	}
	for _, layer := range layers {
		for _, v := range layer {
			set(v)
		}
	}
	for _, k := range order {
		if required[k] && value[k] == "" {
			missing = append(missing, k)
		}
		env = append(env, k+"="+value[k])
	}
	return env, missing
}
