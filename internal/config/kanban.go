package config

const (
	defaultKanbanMode        = "section"
	defaultKanbanStartColumn = "inbox"
)

// KanbanSettings holds persistent configuration for the kanban view.
type KanbanSettings struct {
	DefaultMode        string   `json:"default_mode,omitempty"`         // KanbanMode ID (default: "section")
	DefaultStartColumn string   `json:"default_start_column,omitempty"` // Column ID to focus on open (default: "inbox")
	EnabledModes       []string `json:"enabled_modes,omitempty"`        // Enabled KanbanMode IDs (all enabled when nil)
	SectionColumnOrder []string `json:"section_column_order,omitempty"` // GTD section column order override
	StatusColumnOrder  []string `json:"status_column_order,omitempty"`  // Status column order override
	ShowEmptyColumns   *bool    `json:"show_empty_columns,omitempty"`   // nil=true: show columns even when empty
	DetailPaneVisible  *bool    `json:"detail_pane_visible,omitempty"`  // nil=false: bottom detail pane hidden by default
	DetailPaneHeight   *int     `json:"detail_pane_height,omitempty"`   // nil=3: number of detail pane lines
}
