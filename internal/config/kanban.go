package config

// Default kanban display settings.
const (
	DefaultKanbanMaxColumns      = 6
	DefaultKanbanMaxSwimlaneRows = 4
)

// GetKanbanMaxColumns returns the maximum number of visible kanban columns.
// Returns 6 if not set.
func (c *Config) GetKanbanMaxColumns() int {
	if c.KanbanMaxColumns == nil {
		return DefaultKanbanMaxColumns
	}
	return *c.KanbanMaxColumns
}

// GetKanbanMaxSwimlaneRows returns the maximum number of visible swimlane rows.
// Returns 4 if not set.
func (c *Config) GetKanbanMaxSwimlaneRows() int {
	if c.KanbanMaxSwimlaneRows == nil {
		return DefaultKanbanMaxSwimlaneRows
	}
	return *c.KanbanMaxSwimlaneRows
}
