package util

import (
	"strings"
	"unicode/utf8"
)

// ShellTokenize splits a shell-like command string into tokens, handling:
//   - single-quoted strings ('...'): literal, no escaping inside
//   - double-quoted strings ("..."): \" and \\ are unescaped, other \X is literal
//   - backslash escaping outside quotes (\X → X)
//
// This is intentionally minimal — it covers practical editor-command cases
// (quoted paths with spaces, flag values) without implementing a full POSIX
// shell parser. The result matches what a POSIX shell produces for simple,
// non-interactive command strings.
func ShellTokenize(s string) []string {
	var tokens []string
	var cur strings.Builder
	inToken := false
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\'':
			inToken = true
			i++ // consume opening quote
			for i < len(s) && s[i] != '\'' {
				cur.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				i++ // consume closing quote
			}
		case c == '"':
			inToken = true
			i++ // consume opening quote
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\') {
					i++ // skip backslash; s[i] is now the escaped char
				}
				cur.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				i++ // consume closing quote
			}
		case c == '\\' && i+1 < len(s):
			inToken = true
			i++ // skip backslash
			cur.WriteByte(s[i])
			i++
		case c == '\\':
			inToken = true
			cur.WriteByte(c)
			i++
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if inToken {
				tokens = append(tokens, cur.String())
				cur.Reset()
				inToken = false
			}
			i++
		default:
			inToken = true
			cur.WriteByte(c)
			i++
		}
	}
	if inToken {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// Truncate truncates a string to maxLen runes, adding "..." if truncated.
// If maxLen is less than 4, the string is truncated without ellipsis.
// Uses rune counting to avoid splitting multi-byte UTF-8 characters.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return runeSlice(s, maxLen)
	}
	return runeSlice(s, maxLen-3) + "..."
}

// runeSlice returns the first n runes of s as a string.
func runeSlice(s string, n int) string {
	i := 0
	for j := 0; j < n && i < len(s); j++ {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return s[:i]
}
