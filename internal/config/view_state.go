package config

// ProjectViewState holds filter/sort state for a project's tree view.
type ProjectViewState struct {
	FilterType                string            `json:"filter_type,omitempty"`
	FilterStatus              string            `json:"filter_status,omitempty"`
	FilterPriority            int               `json:"filter_priority,omitempty"`
	FilterTags                []string          `json:"filter_tags,omitempty"`
	FilterSections            []string          `json:"filter_sections,omitempty"`
	FilterDueDate             int               `json:"filter_due_date,omitempty"`
	FilterActionable          bool              `json:"filter_actionable,omitempty"`
	FilterHideDimmed          bool              `json:"filter_hide_dimmed,omitempty"`
	FilterStage               string            `json:"filter_stage,omitempty"`
	SortField                 int               `json:"sort_field,omitempty"`
	SortAscending             bool              `json:"sort_ascending"`
	ShowCompleted             bool              `json:"show_completed"`
	SidebarEnabled            bool              `json:"sidebar_enabled"`
	FilterBarEnabled          bool              `json:"filter_bar_enabled"`
	ViewMode                  int               `json:"view_mode,omitempty"`
	CustomSectionTab          string            `json:"custom_section_tab,omitempty"`
	OutlinerFilterType        string            `json:"outliner_filter_type,omitempty"`
	OutlinerFilterStatus      string            `json:"outliner_filter_status,omitempty"`
	OutlinerFilterPriority    int               `json:"outliner_filter_priority,omitempty"`
	OutlinerFilterTags        []string          `json:"outliner_filter_tags,omitempty"`
	OutlinerFilterDueDate     int               `json:"outliner_filter_due_date,omitempty"`
	OutlinerFilterSections    []string          `json:"outliner_filter_sections,omitempty"`
	OutlinerFilterActionable  bool              `json:"outliner_filter_actionable,omitempty"`
	OutlinerFilterHideDimmed  bool              `json:"outliner_filter_hide_dimmed,omitempty"`
	OutlinerFilterStage       string            `json:"outliner_filter_stage,omitempty"`
	OutlinerFilterAttrs       map[string]string `json:"outliner_filter_attrs,omitempty"`
	OutlinerFilterScoreMin    float64           `json:"outliner_filter_score_min,omitempty"`
	TreeColumnToggles         map[string]bool   `json:"tree_column_toggles,omitempty"`
	OutlinerColumnToggles     map[string]bool   `json:"outliner_column_toggles,omitempty"`
	TreePriorityMode          string            `json:"tree_priority_mode,omitempty"`
	OutlinerPriorityMode      string            `json:"outliner_priority_mode,omitempty"`
	TreeFoldState             map[string]bool   `json:"tree_fold_state,omitempty"`
	OutlinerFoldState         map[string]bool   `json:"outliner_fold_state,omitempty"`
	OutlinerCollapsedSections map[string]bool   `json:"outliner_collapsed_sections,omitempty"`
	OutlinerSortField         int               `json:"outliner_sort_field,omitempty"`
	OutlinerSortAscending     bool              `json:"outliner_sort_ascending"`
	OutlinerActivePerspective int               `json:"outliner_active_perspective,omitempty"`
	OutlinerFilterBarEnabled  bool              `json:"outliner_filter_bar_enabled"`
}

// GetProjectViewState returns the saved view state for a project.
func (c *Config) GetProjectViewState(projectID string) (ProjectViewState, bool) {
	if c.ProjectViewStates == nil {
		return ProjectViewState{}, false
	}
	state, exists := c.ProjectViewStates[projectID]
	return state, exists
}

// SetProjectViewState saves the view state for a project.
func (c *Config) SetProjectViewState(projectID string, state ProjectViewState) {
	if c.ProjectViewStates == nil {
		c.ProjectViewStates = make(map[string]ProjectViewState)
	}
	c.ProjectViewStates[projectID] = state
}
