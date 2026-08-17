package tui

// Column name constants for configurable column visibility.
const (
	ColPriority    = "priority"
	ColDueDate     = "due_date"
	ColTags        = "tags"
	ColComments    = "comments"
	ColAttachments = "attachments"
	ColChecklist   = "checklist"
	ColProgress    = "progress"
	ColDefer       = "defer"
	ColRecency     = "recency"
)

// DefaultOutlinerColumns defines which columns are visible by default in the outliner view.
var DefaultOutlinerColumns = map[string]bool{
	ColPriority:    true,
	ColDueDate:     true,
	ColTags:        true,
	ColComments:    true,
	ColAttachments: true,
	ColChecklist:   true,
	ColProgress:    true,
	ColDefer:       true,
	ColRecency:     true,
}

// ColumnConfig controls which indicator columns are visible in a view.
// Resolution chain: per-project toggles > global defaults > hardcoded defaults > true.
type ColumnConfig struct {
	toggles        map[string]bool // per-project overrides
	globalDefaults map[string]bool // global settings defaults
	defaults       map[string]bool // hardcoded defaults
}

// NewColumnConfig creates a ColumnConfig with a 3-tier resolution chain.
// Any argument may be nil.
func NewColumnConfig(toggles, globalDefaults, defaults map[string]bool) *ColumnConfig {
	return &ColumnConfig{
		toggles:        toggles,
		globalDefaults: globalDefaults,
		defaults:       defaults,
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
