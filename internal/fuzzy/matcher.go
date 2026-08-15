package fuzzy

import "strings"

// Matcher scores text with a fixed case sensitivity. The zero value matches
// case-insensitively, which every non-search call site relies on.
type Matcher struct{ caseSensitive bool }

// CaseSensitiveQuery reports whether the query carries an uppercase rune. This
// is the single smart-case detection rule shared by every search surface.
func CaseSensitiveQuery(query string) bool {
	return strings.ToLower(query) != query
}

// NewMatcher returns a matcher with an explicit case sensitivity.
func NewMatcher(caseSensitive bool) Matcher {
	return Matcher{caseSensitive: caseSensitive}
}

// SmartCase returns the matcher the query implies: an all-lowercase query
// matches anything, a query carrying an uppercase rune matches case-sensitively.
func SmartCase(query string) Matcher {
	return NewMatcher(CaseSensitiveQuery(query))
}

// CaseSensitive reports whether the matcher compares runes without folding.
func (m Matcher) CaseSensitive() bool {
	return m.caseSensitive
}

func (m Matcher) fold(s string) string {
	if m.caseSensitive {
		return s
	}
	return strings.ToLower(s)
}
