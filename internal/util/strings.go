package util

import (
	"strings"
	"unicode/utf8"
)

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

// ContainsIgnoreCase checks if haystack contains needle (case-insensitive).
func ContainsIgnoreCase(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
