package tui

var (
	// FilterTypePrefixHints contains hints for the "Ft" (filter type) sub-prefix.
	FilterTypePrefixHints = []PrefixHint{
		{Key: "e", Description: "epic"},
		{Key: "s", Description: "story"},
		{Key: "t", Description: "task"},
		{Key: "b", Description: "bug"},
		{Key: "i", Description: "idea"},
		{Key: "0", Description: "clear"},
	}

	// FilterStatusPrefixHints contains hints for the "Fs" (filter status) sub-prefix.
	FilterStatusPrefixHints = []PrefixHint{
		{Key: "t", Description: "todo"},
		{Key: "s", Description: "in-progress"},
		{Key: "r", Description: "review"},
		{Key: "S", Description: "standby"},
		{Key: "b", Description: "blocked"},
		{Key: "d", Description: "done"},
		{Key: "x", Description: "canceled"},
		{Key: "0", Description: "clear"},
	}

	// FilterDuePrefixHints contains hints for the "Fd" (filter due-date) sub-prefix.
	FilterDuePrefixHints = []PrefixHint{
		{Key: "o", Description: "overdue"},
		{Key: "t", Description: "today"},
		{Key: "w", Description: "this-week"},
		{Key: "n", Description: "no-due-date"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrPrefixHints contains hints for the "Fa" (filter attribute) sub-prefix.
	FilterAttrPrefixHints = []PrefixHint{
		{Key: "s", Description: "severity", Detail: "Bug severity (critical/major/minor/cosmetic)"},
		{Key: "r", Description: "reproducible", Detail: "Bug reproducibility (yes/no/sometimes)"},
		{Key: "e", Description: "effort", Detail: "Effort estimate (small/medium/large/unknown)"},
		{Key: "c", Description: "category", Detail: "Idea category (feature/improvement/refactor/research)"},
		{Key: "v", Description: "value", Detail: "Idea value (low/medium/high)"},
		{Key: "V", Description: "validated", Detail: "Idea validated flag"},
		{Key: "w", Description: "worktype", Detail: "Task work type (development/documentation/testing/devops/design)"},
		{Key: "o", Description: "scope", Detail: "Epic scope (defined/evolving/undefined)"},
		{Key: "p", Description: "points", Detail: "Story points (1/2/3/5/8)"},
		{Key: "0", Description: "clear", Detail: "Clear all attribute filters"},
	}

	// FilterAttrSeverityPrefixHints contains hints for bug severity values.
	FilterAttrSeverityPrefixHints = []PrefixHint{
		{Key: "c", Description: "critical"},
		{Key: "j", Description: "major"},
		{Key: "m", Description: "minor"},
		{Key: "o", Description: "cosmetic"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrReproduciblePrefixHints contains hints for bug reproducible values.
	FilterAttrReproduciblePrefixHints = []PrefixHint{
		{Key: "y", Description: "yes"},
		{Key: "n", Description: "no"},
		{Key: "s", Description: "sometimes"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrEffortPrefixHints contains hints for effort estimate values.
	FilterAttrEffortPrefixHints = []PrefixHint{
		{Key: "s", Description: "small"},
		{Key: "m", Description: "medium"},
		{Key: "l", Description: "large"},
		{Key: "u", Description: "unknown"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrCategoryPrefixHints contains hints for idea category values.
	FilterAttrCategoryPrefixHints = []PrefixHint{
		{Key: "f", Description: "feature"},
		{Key: "i", Description: "improvement"},
		{Key: "r", Description: "refactor"},
		{Key: "s", Description: "research"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrValuePrefixHints contains hints for idea value values.
	FilterAttrValuePrefixHints = []PrefixHint{
		{Key: "l", Description: "low"},
		{Key: "m", Description: "medium"},
		{Key: "h", Description: "high"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrWorktypePrefixHints contains hints for task worktype values.
	FilterAttrWorktypePrefixHints = []PrefixHint{
		{Key: "d", Description: "development"},
		{Key: "o", Description: "documentation"},
		{Key: "t", Description: "testing"},
		{Key: "v", Description: "devops"},
		{Key: "e", Description: "design"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrScopePrefixHints contains hints for epic scope values.
	FilterAttrScopePrefixHints = []PrefixHint{
		{Key: "d", Description: "defined"},
		{Key: "e", Description: "evolving"},
		{Key: "u", Description: "undefined"},
		{Key: "0", Description: "clear"},
	}

	// FilterAttrPointsPrefixHints contains hints for story points values.
	FilterAttrPointsPrefixHints = []PrefixHint{
		{Key: "1", Description: "1 point"},
		{Key: "2", Description: "2 points"},
		{Key: "3", Description: "3 points"},
		{Key: "5", Description: "5 points"},
		{Key: "8", Description: "8 points"},
		{Key: "0", Description: "clear"},
	}

	// SavedFilterPrefixHints contains hints for the "Fv" (saved filters/filters) sub-prefix.
	SavedFilterPrefixHints = []PrefixHint{
		{Key: "s", Description: "save", Detail: "Save current view"},
		{Key: "l", Description: "load", Detail: "Browse/apply saved filters"},
		{Key: "d", Description: "delete", Detail: "Delete a saved filter"},
		{Key: "c", Description: "criteria", Detail: "Create criteria filter"},
	}
)
