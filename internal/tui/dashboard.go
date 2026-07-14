package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// DashboardModel represents the dashboard view.
type DashboardModel struct {
	BaseSearchableView
	InteractiveViewState
	runtime      *config.RuntimeState
	projects     []*models.Project
	showArchived bool

	// Context filter state
	filterContext       string   // Empty string means "all contexts"
	filterCustomContext string   // Custom context filter (mutually exclusive with filterContext)
	contexts            []string // Dynamically discovered contexts from projects

	// Delete state
	deleteTarget *models.Project // project to delete

	// Pipeline summary counts for the pipeline info card.
	pipelineCounts pipelineCounts

	// dashboardCompactCards collapses inert info cards (no actionable
	// content) into one-line summary rows. Default true; toggled by `,c`.
	dashboardCompactCards bool
}

// SetPipelineCounts updates the cached pipeline counts shown in the info section.
func (m *DashboardModel) SetPipelineCounts(counts pipelineCounts) {
	m.pipelineCounts = counts
}

// NewDashboardModel creates a new dashboard model.
func NewDashboardModel(runtime *config.RuntimeState) *DashboardModel {
	machine := runtime.Machine()
	m := &DashboardModel{
		runtime:               runtime,
		filterContext:         machine.FilterContext,       // Machine-local context filter
		filterCustomContext:   machine.FilterCustomContext, // Machine-local custom context filter
		dashboardCompactCards: true,
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.loadProjects()
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	return m
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *DashboardModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// Refresh reloads the dashboard data.
func (m *DashboardModel) Refresh() {
	machine := m.runtime.Machine()
	m.filterContext = machine.FilterContext
	m.filterCustomContext = machine.FilterCustomContext
	m.loadProjects()
}

func (m *DashboardModel) resetViewScrollOffset() {
	m.viewScrollOffset = 0
}

// SetSize updates the view dimensions and resets the persisted scroll window
// because the visible project-list row budget may have changed.
func (m *DashboardModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
	m.resetViewScrollOffset()
}

func (m *DashboardModel) Init() tea.Cmd {
	return nil
}

// HasTextInput returns true when dashboard search is active.
func (m *DashboardModel) HasTextInput() bool {
	return m.DefaultHasTextInput()
}

// ShouldCaptureEsc returns true when dashboard search or prefix mode is active.
func (m *DashboardModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.HasActiveSearch()
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *DashboardModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			"g": NavigationPrefixHints,
		},
	}
}

// SelectedProject returns the currently highlighted project, or nil if none.
func (m *DashboardModel) SelectedProject() *models.Project {
	visible := m.visibleProjects()
	if CursorInBounds(m.cursor, len(visible)) {
		return visible[m.cursor]
	}
	return nil
}

func (m *DashboardModel) StatusBarLabel() string { return "Dashboard" }
