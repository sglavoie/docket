package format

import (
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// ParseTimeField attempts to parse a time value in datetime or date-only format.
// Returns the parsed time and true if successful, zero time and false otherwise.
func ParseTimeField(value string) (time.Time, bool) {
	if t, err := time.ParseInLocation(util.DateTimeFormat, value, time.Local); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation(util.DateFormat, value, time.Local); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// escapeNewlines escapes backslashes and newlines for single-line metadata storage.
// Backslashes are escaped first so that a literal `\n` in the input becomes `\\n`,
// distinguishable from an actual newline which becomes `\n`.
func escapeNewlines(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// unescapeNewlines reverses escapeNewlines by scanning byte-by-byte.
// `\n` -> newline, `\\` -> `\`, and bare `\` before any other char is kept as-is.
// This sequential approach avoids the ambiguity that arises with chained ReplaceAll
// (e.g., `\\n` must become `\n` literal, not backslash + newline).
func unescapeNewlines(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
			case '\\':
				b.WriteByte('\\')
				i++
			default:
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// escapeChecklistText escapes semicolons and done/undone markers in checklist item text
// so they don't interfere with the storage format delimiters.
func escapeChecklistText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, "[x]", `\[x\]`)
	s = strings.ReplaceAll(s, "[ ]", `\[ \]`)
	return s
}

// unescapeChecklistText reverses the escaping done by escapeChecklistText.
func unescapeChecklistText(s string) string {
	s = strings.ReplaceAll(s, `\;`, ";")
	s = strings.ReplaceAll(s, `\[x\]`, "[x]")
	s = strings.ReplaceAll(s, `\[ \]`, "[ ]")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

// splitChecklistItems splits a checklist value string by unescaped semicolons.
func splitChecklistItems(s string) []string {
	var items []string
	var current strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			current.WriteByte(s[i])
			i++
			current.WriteByte(s[i])
		} else if s[i] == ';' {
			items = append(items, current.String())
			current.Reset()
		} else {
			current.WriteByte(s[i])
		}
	}
	items = append(items, current.String())
	return items
}

// sortedKeys returns the keys of an int map in sorted order.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
