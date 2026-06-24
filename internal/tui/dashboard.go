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
	config        *config.Config
	configManager *config.Manager
	projects      []*models.Project
	showArchived  bool

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

	// openLoops mirrors the AppModel-level cross-project counters; injected
	// before each render via setOpenLoops. Drives the Next Action strip's
	// inbox-only / first-run / caught-up variant resolution.
	openLoops OpenLoops
}

// setOpenLoops injects the latest cross-project open-loop counts into the
// dashboard so the Next Action strip can pick the right AllDone variant
// without recomputing the counters per render.
func (m *DashboardModel) setOpenLoops(o OpenLoops) { m.openLoops = o }

// SetPipelineCounts updates the cached pipeline counts shown in the info section.
func (m *DashboardModel) SetPipelineCounts(counts pipelineCounts) {
	m.pipelineCounts = counts
}

// NewDashboardModel creates a new dashboard model.
func NewDashboardModel(cfg *config.Config, manager *config.Manager) *DashboardModel {
	m := &DashboardModel{
		config:                cfg,
		configManager:         manager,
		filterContext:         cfg.FilterContext,       // Load persisted context filter
		filterCustomContext:   cfg.FilterCustomContext, // Load persisted custom context filter
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
func (m *DashboardModel) Refresh(cfg *config.Config) {
	m.config = cfg
	m.filterContext = cfg.FilterContext
	m.filterCustomContext = cfg.FilterCustomContext
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
