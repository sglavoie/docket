package tui

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
