package envfile

import "strings"

// Declaration contains documentation only. No assignment value can leave here.
type Declaration struct {
	Name     string `json:"name"`
	Help     string `json:"help"`
	Required bool   `json:"required"`
}

func Describe(src []byte) ([]Declaration, error) {
	vars, err := Parse(src)
	if err != nil {
		return nil, err
	}
	text := strings.NewReplacer("\r\n", "\n", "\r", "\n", "\u2028", "\n", "\u2029", "\n").Replace(string(src))
	out := make([]Declaration, 0, len(vars))
	offset, previousValueEnd, i := 0, 0, 0
	for offset < len(text) {
		rest := text[offset:]
		m := linePattern.FindStringSubmatchIndex(rest)
		if m != nil {
			// The dotenv matcher can consume next-line comments as trailing whitespace.
			// Recover documentation from the original lines before this key, but never
			// cross into the preceding assignment's (possibly multiline) value.
			end := strings.LastIndex(text[:offset+m[2]], "\n")
			comments := []string{}
			for end >= previousValueEnd && end >= 0 {
				start := strings.LastIndex(text[:end], "\n") + 1
				if start < previousValueEnd {
					break
				}
				line := trim(text[start:end])
				if !strings.HasPrefix(line, "#") {
					break
				}
				comments = append([]string{strings.TrimSpace(strings.TrimPrefix(line, "#"))}, comments...)
				end = start - 1
			}
			out = append(out, Declaration{Name: vars[i].Key, Help: strings.Join(comments, "\n"), Required: vars[i].Value == ""})
			previousValueEnd = offset + m[3]
			if m[4] >= 0 {
				previousValueEnd = offset + m[5]
			}
			i++
			offset += m[1]
			continue
		}
		end := strings.IndexByte(rest, '\n')
		if end < 0 {
			break
		}
		offset += end + 1
	}
	return out, nil
}
