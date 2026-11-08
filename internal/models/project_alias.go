package models

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

// MaxProjectAliasWidth bounds a project alias in terminal columns rather than
// runes or bytes. The alias exists to replace a truncated `[slug]` label in a
// row whose width is budgeted in columns, so the limit is measured in the same
// unit the row spends: an East Asian wide character costs two of them.
const MaxProjectAliasWidth = 12

// ValidateProjectAlias is the one gate a stored project alias passes. The empty
// alias is valid and means the project has none, so the slug labels it. A
// non-empty alias must already be canonical: trimmed, free of whitespace and
// control characters, free of the square brackets the row draws around it, and
// at most MaxProjectAliasWidth terminal columns wide. Validation never
// normalizes; NormalizeProjectAlias is the input-side counterpart.
func ValidateProjectAlias(alias string) error {
	if alias == "" {
		return nil
	}
	for _, r := range alias {
		switch {
		case unicode.IsSpace(r):
			return fmt.Errorf("project alias %q must not contain whitespace", alias)
		case unicode.IsControl(r):
			return fmt.Errorf("project alias %q must not contain control characters", alias)
		case r == '[' || r == ']':
			return fmt.Errorf("project alias %q must not contain square brackets", alias)
		}
	}
	width := uniseg.StringWidth(alias)
	if width == 0 {
		// A label that draws nothing would leave a row naming no project at all,
		// which is worse than the slug it replaces.
		return fmt.Errorf("project alias %q has no visible width", alias)
	}
	if width > MaxProjectAliasWidth {
		return fmt.Errorf("project alias %q is too wide (%d columns, max %d)", alias, width, MaxProjectAliasWidth)
	}
	return nil
}

// NormalizeProjectAlias trims surrounding whitespace from user input and then
// holds the result to ValidateProjectAlias, so an edit surface accepts exactly
// what the catalog will store. Interior whitespace is refused rather than
// collapsed: an alias is one token, and silently joining two words would store
// a label nobody typed.
func NormalizeProjectAlias(raw string) (string, error) {
	alias := strings.TrimSpace(raw)
	if err := ValidateProjectAlias(alias); err != nil {
		return "", err
	}
	return alias, nil
}

// ProjectDisplayLabel is the short label a surface draws for a project where
// space is scarce: its alias when one is set, else its slug. isAlias reports
// which one it is, so a caller can style or truncate the two differently; a
// slug may be long and need shortening, while an alias is already bounded by
// MaxProjectAliasWidth. A nil project has no label.
func ProjectDisplayLabel(p *Project) (label string, isAlias bool) {
	if p == nil {
		return "", false
	}
	if p.Alias != "" {
		return p.Alias, true
	}
	return p.Slug, false
}
