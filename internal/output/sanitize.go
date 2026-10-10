package output

import (
	"io"
	"strings"
)

// Sanitize makes remote text safe to print as part of one output line. Line
// breaks become a literal `\n` so message content cannot forge extra records,
// tabs become spaces so table columns hold, and the remaining control
// characters (ANSI escapes included) and bidi overrides are dropped.
func Sanitize(s string) string {
	if !strings.ContainsFunc(s, unsafeRune) {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\u2028' || r == '\u2029':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteByte(' ')
		case unsafeRune(r):
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// unsafeRune reports whether r is a control character, a line/paragraph
// separator, or a bidirectional formatting character.
func unsafeRune(r rune) bool {
	switch {
	case r < 0x20, r >= 0x7f && r <= 0x9f:
		return true
	case r == '\u2028', r == '\u2029':
		return true
	case r >= '\u202a' && r <= '\u202e', r >= '\u2066' && r <= '\u2069':
		return true
	}
	return false
}

// SafeWriter wraps a terminal-bound stream and drops raw C0 control bytes
// other than newline and tab. Item output is already sanitized per line; this
// covers the verbs that print remote names directly and API error text. JSON
// output is unaffected because encoding/json escapes control characters.
func SafeWriter(w io.Writer) io.Writer { return safeWriter{w} }

type safeWriter struct{ w io.Writer }

func (s safeWriter) Write(p []byte) (int, error) {
	clean := p
	for i, c := range p {
		if (c < 0x20 && c != '\n' && c != '\t') || c == 0x7f {
			clean = make([]byte, 0, len(p))
			clean = append(clean, p[:i]...)
			for _, c := range p[i:] {
				if (c < 0x20 && c != '\n' && c != '\t') || c == 0x7f {
					continue
				}
				clean = append(clean, c)
			}
			break
		}
	}
	if _, err := s.w.Write(clean); err != nil {
		return 0, err
	}
	return len(p), nil
}
