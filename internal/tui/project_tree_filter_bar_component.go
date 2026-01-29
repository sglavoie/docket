package tui

// FilterBarComponent encapsulates filter bar state and functionality.
type FilterBarComponent struct {
	enabled bool           // Toggle filter bar visibility
	counts  map[string]int // Cached task counts per filter category

	// Children in other sections cache (computed during calculateFilterCounts)
	// Maps taskID -> sectionName -> count of descendants in that section
	childrenInOtherSections map[string]map[string]int
}

// NewFilterBarComponent creates a new filter bar component.
func NewFilterBarComponent() *FilterBarComponent {
	return &FilterBarComponent{
		enabled:                 true, // Filter bar on by default
		counts:                  make(map[string]int),
		childrenInOtherSections: make(map[string]map[string]int),
	}
}

// IsEnabled returns whether the filter bar is visible.
func (f *FilterBarComponent) IsEnabled() bool {
	return f.enabled
}

// SetEnabled sets the filter bar visibility.
func (f *FilterBarComponent) SetEnabled(enabled bool) {
	f.enabled = enabled
}

// Toggle toggles the filter bar visibility.
func (f *FilterBarComponent) Toggle() {
	f.enabled = !f.enabled
}

// GetCounts returns the cached filter counts.
func (f *FilterBarComponent) GetCounts() map[string]int {
	return f.counts
}

// SetCounts updates the cached filter counts.
func (f *FilterBarComponent) SetCounts(counts map[string]int) {
	f.counts = counts
}

// GetChildrenInOtherSections returns the children in other sections cache.
func (f *FilterBarComponent) GetChildrenInOtherSections() map[string]map[string]int {
	return f.childrenInOtherSections
}

// SetChildrenInOtherSections updates the children in other sections cache.
func (f *FilterBarComponent) SetChildrenInOtherSections(children map[string]map[string]int) {
	f.childrenInOtherSections = children
}

// Render renders the filter bar.
func (f *FilterBarComponent) Render(m *ProjectTreeModel) string {
	if !f.enabled {
		return ""
	}
	return m.renderFilterBar()
}
