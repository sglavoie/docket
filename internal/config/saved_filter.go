package config

import "github.com/google/uuid"

// savedView is retained only as an input shape for migrating early planning
// domain files into SavedFilter entries.
type savedView struct {
	Name              string            `json:"name"`
	ViewMode          int               `json:"view_mode,omitempty"`
	FilterType        string            `json:"filter_type,omitempty"`
	FilterStatus      string            `json:"filter_status,omitempty"`
	FilterPriority    int               `json:"filter_priority,omitempty"`
	FilterTags        []string          `json:"filter_tags,omitempty"`
	FilterSections    []string          `json:"filter_sections,omitempty"`
	FilterDueDate     int               `json:"filter_due_date,omitempty"`
	FilterActionable  bool              `json:"filter_actionable,omitempty"`
	FilterGitHubIssue bool              `json:"filter_github_issue,omitempty"`
	FilterParked      bool              `json:"filter_parked,omitempty"`
	FilterAttrs       map[string]string `json:"filter_attrs,omitempty"`
	FilterScoreMin    float64           `json:"filter_score_min,omitempty"`
	SortField         int               `json:"sort_field,omitempty"`
	SortAscending     bool              `json:"sort_ascending"`
	PinnedTagTab      string            `json:"pinned_tag_tab,omitempty"`
}

// SavedFilter is a unified filter preset that replaces both savedView (flat
// filter snapshots) and smartList (compound criteria).  Mode selects which
// field set is active: "snapshot" or "criteria".
type SavedFilter struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// Mode: "snapshot" (flat filters from savedView) or "criteria" (compound rules from smartList).
	Mode string `json:"mode"`

	// --- Snapshot mode (from savedView) ---
	FilterType        string            `json:"filter_type,omitempty"`
	FilterStatus      string            `json:"filter_status,omitempty"`
	FilterPriority    int               `json:"filter_priority,omitempty"`
	FilterTags        []string          `json:"filter_tags,omitempty"`
	FilterSections    []string          `json:"filter_sections,omitempty"`
	FilterDueDate     int               `json:"filter_due_date,omitempty"`
	FilterActionable  bool              `json:"filter_actionable,omitempty"`
	FilterHideDimmed  bool              `json:"filter_hide_dimmed,omitempty"`
	FilterGitHubIssue bool              `json:"filter_github_issue,omitempty"`
	FilterParked      bool              `json:"filter_parked,omitempty"`
	FilterAttrs       map[string]string `json:"filter_attrs,omitempty"`
	FilterScoreMin    float64           `json:"filter_score_min,omitempty"`

	// --- Criteria mode (from smartList) ---
	Criteria         []SmartListCriterion `json:"criteria,omitempty"`
	Logic            string               `json:"logic,omitempty"`
	IncludeCompleted bool                 `json:"include_completed,omitempty"`
}

// EffectiveLogic returns the logic operator for criteria mode, defaulting to "and".
func (f *SavedFilter) EffectiveLogic() string {
	if f.Logic == "or" {
		return "or"
	}
	return "and"
}

// --- Migration helpers ---

// savedViewToFilter converts a legacy savedView into a SavedFilter with mode "snapshot".
func savedViewToFilter(sv savedView) SavedFilter {
	return SavedFilter{
		ID:                uuid.New().String(),
		Name:              sv.Name,
		Mode:              "snapshot",
		FilterType:        sv.FilterType,
		FilterStatus:      sv.FilterStatus,
		FilterPriority:    sv.FilterPriority,
		FilterTags:        sv.FilterTags,
		FilterSections:    sv.FilterSections,
		FilterDueDate:     sv.FilterDueDate,
		FilterActionable:  sv.FilterActionable,
		FilterGitHubIssue: sv.FilterGitHubIssue,
		FilterParked:      sv.FilterParked,
		FilterAttrs:       sv.FilterAttrs,
		FilterScoreMin:    sv.FilterScoreMin,
	}
}

// smartListToFilter converts a legacy smartList into a SavedFilter with mode "criteria".
// A missing or empty legacy ID is replaced with a fresh UUID so the migrated
// filter is uniquely addressable; otherwise multiple legacy entries without
// IDs would collide and ID lookups would be ambiguous.
func smartListToFilter(sl smartList) SavedFilter {
	id := sl.ID
	if id == "" {
		id = uuid.New().String()
	}
	return SavedFilter{
		ID:               id,
		Name:             sl.Name,
		Mode:             "criteria",
		Criteria:         sl.Criteria,
		Logic:            sl.Logic,
		IncludeCompleted: sl.IncludeCompleted,
	}
}

// MigrateLegacyFilters converts savedViews and smartLists to SavedFilters.
// It is idempotent: if both legacy slices are already empty nothing changes.
// Any migrated smartList whose preserved ID would collide with an existing
// SavedFilter (or another smartList) receives a fresh UUID so ID-based
// lookups remain unambiguous.
// Returns true if a migration was performed.
func (p *PlanningState) MigrateLegacyFilters() bool {
	if len(p.SavedViews) == 0 && len(p.SmartLists) == 0 {
		return false
	}

	seenIDs := make(map[string]bool, len(p.SavedFilters)+len(p.SavedViews)+len(p.SmartLists))
	for _, f := range p.SavedFilters {
		if f.ID != "" {
			seenIDs[f.ID] = true
		}
	}

	for _, sv := range p.SavedViews {
		f := savedViewToFilter(sv)
		// savedViewToFilter always mints a fresh UUID, but be defensive in
		// case of an astronomical UUID collision.
		for seenIDs[f.ID] {
			f.ID = uuid.New().String()
		}
		seenIDs[f.ID] = true
		p.SavedFilters = append(p.SavedFilters, f)
	}
	for _, sl := range p.SmartLists {
		f := smartListToFilter(sl)
		for seenIDs[f.ID] {
			f.ID = uuid.New().String()
		}
		seenIDs[f.ID] = true
		p.SavedFilters = append(p.SavedFilters, f)
	}

	p.SavedViews = nil
	p.SmartLists = nil
	return true
}
