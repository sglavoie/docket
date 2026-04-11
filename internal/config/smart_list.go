package config

// SmartListCriterion represents a single filter criterion.
// Used by both the legacy smartList type and the unified SavedFilter.
type SmartListCriterion struct {
	Field     string `json:"field"`     // type, status, priority, tags, due_date, section, score_min, actionable, created_days, updated_days, completed_days
	Operation string `json:"operation"` // equals, not_equals, contains, greater_than, less_than
	Value     string `json:"value"`
}

// smartList is a legacy type kept for JSON deserialization during migration.
// New code should use SavedFilter instead.
type smartList struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Criteria         []SmartListCriterion `json:"criteria"`
	Logic            string               `json:"logic,omitempty"` // "and" (default) or "or"
	SortField        int                  `json:"sort_field,omitempty"`
	SortAscending    bool                 `json:"sort_ascending"`
	IncludeCompleted bool                 `json:"include_completed,omitempty"`
}

// EffectiveLogic returns the logic operator, defaulting to "and".
func (s *smartList) EffectiveLogic() string {
	if s.Logic == "or" {
		return "or"
	}
	return "and"
}
