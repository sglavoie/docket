package util

import "strings"

// Truncate truncates a string to maxLen characters, adding "..." if truncated.
// If maxLen is less than 4, the string is truncated without ellipsis.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ContainsIgnoreCase checks if haystack contains needle (case-insensitive).
func ContainsIgnoreCase(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
