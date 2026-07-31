package util

import (
	"runtime"
	"strings"
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
				if s[i] == '\\' && i+1 < len(s) && s[i+1] == '\n' {
					i += 2 // shell line continuation: remove backslash-newline
					continue
				}
				if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\') {
					i++ // skip backslash; s[i] is now the escaped char
				}
				cur.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				i++ // consume closing quote
			}
		case c == '\\' && i+1 < len(s) && s[i+1] == '\n':
			i += 2 // shell line continuation: remove backslash-newline
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

// NormalizePastedPathInput normalizes pasted shell-style single-path input into
// a plain filesystem path. It collapses paths that a shell or file manager copied
// with quoting ('…' or "…") or backslash-escaped spaces down to the underlying
// path, using ShellTokenize to match quoted or shell-escaped single-token input.
//
// Ordinary unquoted input is returned unchanged: paths with spaces that tokenize
// into multiple tokens are preserved verbatim, as are unquoted paths containing
// stray quote characters (e.g. an apostrophe) that shell tokenization would
// misinterpret. On Windows, unquoted backslashes are path separators rather than
// shell escapes, so only quote-wrapped input is normalized there.
func NormalizePastedPathInput(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return trimmed
	}
	wrapped := isQuoteWrapped(trimmed)
	if runtime.GOOS == "windows" && !wrapped {
		// Don't treat ordinary unquoted backslashes as shell escapes on Windows.
		return trimmed
	}
	if !wrapped && containsUnescapedQuote(trimmed) {
		// Unquoted input with stray quote chars would be misread by the shell
		// tokenizer, so preserve the original path unchanged.
		return trimmed
	}
	if tokens := ShellTokenize(trimmed); len(tokens) == 1 && tokens[0] != "" {
		return tokens[0]
	}
	return trimmed
}

// isQuoteWrapped reports whether s begins and ends with the same quote character.
func isQuoteWrapped(s string) bool {
	if len(s) < 2 {
		return false
	}
	q := s[0]
	return (q == '\'' || q == '"') && s[len(s)-1] == q
}

func containsUnescapedQuote(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '\'' && s[i] != '"' {
			continue
		}
		backslashes := 0
		for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return true
		}
	}
	return false
}
