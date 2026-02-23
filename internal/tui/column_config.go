package tui

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
	ColShortID:       true,
}

// DefaultKanbanColumns defines which columns are visible by default in the kanban view.
var DefaultKanbanColumns = map[string]bool{
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
	ColShortID:       true,
}

// ColumnConfig controls which indicator columns are visible in a view.
// User toggles override the defaults: an explicit false hides a column that
// would otherwise be shown, and an explicit true shows a column that would
// otherwise be hidden.
type ColumnConfig struct {
	toggles  map[string]bool
	defaults map[string]bool
}

// NewColumnConfig creates a ColumnConfig from user toggles and a set of defaults.
// Either argument may be nil.
func NewColumnConfig(toggles map[string]bool, defaults map[string]bool) *ColumnConfig {
	return &ColumnConfig{
		toggles:  toggles,
		defaults: defaults,
	}
}

// Toggles returns the user-override toggle map (may be nil).
func (c *ColumnConfig) Toggles() map[string]bool {
	if c == nil {
		return nil
	}
	return c.toggles
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
	if c.defaults != nil {
		if v, ok := c.defaults[col]; ok {
			return v
		}
	}
	return true
}
