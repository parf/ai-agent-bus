package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The env files are the common dotenv format, read here rather than by a
// library because the libraries expand $VAR, and a secret is what is written
// (Plans/R1.0-Release/runner-config.md#file-formats):
//
//	# a comment             KEY=value  # a comment after it
//	export KEY=value        KEY='literal, # and $ included'
//	KEY="with \n \" \\ escapes"         KEY=
//
// One value per line; a quoted value ends on its own line.

type envVar struct{ Key, Value string }

var envKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// parseEnv reads one env file's variables in order, naming the file and the
// line of anything it cannot read.
func parseEnv(name string, src []byte) ([]envVar, error) {
	var out []envVar
	for i, line := range strings.Split(string(src), "\n") {
		where := fmt.Sprintf("%s:%d", name, i+1)
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, raw, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok {
			return nil, fmt.Errorf("%s: %q has no =; a required variable is KEY= with nothing after it", where, line)
		}
		if !envKey.MatchString(key) {
			return nil, fmt.Errorf("%s: %q is not a variable name", where, key)
		}
		value, err := envValue(strings.TrimLeft(raw, " \t"))
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", where, key, err)
		}
		out = append(out, envVar{key, value})
	}
	return out, nil
}

// envValue is one value as written: single quotes literal, double quotes with
// \n \t \" \\ escapes, and an unquoted one up to a # after whitespace.
func envValue(raw string) (string, error) {
	rest := ""
	var value string
	switch {
	case strings.HasPrefix(raw, "'"):
		end := strings.IndexByte(raw[1:], '\'')
		if end < 0 {
			return "", errors.New("the ' is never closed")
		}
		value, rest = raw[1:1+end], raw[2+end:]
	case strings.HasPrefix(raw, `"`):
		var b strings.Builder
		i := 1
		for ; i < len(raw) && raw[i] != '"'; i++ {
			if raw[i] == '\\' && i+1 < len(raw) {
				i++
				switch raw[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case '"', '\\':
					b.WriteByte(raw[i])
				default:
					b.WriteByte('\\')
					b.WriteByte(raw[i])
				}
				continue
			}
			b.WriteByte(raw[i])
		}
		if i >= len(raw) {
			return "", errors.New(`the " is never closed`)
		}
		value, rest = b.String(), raw[i+1:]
	default:
		for i := 0; i < len(raw); i++ {
			if raw[i] == '#' && (i == 0 || raw[i-1] == ' ' || raw[i-1] == '\t') {
				return strings.TrimRight(raw[:i], " \t"), nil
			}
		}
		return strings.TrimRight(raw, " \t"), nil
	}
	if rest = strings.TrimSpace(rest); rest != "" && !strings.HasPrefix(rest, "#") {
		return "", fmt.Errorf("%q after the closing quote", rest)
	}
	return value, nil
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
