package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// DashboardModel represents the dashboard view.
type DashboardModel struct {
	BaseSearchableView
	config       *config.Config
	projects     []*models.Project
	cursor       int
	showArchived bool

	// Context filter state
	filterContext       string   // Empty string means "all contexts"
	filterCustomContext string   // Custom context filter (mutually exclusive with filterContext)
	contexts            []string // Dynamically discovered contexts from projects

	// Confirmation state
	confirm      ConfirmDialog   // confirmation dialog
	deleteTarget *models.Project // project to delete
}

// NewDashboardModel creates a new dashboard model.
func NewDashboardModel(cfg *config.Config) *DashboardModel {
	m := &DashboardModel{
		config:              cfg,
		cursor:              0,
		filterContext:       cfg.FilterContext,       // Load persisted context filter
		filterCustomContext: cfg.FilterCustomContext, // Load persisted custom context filter
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.loadProjects()
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

// ResetCursor moves the cursor to the first project.
func (m *DashboardModel) ResetCursor() {
	m.cursor = 0
}

// SetSize sets the dashboard dimensions.
func (m *DashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *DashboardModel) Init() tea.Cmd {
	return nil
}

// HasTextInput returns true when dashboard search is active.
func (m *DashboardModel) HasTextInput() bool {
	return m.HasActiveSearch()
}

// ShouldCaptureEsc returns true when dashboard search or prefix mode is active.
func (m *DashboardModel) ShouldCaptureEsc() bool {
	return m.HasActiveSearch()
}
