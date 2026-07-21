package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// DashboardModel renders the Home route's Projects and Attention panes.
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
	attentionCounts attentionCounts

	// Home owns one project list and one actionable Attention list. Pane focus
	// is presentation-local and is never persisted.
	homePane                  homePane
	attentionCursor           int
	attentionViewScrollOffset int
	attentionRows             []homeAttentionRow
	attentionAlerts           *alerts.AlertSnapshot
	attentionProjects         []*models.Project
}

// SetPipelineCounts updates the cached pipeline counts shown in the info section.
func (m *DashboardModel) SetPipelineCounts(counts attentionCounts) {
	m.attentionCounts = counts
	m.rebuildHomeAttention()
}

// NewDashboardModel creates a new dashboard model.
func NewDashboardModel(runtime *config.RuntimeState) *DashboardModel {
	if runtime == nil {
		runtime = config.EmptyRuntimeState("")
	}
	m := &DashboardModel{
		runtime:  runtime,
		homePane: homePaneProjects,
	}
	m.InitSearchInput("Search Home...", 100, 40)
	m.loadProjects()
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	return m
}

// NewHomeModel constructs the Home renderer.
func NewHomeModel(runtime *config.RuntimeState) *DashboardModel {
	return NewDashboardModel(runtime)
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *DashboardModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// Refresh reloads the dashboard data.
func (m *DashboardModel) Refresh() {
	m.loadProjects()
}

func (m *DashboardModel) resetViewScrollOffset() {
	m.viewScrollOffset = 0
	m.attentionViewScrollOffset = 0
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

// ShouldCaptureEsc returns true when Home search is active.
func (m *DashboardModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.HasActiveSearch()
}

// GetPrefixPopupInfo returns nil because Home has no prefix grammar.
func (m *DashboardModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	return nil
}

// SelectedProject returns the currently highlighted project, or nil if none.
func (m *DashboardModel) SelectedProject() *models.Project {
	visible := m.visibleProjects()
	if CursorInBounds(m.cursor, len(visible)) {
		return visible[m.cursor]
	}
	return nil
}

func (m *DashboardModel) StatusBarLabel() string { return "Home" }
