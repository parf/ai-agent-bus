package envfile

import (
	"strings"
	"unicode/utf8"
)

// Declaration contains documentation only. No assignment value can leave here.
type Declaration struct {
	Name       string `json:"name"`
	Help       string `json:"help"`
	Required   bool   `json:"required"`
	HasDefault bool   `json:"has_default"`
}

func Describe(src []byte) ([]Declaration, error) {
	vars, err := Parse(src)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(src), "\r\n", "\n"), "\r", "\n")
	out := make([]Declaration, 0, len(vars))
	offset, previousValueEnd, i := 0, 0, 0
	for offset < len(text) {
		rest := text[offset:]
		m := linePattern.FindStringSubmatchIndex(rest)
		if m != nil {
			// The dotenv matcher can consume next-line comments as trailing whitespace.
			// Recover documentation from the original lines before this key, but never
			// cross into the preceding assignment's (possibly multiline) value.
			end := strings.LastIndexFunc(text[:offset+m[2]], lineEnd)
			comments := []string{}
			for end >= previousValueEnd && end >= 0 {
				previous := strings.LastIndexFunc(text[:end], lineEnd)
				start := 0
				if previous >= 0 {
					_, size := utf8.DecodeRuneInString(text[previous:])
					start = previous + size
				}
				if start < previousValueEnd {
					break
				}
				line := trim(text[start:end])
				if !strings.HasPrefix(line, "#") {
					break
				}
				comments = append([]string{strings.TrimSpace(strings.TrimPrefix(line, "#"))}, comments...)
				end = previous
			}
			out = append(out, Declaration{Name: vars[i].Key, Help: strings.Join(comments, "\n"), Required: vars[i].Required, HasDefault: vars[i].Value != ""})
			previousValueEnd = offset + m[3]
			if m[4] >= 0 {
				previousValueEnd = offset + m[5]
			}
			i++
			offset += m[1]
			continue
		}
		end := strings.IndexFunc(rest, lineEnd)
		if end < 0 {
			break
		}
		_, size := utf8.DecodeRuneInString(rest[end:])
		offset += end + size
	}
	return out, nil
}
