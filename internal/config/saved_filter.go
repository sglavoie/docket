package config

import (
	"strings"

	"github.com/google/uuid"
)

// SavedFilter is a unified filter preset that replaces both SavedView (flat
// filter snapshots) and SmartList (compound criteria).  Mode selects which
// field set is active: "snapshot" or "criteria".
type SavedFilter struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// Mode: "snapshot" (flat filters from SavedView) or "criteria" (compound rules from SmartList).
	Mode string `json:"mode"`

	// --- Snapshot mode (from SavedView) ---
	ViewMode         int               `json:"view_mode,omitempty"`
	FilterType       string            `json:"filter_type,omitempty"`
	FilterStatus     string            `json:"filter_status,omitempty"`
	FilterPriority   int               `json:"filter_priority,omitempty"`
	FilterTags       []string          `json:"filter_tags,omitempty"`
	FilterSections   []string          `json:"filter_sections,omitempty"`
	FilterDueDate    int               `json:"filter_due_date,omitempty"`
	FilterActionable bool              `json:"filter_actionable,omitempty"`
	FilterAttrs      map[string]string `json:"filter_attrs,omitempty"`
	FilterScoreMin   float64           `json:"filter_score_min,omitempty"`
	SortField        int               `json:"sort_field,omitempty"`
	SortAscending    bool              `json:"sort_ascending"`
	CustomSectionTab string            `json:"custom_section_tab,omitempty"`

	// --- Criteria mode (from SmartList) ---
	Criteria         []SmartListCriterion `json:"criteria,omitempty"`
	Logic            string               `json:"logic,omitempty"`
	IncludeCompleted bool                 `json:"include_completed,omitempty"`

	// --- Tab behavior ---
	ShowAsTab bool `json:"show_as_tab,omitempty"`
}

// EffectiveLogic returns the logic operator for criteria mode, defaulting to "and".
func (f *SavedFilter) EffectiveLogic() string {
	if f.Logic == "or" {
		return "or"
	}
	return "and"
}

// --- CRUD operations on Config ---

// GetSavedFilters returns all saved filters.
func (c *Config) GetSavedFilters() []SavedFilter {
	if c.SavedFilters == nil {
		return []SavedFilter{}
	}
	return c.SavedFilters
}

// GetSavedFilter returns a saved filter by ID, or nil if not found.
func (c *Config) GetSavedFilter(id string) *SavedFilter {
	for i := range c.SavedFilters {
		if c.SavedFilters[i].ID == id {
			return &c.SavedFilters[i]
		}
	}
	return nil
}

// GetSavedFilterByName returns the first saved filter matching name (case-insensitive).
func (c *Config) GetSavedFilterByName(name string) *SavedFilter {
	nameLower := strings.ToLower(name)
	for i := range c.SavedFilters {
		if strings.ToLower(c.SavedFilters[i].Name) == nameLower {
			return &c.SavedFilters[i]
		}
	}
	return nil
}

// SaveFilter upserts a saved filter by ID.  If a filter with the same ID
// exists it is replaced; otherwise the filter is appended.
func (c *Config) SaveFilter(f SavedFilter) {
	for i := range c.SavedFilters {
		if c.SavedFilters[i].ID == f.ID {
			c.SavedFilters[i] = f
			return
		}
	}
	c.SavedFilters = append(c.SavedFilters, f)
}

// DeleteSavedFilter removes a saved filter by ID and cleans up AllTasksTabOrder.
// Returns true if the filter existed and was deleted.
func (c *Config) DeleteSavedFilter(id string) bool {
	found := false
	for i := range c.SavedFilters {
		if c.SavedFilters[i].ID == id {
			c.SavedFilters = append(c.SavedFilters[:i], c.SavedFilters[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return false
	}
	// Remove from tab order if present.
	var newOrder []string
	for _, entry := range c.AllTasksTabOrder {
		if entry != id {
			newOrder = append(newOrder, entry)
		}
	}
	c.AllTasksTabOrder = newOrder
	return true
}

// GetTabFilters returns saved filters where ShowAsTab is true.
func (c *Config) GetTabFilters() []SavedFilter {
	var tabs []SavedFilter
	for _, f := range c.SavedFilters {
		if f.ShowAsTab {
			tabs = append(tabs, f)
		}
	}
	return tabs
}

// --- Migration helpers ---

// savedViewToFilter converts a legacy SavedView into a SavedFilter with mode "snapshot".
func savedViewToFilter(sv SavedView) SavedFilter {
	return SavedFilter{
		ID:               uuid.New().String(),
		Name:             sv.Name,
		Mode:             "snapshot",
		ViewMode:         sv.ViewMode,
		FilterType:       sv.FilterType,
		FilterStatus:     sv.FilterStatus,
		FilterPriority:   sv.FilterPriority,
		FilterTags:       sv.FilterTags,
		FilterSections:   sv.FilterSections,
		FilterDueDate:    sv.FilterDueDate,
		FilterActionable: sv.FilterActionable,
		FilterAttrs:      sv.FilterAttrs,
		FilterScoreMin:   sv.FilterScoreMin,
		SortField:        sv.SortField,
		SortAscending:    sv.SortAscending,
		CustomSectionTab: sv.CustomSectionTab,
	}
}

// smartListToFilter converts a legacy SmartList into a SavedFilter with mode "criteria".
func smartListToFilter(sl SmartList) SavedFilter {
	return SavedFilter{
		ID:               sl.ID,
		Name:             sl.Name,
		Mode:             "criteria",
		Criteria:         sl.Criteria,
		Logic:            sl.Logic,
		SortField:        sl.SortField,
		SortAscending:    sl.SortAscending,
		IncludeCompleted: sl.IncludeCompleted,
		ShowAsTab:        true,
	}
}

// MigrateLegacyFilters converts SavedViews and SmartLists to SavedFilters.
// It is idempotent: if both legacy slices are already empty nothing changes.
// Returns true if a migration was performed.
func (c *Config) MigrateLegacyFilters() bool {
	if len(c.SavedViews) == 0 && len(c.SmartLists) == 0 {
		return false
	}

	for _, sv := range c.SavedViews {
		c.SavedFilters = append(c.SavedFilters, savedViewToFilter(sv))
	}
	for _, sl := range c.SmartLists {
		c.SavedFilters = append(c.SavedFilters, smartListToFilter(sl))
	}

	c.SavedViews = nil
	c.SmartLists = nil
	return true
}
