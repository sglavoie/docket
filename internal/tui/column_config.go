package tui

import "github.com/sglavoie/tt/internal/config"

// Priority display mode constants.
const (
	PriorityModeAll         = "all"
	PriorityModeConditional = "conditional"
	PriorityModeHidden      = "hidden"
)

// Column name constants for configurable column visibility.
const (
	ColPriority      = "priority"
	ColDueDate       = "due_date"
	ColTags          = "tags"
	ColScore         = "score"
	ColCustomSection = "custom_section"
	ColDependencies  = "dependencies"
	ColComments      = "comments"
	ColAttachments   = "attachments"
	ColChecklist     = "checklist"
	ColProgress      = "progress"
	ColRecurrence    = "recurrence"
	ColSnooze        = "snooze"
	ColDecisions     = "decisions"
	ColShortID       = "short_id"
)

// DefaultTreeColumns defines which columns are visible by default in the tree view.
var DefaultTreeColumns = map[string]bool{
	ColPriority:      true,
	ColDueDate:       true,
	ColTags:          true,
	ColScore:         true,
	ColCustomSection: true,
	ColDependencies:  true,
	ColComments:      true,
	ColAttachments:   true,
	ColChecklist:     true,
	ColProgress:      true,
	ColRecurrence:    true,
	ColSnooze:        true,
	ColDecisions:     true,
	ColShortID:       true,
}

// DefaultOutlinerColumns defines which columns are visible by default in the outliner view.
var DefaultOutlinerColumns = map[string]bool{
	ColPriority:      true,
	ColDueDate:       true,
	ColTags:          true,
	ColScore:         true,
	ColCustomSection: true,
	ColDependencies:  true,
	ColComments:      true,
	ColAttachments:   true,
	ColChecklist:     true,
	ColProgress:      true,
	ColRecurrence:    true,
	ColSnooze:        true,
	ColDecisions:     true,
	ColShortID:       true,
}

// ColumnConfig controls which indicator columns are visible in a view.
// Resolution chain: per-project toggles > global defaults > hardcoded defaults > true.
type ColumnConfig struct {
	toggles        map[string]bool // per-project overrides
	globalDefaults map[string]bool // global settings defaults
	defaults       map[string]bool // hardcoded defaults
	priorityMode   string          // "all", "conditional", "hidden" (empty = conditional)
}

// NewColumnConfig creates a ColumnConfig with a 3-tier resolution chain.
// Any argument may be nil. priorityMode controls priority badge display.
func NewColumnConfig(toggles, globalDefaults, defaults map[string]bool, priorityMode string) *ColumnConfig {
	return &ColumnConfig{
		toggles:        toggles,
		globalDefaults: globalDefaults,
		defaults:       defaults,
		priorityMode:   priorityMode,
	}
}

// ShouldShowPriority returns whether a priority badge should be displayed for
// the given priority level, based on the configured priority display mode.
func (c *ColumnConfig) ShouldShowPriority(priority int) bool {
	if c == nil {
		return priority <= 2
	}
	switch c.priorityMode {
	case PriorityModeAll:
		return true
	case PriorityModeHidden:
		return false
	default:
		return priority <= 2
	}
}

// Toggles returns the user-override toggle map (may be nil).
func (c *ColumnConfig) Toggles() map[string]bool {
	if c == nil {
		return nil
	}
	return c.toggles
}

// PriorityMode returns the current priority display mode string.
func (c *ColumnConfig) PriorityMode() string {
	if c == nil {
		return PriorityModeConditional
	}
	if c.priorityMode == "" {
		return PriorityModeConditional
	}
	return c.priorityMode
}

// resolvePriorityMode resolves the priority mode for a view, checking per-project
// state first, then falling back to the global config default.
// viewName must be "tree" or "outliner".
func resolvePriorityMode(projectID string, cfg *config.Config, viewName string) string {
	if cfg == nil {
		return ""
	}
	if projectID != "" {
		if vs, ok := cfg.ProjectViewStates[projectID]; ok {
			var mode string
			switch viewName {
			case "tree":
				mode = vs.TreePriorityMode
			case "outliner":
				mode = vs.OutlinerPriorityMode
			}
			if mode != "" {
				return mode
			}
		}
	}
	switch viewName {
	case "tree":
		return cfg.TreePriorityMode
	case "outliner":
		return cfg.OutlinerPriorityMode
	}
	return ""
}

// IsVisible returns whether the named column should be rendered.
// User toggles take precedence over defaults. If neither map contains the
// column, it is visible by default.
func (c *ColumnConfig) IsVisible(col string) bool {
	if c == nil {
		return true
	}
	if c.toggles != nil {
		if v, ok := c.toggles[col]; ok {
			return v
		}
	}
	if c.globalDefaults != nil {
		if v, ok := c.globalDefaults[col]; ok {
			return v
		}
	}
	if c.defaults != nil {
		if v, ok := c.defaults[col]; ok {
			return v
		}
	}
	return true
}
