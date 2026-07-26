package config

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

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
	FilterType        string                  `json:"filter_type,omitempty"`
	FilterStates      models.WorkflowStateSet `json:"-"`
	FilterPriority    int                     `json:"filter_priority,omitempty"`
	FilterTags        []string                `json:"filter_tags,omitempty"`
	FilterDueDate     int                     `json:"filter_due_date,omitempty"`
	FilterActionable  bool                    `json:"filter_actionable,omitempty"`
	FilterHideDimmed  bool                    `json:"filter_hide_dimmed,omitempty"`
	FilterGitHubIssue bool                    `json:"filter_github_issue,omitempty"`
	FilterParked      bool                    `json:"filter_parked,omitempty"`
	FilterAttrs       map[string]string       `json:"filter_attrs,omitempty"`
	FilterScoreMin    float64                 `json:"filter_score_min,omitempty"`

	// --- Criteria mode (from smartList) ---
	Criteria         []SmartListCriterion `json:"criteria,omitempty"`
	Logic            string               `json:"logic,omitempty"`
	IncludeCompleted bool                 `json:"include_completed,omitempty"`

	filterStatesSet        bool
	legacyLifecycleDecoded bool
}

type savedFilterWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Mode string `json:"mode"`

	FilterType        string               `json:"filter_type,omitempty"`
	FilterStates      json.RawMessage      `json:"filter_states,omitempty"`
	FilterStatus      string               `json:"filter_status,omitempty"`
	FilterPriority    int                  `json:"filter_priority,omitempty"`
	FilterTags        []string             `json:"filter_tags,omitempty"`
	FilterSections    []string             `json:"filter_sections,omitempty"`
	FilterDueDate     int                  `json:"filter_due_date,omitempty"`
	FilterActionable  bool                 `json:"filter_actionable,omitempty"`
	FilterHideDimmed  bool                 `json:"filter_hide_dimmed,omitempty"`
	FilterGitHubIssue bool                 `json:"filter_github_issue,omitempty"`
	FilterParked      bool                 `json:"filter_parked,omitempty"`
	FilterAttrs       map[string]string    `json:"filter_attrs,omitempty"`
	FilterScoreMin    float64              `json:"filter_score_min,omitempty"`
	Criteria          []SmartListCriterion `json:"criteria,omitempty"`
	Logic             string               `json:"logic,omitempty"`
	IncludeCompleted  bool                 `json:"include_completed,omitempty"`
}

// HasStateFilter reports whether filter_states is an active predicate. It is
// distinct from Len so a migrated contradictory legacy status+section pair can
// remain an explicit match-nothing filter.
func (f SavedFilter) HasStateFilter() bool {
	return f.filterStatesSet || !f.FilterStates.IsEmpty()
}

// SetFilterStates assigns a defensive copy of the canonical state predicate.
func (f *SavedFilter) SetFilterStates(states models.WorkflowStateSet, active bool) {
	f.FilterStates = cloneWorkflowStateSet(states)
	f.filterStatesSet = active || !states.IsEmpty()
}

// Validate checks the canonical lifecycle portions of a saved filter.
func (f SavedFilter) Validate() error {
	for i, criterion := range f.Criteria {
		if criterion.Field == "status" || criterion.Field == "section" {
			return fmt.Errorf("criterion %d uses legacy lifecycle field %q", i, criterion.Field)
		}
		if err := criterion.validate(); err != nil {
			return fmt.Errorf("criterion %d: %w", i, err)
		}
	}
	return nil
}

// MarshalJSON persists only canonical lifecycle fields.
func (f SavedFilter) MarshalJSON() ([]byte, error) {
	f.normalizeLifecycle()
	if err := f.Validate(); err != nil {
		return nil, err
	}

	wire := savedFilterWire{
		ID:                f.ID,
		Name:              f.Name,
		Mode:              f.Mode,
		FilterType:        f.FilterType,
		FilterPriority:    f.FilterPriority,
		FilterTags:        f.FilterTags,
		FilterDueDate:     f.FilterDueDate,
		FilterActionable:  f.FilterActionable,
		FilterHideDimmed:  f.FilterHideDimmed,
		FilterGitHubIssue: f.FilterGitHubIssue,
		FilterParked:      f.FilterParked,
		FilterAttrs:       f.FilterAttrs,
		FilterScoreMin:    f.FilterScoreMin,
		Criteria:          f.Criteria,
		Logic:             f.Logic,
		IncludeCompleted:  f.IncludeCompleted,
	}
	if f.HasStateFilter() {
		data, err := json.Marshal(f.FilterStates)
		if err != nil {
			return nil, err
		}
		wire.FilterStates = data
	}
	return json.Marshal(wire)
}

