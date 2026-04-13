package tui

import "sort"

// sortPrefixHintsLowercaseFirst sorts hints in-place: lowercase keys first
// (alphabetically), then non-letter keys, then uppercase keys (alphabetically).
func sortPrefixHintsLowercaseFirst(hints []PrefixHint) {
	sort.SliceStable(hints, func(i, j int) bool {
		a, b := hints[i].Key, hints[j].Key
		aLower := len(a) > 0 && a[0] >= 'a' && a[0] <= 'z'
		bLower := len(b) > 0 && b[0] >= 'a' && b[0] <= 'z'
		aUpper := len(a) > 0 && a[0] >= 'A' && a[0] <= 'Z'
		bUpper := len(b) > 0 && b[0] >= 'A' && b[0] <= 'Z'
		// lowercase < non-letter < uppercase
		aRank := 1 // non-letter
		if aLower {
			aRank = 0
		} else if aUpper {
			aRank = 2
		}
		bRank := 1
		if bLower {
			bRank = 0
		} else if bUpper {
			bRank = 2
		}
		if aRank != bRank {
			return aRank < bRank
		}
		return a < b
	})
}

// PrefixHint represents a single key hint in a prefix popup.
type PrefixHint struct {
	Key          string
	Description  string
	Detail       string
	Active       bool   // When true, description and detail are rendered in orange
	Disabled     bool   // When true, all columns are rendered dimmed (muted)
	RenderedIcon string // Pre-styled icon string prepended before the description column
}

// PrefixHints maps prefix characters to their available key hints.
type PrefixHints map[string][]PrefixHint

// PrefixPopupInfo holds everything needed to render a prefix popup overlay.
type PrefixPopupInfo struct {
	Prefix string
	Hints  PrefixHints
}

// PrefixPopupProvider is implemented by view models that support prefix key popups.
type PrefixPopupProvider interface {
	GetPrefixPopupInfo() *PrefixPopupInfo
}

// prefixCategoryNames maps prefix characters to human-readable category names.
var prefixCategoryNames = map[string]string{
	",":   "status",
	"z":   "fold",
	"g":   "go-to",
	"m":   "move",
	"!":   "priority",
	"d":   "due-date",
	"s":   "snooze",
	"c":   "create",
	"t":   "task",
	"b":   "blockers",
	"P":   "perspectives",
	"F":   "filter",
	"Ft":  "filter-type",
	"Fs":  "filter-status",
	"Fd":  "filter-due",
	"Fa":  "filter-attr",
	"Fas": "severity",
	"Far": "reproducible",
	"Fae": "effort",
	"Fac": "category",
	"Fav": "value",
	"Faw": "worktype",
	"Fao": "scope",
	"Fap": "points",
	"FS":  "filter-score",
	"F.":  "filter-stage",
	"Fv":  "saved-filters",
	".":   "stage",
	";":   "tt",
}
