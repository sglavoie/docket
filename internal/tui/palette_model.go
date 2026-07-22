package tui

import (
	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// NewCommandPaletteModel creates a new command palette model.
// The currentView parameter determines which commands are shown (context-aware filtering).
// The currentProject parameter filters tasks to only show those from the specified project (nil = show all).
func NewCommandPaletteModel(runtime *config.RuntimeState, registry *CommandRegistry, currentView PresentationID, currentProject *models.Project) *CommandPaletteModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search commands, tasks, projects, notes, filters, and more"
	searchInput.CharLimit = 100
	searchInput.SetWidth(40)
	searchInput.Focus()
	applyThemedTextInput(&searchInput)

	// Load persisted recent items from machine-local state.
	recentPaletteSelections = nil
	for _, item := range runtime.Machine().GetRecentPaletteItems() {
		recentPaletteSelections = append(recentPaletteSelections, recentSelection{
			Category: ItemCategory(item.Category),
			ID:       item.ID,
			Count:    item.Count,
		})
	}

	m := &CommandPaletteModel{
		runtime:        runtime,
		registry:       registry,
		currentView:    currentView,
		currentProject: currentProject,
		searchInput:    searchInput,
		cursor:         0,
		scrollAnchor:   -1,
		showTips:       true,
		selectedTasks:  make(map[TaskIdentity]bool),
	}

	// Load all searchable items (lazy loading on palette open per design decision)
	m.allItems = m.loadAllItems()
	// Start with all items visible, sorted by category (no search filter, no highlights)
	m.filterItems()
	return m
}

// SetCommandContext applies a typed route/selection context and rebuilds the
// palette's lazy item index.
func (m *CommandPaletteModel) SetCommandContext(ctx CommandContext) {
	m.commandContext = &ctx
	m.allItems = m.loadAllItems()
	m.filterItems()
}

// HasActiveSearch returns true (search input is always active in the palette).
func (m *CommandPaletteModel) HasActiveSearch() bool {
	return true
}

// HasTextInput returns true because the palette always has input focus.
func (m *CommandPaletteModel) HasTextInput() bool {
	return true
}

// IsFullHeight returns true because the command palette renders its own footer.
func (m *CommandPaletteModel) IsFullHeight() bool {
	return true
}

// IsActionMode returns true if the palette is showing inline actions for an item.
func (m *CommandPaletteModel) IsActionMode() bool {
	return m.actionMode
}

// ShouldCaptureEsc returns true when inline actions are shown.
func (m *CommandPaletteModel) ShouldCaptureEsc() bool {
	return m.IsActionMode()
}

// SetSize sets the view dimensions.
func (m *CommandPaletteModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate inner content width for split-pane overlay:
	// cardWidth = viewWidth - 4 (margin)
	// lipgloss Width = cardWidth - 2 (border adds 2 to total)
	// usable text area = Width - 4 (padding inside) = viewWidth - 10
	innerWidth := max(width-10, 1)

	m.searchInput.SetWidth(max(innerWidth-4, 1))

	// Calculate split pane widths: 65% list, 35% preview based on inner width.
	// On very narrow terminals, prefer a single-pane layout and let compact mode hide preview.
	if innerWidth <= 2 {
		m.listWidth = innerWidth
		m.previewWidth = 0
	} else {
		m.listWidth = max(innerWidth*65/100, 1)
		m.previewWidth = innerWidth - m.listWidth - 1
		if m.previewWidth < 1 {
			m.previewWidth = 1
			m.listWidth = max(innerWidth-m.previewWidth-1, 1)
		}
	}

	// Narrow layouts remain responsive, but compact presentation is no longer
	// a user-controlled or persisted palette mode.
	m.compactMode = width < 100 || m.previewWidth < 25
}
