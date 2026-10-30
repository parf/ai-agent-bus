// Adapted from motdotla/dotenv; copyright Scott Motte, BSD-2-Clause (licenses/NOTICE.md).
// Package envfile parses the official dotenv format without evaluating values.
// Unreadable non-comment lines are refused without repeating private bytes.
package envfile

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Var is an assignment in source order; consumers apply later duplicates last.
type Var struct {
	Key, Value string
	Required   bool `json:",omitempty"`
}

// JavaScript's whitespace class differs from Go's (notably BOM and NEL).
const whitespace = `[\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

// Follow parseRegex's alternative order. Consume the line terminator so the
// next match starts at a JavaScript line boundary, including U+2028/U+2029.
var linePattern = regexp.MustCompile(strings.ReplaceAll(`\A\s*(?:export\s+)?([\w.-]+)(?:\s*=\s*?|:\s+?)(\s*'(?:\\'|[^'])*'|\s*"(?:\\"|[^"])*"|\s*`+"`(?:"+`\\`+"`|[^`])*`"+`|[^#\r\n]+)?\s*(?:#([^\n\x{2028}\x{2029}]*))?(?:\z|[\n\x{2028}\x{2029}])`, `\s`, whitespace))

// A reference as dotenvx expands one — $NAME, ${…} or $(…) — in an unquoted
// or double-quoted value. Nothing is expanded here, so every reference is an
// unknown one and is refused; single quotes and backticks keep a $ literal.
var referencePattern = regexp.MustCompile(`\$[A-Za-z_{(]`)

var whitespacePattern = regexp.MustCompile(`\A` + whitespace + `+|` + whitespace + `+\z`)

func trim(s string) string { return whitespacePattern.ReplaceAllString(s, "") }
func lineEnd(r rune) bool  { return r == '\n' || r == '\u2028' || r == '\u2029' }

// Parse keeps dotenv's accepted bytes and reports the first line it would skip.
func Parse(src []byte) ([]Var, error) {
	text := strings.ReplaceAll(strings.ReplaceAll(string(src), "\r\n", "\n"), "\r", "\n")
	if at := strings.IndexByte(text, 0); at >= 0 {
		return nil, fmt.Errorf("line %d: NUL cannot be delivered in an environment", lineNumber(text[:at]))
	}
	var out []Var
	offset := 0
	for offset < len(text) {
		rest := text[offset:]
		m := linePattern.FindStringSubmatchIndex(rest)
		if m == nil {
			end := strings.IndexFunc(rest, lineEnd)
			if end < 0 {
				end = len(rest)
			}
			if v := trim(rest[:end]); v != "" && !strings.HasPrefix(v, "#") {
				return nil, fmt.Errorf("line %d: expected a dotenv assignment", lineNumber(text[:offset]))
			}
			if end == len(rest) {
				break
			}
			_, size := utf8.DecodeRuneInString(rest[end:])
			offset += end + size
			continue
		}
		key := rest[m[2]:m[3]]
		value := ""
		if m[4] >= 0 {
			value = trim(rest[m[4]:m[5]])
		}
		double := strings.HasPrefix(value, `"`)
		if !strings.HasPrefix(value, "'") && !strings.HasPrefix(value, "`") && strings.IndexByte(value, '$') >= 0 && referencePattern.MatchString(value) {
			return nil, fmt.Errorf("line %d: a reference ($NAME, ${…} or $(…)) is never expanded; put the value in '…' to keep a $", lineNumber(text[:offset+m[2]]))
		}
		value = unquote(value)
		if double {
			value = strings.ReplaceAll(strings.ReplaceAll(value, `\n`, "\n"), `\r`, "\r")
		}
		required := false
		if m[6] >= 0 {
			end := m[3]
			if m[4] >= 0 {
				raw := rest[m[4]:m[5]]
				if v := trim(raw); v != "" {
					end = m[4] + strings.LastIndex(raw, v) + len(v)
				}
			}
			// A comment on the following line is documentation, not this key's marker.
			if strings.IndexFunc(rest[end:m[6]-1], lineEnd) < 0 {
				words := strings.Fields(rest[m[6]:m[7]])
				required = len(words) > 0 && words[0] == "required"
			}
		}
		out = append(out, Var{Key: key, Value: value, Required: required})
		offset += m[1]
	}
	return out, nil
}

func lineNumber(s string) int {
	n := 1
	for _, r := range s {
		if lineEnd(r) {
			n++
		}
	}
	return n
}

// dotenv removes matching quote pairs at JavaScript multiline boundaries,
// including quoted segments in an otherwise unquoted fallback value.
// A pair opening at a line start closes at the last quote of its kind that
// ends a line, so each kind's last such quote is found once, up front:
// searching backwards from every opening made a long value quadratic.
func unquote(s string) string {
	var last [256]int // a closing quote's index plus one; 0 is none
	for j := 0; j < len(s); j++ {
		if q := s[j]; q == '\'' || q == '"' || q == '`' {
			if r, _ := utf8.DecodeRuneInString(s[j+1:]); j+1 == len(s) || lineEnd(r) {
				last[q] = j + 1
			}
		}
	}
	var out strings.Builder
	copied := 0
	for i := 0; i < len(s); i++ {
		if i > 0 {
			r, _ := utf8.DecodeLastRuneInString(s[:i])
			if !lineEnd(r) {
				continue
			}
		}
		q := s[i]
		if q != '\'' && q != '"' && q != '`' {
			continue
		}
		j := last[q] - 1
		if j <= i {
			continue
		}
		out.WriteString(s[copied:i])
		out.WriteString(s[i+1 : j])
		copied = j + 1
		i = j
	}
	out.WriteString(s[copied:])
	return out.String()
}