// UnmarshalJSON accepts canonical filter_states and legacy
// filter_status/filter_sections snapshots. Canonical state data wins when both
// shapes are present.
func (f *SavedFilter) UnmarshalJSON(data []byte) error {
	var wire savedFilterWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	decoded := SavedFilter{
		ID:                wire.ID,
		Name:              wire.Name,
		Mode:              wire.Mode,
		FilterType:        wire.FilterType,
		FilterPriority:    wire.FilterPriority,
		FilterTags:        wire.FilterTags,
		FilterDueDate:     wire.FilterDueDate,
		FilterActionable:  wire.FilterActionable,
		FilterHideDimmed:  wire.FilterHideDimmed,
		FilterGitHubIssue: wire.FilterGitHubIssue,
		FilterParked:      wire.FilterParked,
		FilterAttrs:       wire.FilterAttrs,
		FilterScoreMin:    wire.FilterScoreMin,
		Criteria:          wire.Criteria,
		Logic:             wire.Logic,
		IncludeCompleted:  wire.IncludeCompleted,
	}

	hasLegacySnapshot := wire.FilterStatus != "" || len(wire.FilterSections) > 0
	decoded.legacyLifecycleDecoded = hasLegacySnapshot
	if len(wire.FilterStates) > 0 {
		var states models.WorkflowStateSet
		if err := json.Unmarshal(wire.FilterStates, &states); err != nil {
			return fmt.Errorf("filter_states: %w", err)
		}
		decoded.SetFilterStates(states, true)
	} else if hasLegacySnapshot {
		states, active := legacySnapshotStates(wire.FilterStatus, wire.FilterSections)
		decoded.SetFilterStates(states, active)
	}

	for i := range decoded.Criteria {
		if decoded.Criteria[i].legacyLifecycleDecoded {
			decoded.legacyLifecycleDecoded = true
			decoded.Criteria[i].legacyLifecycleDecoded = false
		}
	}
	if err := decoded.Validate(); err != nil {
		return err
	}
	*f = decoded
	return nil
}

func cloneWorkflowStateSet(states models.WorkflowStateSet) models.WorkflowStateSet {
	cloned, err := models.NewWorkflowStateSet(states.Values()...)
	if err != nil {
		panic(err)
	}
	return cloned
}

func legacySnapshotStates(status string, sections []string) (models.WorkflowStateSet, bool) {
	statusActive := status != ""
	sectionsActive := len(sections) > 0

	statusStates := legacyCriterionStates("status", "equals", status)
	var sectionStates models.WorkflowStateSet
	for _, section := range sections {
		sectionStates = sectionStates.Union(legacyCriterionStates("section", "equals", section))
	}

	switch {
	case statusActive && sectionsActive:
		return statusStates.Intersection(sectionStates), true
	case statusActive:
		return statusStates, true
	case sectionsActive:
		return sectionStates, true
	default:
		return models.WorkflowStateSet{}, false
	}
}

func (f *SavedFilter) normalizeLifecycle() bool {
	changed := f.legacyLifecycleDecoded
	f.legacyLifecycleDecoded = false
	f.FilterStates = cloneWorkflowStateSet(f.FilterStates)

	for i := range f.Criteria {
		canonical, criterionChanged := canonicalizeSmartListCriterion(f.Criteria[i])
		if criterionChanged || f.Criteria[i].legacyLifecycleDecoded {
			changed = true
		}
		canonical.legacyLifecycleDecoded = false
		f.Criteria[i] = canonical
	}
	return changed
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
	filter := SavedFilter{
		ID:                uuid.New().String(),
		Name:              sv.Name,
		Mode:              "snapshot",
		FilterType:        sv.FilterType,
		FilterPriority:    sv.FilterPriority,
		FilterTags:        sv.FilterTags,
		FilterDueDate:     sv.FilterDueDate,
		FilterActionable:  sv.FilterActionable,
		FilterGitHubIssue: sv.FilterGitHubIssue,
		FilterParked:      sv.FilterParked,
		FilterAttrs:       sv.FilterAttrs,
		FilterScoreMin:    sv.FilterScoreMin,
	}
	states, active := legacySnapshotStates(sv.FilterStatus, sv.FilterSections)
	filter.SetFilterStates(states, active)
	return filter
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
	filter := SavedFilter{
		ID:               id,
		Name:             sl.Name,
		Mode:             "criteria",
		Criteria:         sl.Criteria,
		Logic:            sl.Logic,
		IncludeCompleted: sl.IncludeCompleted,
	}
	filter.normalizeLifecycle()
	return filter
}

// MigrateLegacyFilters converts savedViews and smartLists to SavedFilters.
// It is idempotent: if both legacy slices are already empty nothing changes.
// Any migrated smartList whose preserved ID would collide with an existing
// SavedFilter (or another smartList) receives a fresh UUID so ID-based
// lookups remain unambiguous.
// Returns true if a migration was performed.
func (p *PlanningState) MigrateLegacyFilters() bool {
	changed := false
	for i := range p.SavedFilters {
		if p.SavedFilters[i].normalizeLifecycle() {
			changed = true
		}
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
		changed = true
	}
	for _, sl := range p.SmartLists {
		f := smartListToFilter(sl)
		for seenIDs[f.ID] {
			f.ID = uuid.New().String()
		}
		seenIDs[f.ID] = true
		p.SavedFilters = append(p.SavedFilters, f)
		changed = true
	}

	if len(p.SavedViews) > 0 {
		p.SavedViews = nil
	}
	if len(p.SmartLists) > 0 {
		p.SmartLists = nil
	}
	return changed
}
