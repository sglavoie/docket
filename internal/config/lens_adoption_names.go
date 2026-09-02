package config

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// Naming is the merge's last stage: identities are already settled, so this
// half only decides which label each resolved record wears.

// assignLensNames gives every resolved record a label, in an order derived from
// the records themselves: a record that kept an identity it was contributed
// under is labeled before one that had to derive a new identity, and canonical
// id order decides within each half. A record takes its preferred label when
// nothing holds it and otherwise the first free " (N)" variant of that label's
// base, so a record already carrying a disambiguated label settles back onto it
// instead of accumulating suffixes.
func assignLensNames(records []*resolvedLensRecord) {
	ordered := append([]*resolvedLensRecord(nil), records...)
	sort.Slice(ordered, func(i, j int) bool {
		leftDerived, rightDerived := len(ordered[i].own) == 0, len(ordered[j].own) == 0
		if leftDerived != rightDerived {
			return rightDerived
		}
		return ordered[i].id < ordered[j].id
	})
	taken := make(map[string]bool, len(ordered))
	for _, record := range ordered {
		preferred := preferredLensName(record)
		if folded := strings.ToLower(preferred); !taken[folded] {
			record.name = preferred
			taken[folded] = true
			continue
		}
		record.name = unusedLensName(baseLensName(preferred), taken)
		taken[strings.ToLower(record.name)] = true
	}
}

// preferredLensName is the label a resolved record asks for. Labels contributed
// under the record's own identity win over labels a re-identified contribution
// carried, which is what keeps a settled collection settled: a machine still
// holding the pre-merge spelling of a record it has since re-identified no
// longer drags that spelling back. Case-folded order decides between equals, so
// the choice is a function of the label set and not of arrival.
func preferredLensName(record *resolvedLensRecord) string {
	candidates := record.own
	if len(candidates) == 0 {
		candidates = record.names
	}
	best := candidates[0]
	for _, name := range candidates[1:] {
		if lensNameLess(name, best) {
			best = name
		}
	}
	return best
}

func lensNameLess(a, b string) bool {
	foldedA, foldedB := strings.ToLower(a), strings.ToLower(b)
	if foldedA != foldedB {
		return foldedA < foldedB
	}
	return a < b
}

// baseLensName strips a trailing " (N)" disambiguation suffix so a record that
// already carries one asks for the same series again rather than nesting a
// second suffix inside the first. A name that does not end in one is its own
// base, and the suffix is only stripped when something is left in front of it.
func baseLensName(name string) string {
	trimmed := strings.TrimSuffix(name, ")")
	if trimmed == name {
		return name
	}
	open := strings.LastIndex(trimmed, " (")
	if open <= 0 {
		return name
	}
	digits := trimmed[open+2:]
	if digits == "" {
		return name
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return name
		}
	}
	return trimmed[:open]
}

// unusedLensName returns name, or the first " (N)" variant of it that is not
// already taken. The base is truncated when the suffix would push it past the
// durable rune boundary, so a disambiguated name still validates.
func unusedLensName(name string, taken map[string]bool) string {
	if !taken[strings.ToLower(name)] {
		return name
	}
	for n := 2; ; n++ {
		suffix := fmt.Sprintf(" (%d)", n)
		candidate := truncateLensNameRunes(name, MaxNamedLensNameRunes-utf8.RuneCountInString(suffix)) + suffix
		if !taken[strings.ToLower(candidate)] {
			return candidate
		}
	}
}

// truncateLensNameRunes cuts a name to at most limit runes. A trimmed name can
// only lose a trailing rune, never gain leading whitespace, so the result is
// still trimmed and still non-empty for any positive limit.
func truncateLensNameRunes(name string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(name) <= limit {
		return name
	}
	runes := []rune(name)
	return strings.TrimRight(string(runes[:limit]), " ")
}
