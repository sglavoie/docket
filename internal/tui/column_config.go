package tui

import "github.com/sglavoie/tt/internal/models"

// Priority display mode constants.
const (
	PriorityModeAll         = "all"
	PriorityModeConditional = "conditional"
	PriorityModeHidden      = "hidden"
)

// Column name constants for configurable column visibility.
const (
	ColPriority     = "priority"
	ColDueDate      = "due_date"
	ColTags         = "tags"
	ColDependencies = "dependencies"
	ColComments     = "comments"
	ColAttachments  = "attachments"
	ColChecklist    = "checklist"
	ColProgress     = "progress"
	ColRecurrence   = "recurrence"
	ColSnooze       = "snooze"
	ColDecisions    = "decisions"
	ColShortID      = "short_id"
	ColDescription  = "description"
)

// DefaultOutlinerColumns defines which columns are visible by default in the outliner view.
var DefaultOutlinerColumns = map[string]bool{
	ColPriority:     true,
	ColDueDate:      true,
	ColTags:         true,
	ColDependencies: true,
	ColComments:     true,
	ColAttachments:  true,
	ColChecklist:    true,
	ColProgress:     true,
	ColRecurrence:   true,
	ColSnooze:       true,
	ColDecisions:    true,
	ColShortID:      true,
	ColDescription:  true,
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
		return models.IsHighPriority(priority)
	}
	switch c.priorityMode {
	case PriorityModeAll:
		return true
	case PriorityModeHidden:
		return false
	default:
		return models.IsHighPriority(priority)
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
